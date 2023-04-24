package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	pb "example.com/chatGrpc"
	"github.com/manifoldco/promptui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type userMetadata struct {
	pubKey *rsa.PublicKey
}

type myMetadata struct {
	userMetadata
	name    string
	privKey *rsa.PrivateKey
}

type mutexMsgs struct {
	// Key is the seqNum.
	msgs map[uint64]*pb.MsgWrap
	mu  sync.Mutex
}

type client struct {
	rpc    pb.ChatClient
	myData *myMetadata
	// Key is the username.
	allData map[string]*userMetadata
	msgs mutexMsgs
}

func newClient() (*client, *grpc.ClientConn) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln("failed to connect:", err)
	}
	c := &client{rpc: pb.NewChatClient(conn)}
	c.myData = &myMetadata{}
	c.allData = make(map[string]*userMetadata)
	c.msgs.msgs = make(map[uint64]*pb.MsgWrap)
	return c, conn
}

func (myClient *client) runNameLoop() {
	for {
		prompt := promptui.Select{
			Label: "Name",
			Items: []string{"alice", "bob", "charlie", "danny", "eve"},
		}
		_, name, err := prompt.Run()
		if err != nil {
			log.Println("warning: failed prompt:", err)
			continue
		}
		myClient.myData.name = name
		return
	}
}

func (myClient *client) loadKeys() error {
	fileBytes, err := os.ReadFile("demo_keys")
	if err != nil {
		return err
	}
	manyUserKeys := &pb.ManyUserKeys{}
	if err := proto.Unmarshal(fileBytes, manyUserKeys); err != nil {
		return err
	}

	for _, userKey := range manyUserKeys.UserKeys {
		pubKey, err := x509.ParsePKCS1PublicKey(userKey.PubKey)
		if err != nil {
			return err
		}

		if myClient.myData.name == userKey.Name {
			privKey, err := x509.ParsePKCS1PrivateKey(userKey.PrivKey)
			if err != nil {
				return err
			}
			myClient.myData.privKey = privKey
			myClient.myData.pubKey = pubKey
		} else {
			user, ok := myClient.allData[userKey.Name]
			if !ok {
				user = &userMetadata{}
				myClient.allData[userKey.Name] = user
			}
			user.pubKey = pubKey
		}
	}

	if myClient.myData.privKey == nil {
		return errors.New("do not have private key for user")
	}
	return nil
}

func hash(msg proto.Message) ([]byte, error) {
	bytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	hash := sha512.Sum512(bytes)
	return hash[:], nil
}

func checkHash(msgWrap *pb.MsgWrap) error {
	refHash, err := hash(msgWrap.Msg)
	if err != nil {
		return err
	}
	if !bytes.Equal(refHash, msgWrap.Hash) {
		return errors.New("given hash does not match reference hash")
	}
	return nil
}

func (myClient *client) checkSig(msgWrap *pb.MsgWrap) error {
	userData, ok := myClient.allData[msgWrap.Msg.Sender]
	if !ok {
		return errors.New("do not have public key for user")
	}
	if err := rsa.VerifyPSS(userData.pubKey, crypto.SHA512, msgWrap.Hash, msgWrap.Sig, nil); err != nil {
		return err
	}
	return nil
}

func (myClient *client) checkPins(msgWrap *pb.MsgWrap) error {
	msgs := myClient.msgs.msgs
	// TODO: add functionality to wait on pins not being there.
	for _, pin := range msgWrap.Msg.Pins {
		pinnedMsg, ok := msgs[pin.SeqNum]
		if !ok {
			return errors.New("pinned msg not contained in local history")
		}
		if msgWrap.SeqNum <= pinnedMsg.SeqNum {
			return errors.New("pinned msg has greater seqNum than actual msg")
		}	
		if !bytes.Equal(pinnedMsg.Hash, pin.Hash) {
			return errors.New("pin has diff msg hash than local history")
		}
	}
	return nil
}

func (myClient *client) checkDupSeqNumAndAdd(msgWrap *pb.MsgWrap) error {
	myClient.msgs.mu.Lock()
	msgs := myClient.msgs.msgs
	defer myClient.msgs.mu.Unlock()
	if _, ok := msgs[msgWrap.SeqNum]; ok {
		return errors.New("seqNum already exists in local history")
	}
	log.Printf("`%v`: \"%v\"\n", msgWrap.Msg.Sender, msgWrap.Msg.Body)
	msgs[msgWrap.SeqNum] = msgWrap
	return nil
}

func (myClient *client) checkAndAddRcvdMsg(msgWrap *pb.MsgWrap) error {
	if err := checkHash(msgWrap); err != nil {
		return err
	}
	// Note: sig check relies on hash check occuring *before* it.
	if err := myClient.checkSig(msgWrap); err != nil {
		return err
	}
	if err := myClient.checkPins(msgWrap); err != nil {
		return err
	}
	if err := myClient.checkDupSeqNumAndAdd(msgWrap); err != nil {
		return err
	}
	return nil
}

func (myClient *client) getMsgs() {
	// TODO: don't know how to do indefinite timeout, so use an hour instead.
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	stream, err := myClient.rpc.GetMsgs(ctx, &pb.GetMsgsReq{Sender: myClient.myData.name})
	if err != nil {
		log.Fatalln("failed getMsgs:", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Fatalln("failed getMsgs stream recv:", err)
		}
		if err = myClient.checkAndAddRcvdMsg(resp.Msg); err != nil {
			log.Println("failed to add msg:", err)
		}
	}
}

func (myClient *client) getPins() (maxPinSeqNum uint64, pins []*pb.Pin) {
	pins = make([]*pb.Pin, 0, len(myClient.msgs.msgs))
	for seqNum, msgWrap := range myClient.msgs.msgs {
		if maxPinSeqNum < seqNum {
			maxPinSeqNum = seqNum
		}
		pin := &pb.Pin{
			SeqNum: seqNum, Hash: msgWrap.Hash,
		}
		pins = append(pins, pin)
	}
	return
}

func (myClient *client) hashAndSignMsg(msg *pb.Msg) (*pb.MsgWrap, error) {
	msgWrap := &pb.MsgWrap{}
	msgWrap.Msg = msg
	hash, err := hash(msg)
	if err != nil {
		return nil, err
	}
	msgWrap.Hash = hash
	sig, err := rsa.SignPSS(rand.Reader, myClient.myData.privKey, crypto.SHA512, hash, nil)
	if err != nil {
		return nil, err
	}
	msgWrap.Sig = sig
	return msgWrap, nil
}

func (myClient *client) putMsg(body *string) error {
	maxPinSeqNum, pins := myClient.getPins()
	msg := &pb.Msg{
		Sender: myClient.myData.name, Body: *body, Pins: pins,
	}
	msgWrap, err := myClient.hashAndSignMsg(msg)
	if err != nil {
		return err
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		putResp, err := myClient.rpc.PutMsg(ctx, &pb.PutMsgReq{Msg: msgWrap})
		if err != nil {
			log.Println("put rpc returned an err, retrying...")
			continue
		}
		if putResp.SeqNum <= maxPinSeqNum {
			log.Println("put rpc gave a seqNum before our pins, retrying...")
			continue
		}
		msgWrap.SeqNum = putResp.SeqNum	
		if err := myClient.checkDupSeqNumAndAdd(msgWrap); err != nil {
			log.Println("put rpc return a duplicate seqNum, retrying...")
			continue
		}
		break
	}
	return nil
}

func (myClient *client) runMsgLoop() {
	for {
		prompt := promptui.Select{
			Label: "Action",
			Items: []string{"PutMsg", "End"},
		}
		_, action, err := prompt.Run()
		if err != nil {
			log.Println("warning: failed prompt:", err)
			continue
		}

		if action == "End" {
			return
		} else if action == "PutMsg" {
			prompt := promptui.Prompt{
				Label: "Msg",
			}
			msg, err := prompt.Run()
			if err != nil {
				log.Println("warning: failed prompt:", err)
				continue
			}
			if err = myClient.putMsg(&msg); err != nil {
				log.Println("failed putMsg:", err)
			}
		} else {
			log.Println("warning: unrecognized action:", action)
		}
	}
}

func main() {
	myClient, conn := newClient()
	defer conn.Close()
	myClient.runNameLoop()
	if err := myClient.loadKeys(); err != nil {
		log.Fatalln("failed to load keys:", err)
	}
	go myClient.getMsgs()
	myClient.runMsgLoop()
}
