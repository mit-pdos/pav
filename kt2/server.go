package kt2

import (
	"errors"
	"github.com/goose-lang/primitive"
	"github.com/goose-lang/std"
	"github.com/mit-pdos/pav/cryptoffi"
	"github.com/mit-pdos/pav/merkle"
	"github.com/mit-pdos/pav/rpcffi"
	"sync"
)

type linkTy = []byte
type errorTy = bool
type epochTy = uint64

type epochChain struct {
	epochs []*epochInfo
}

type mapLabel struct {
	uid uint64
	ver uint64
}

// labelHash = VRF(Encode(mapLabel)).
// valHash = Hash(pk).
type mapEntry struct {
	labelHash []byte
	valHash   []byte
}

type epochInfo struct {
	updates  []*mapEntry
	prevLink linkTy
	dig      merkle.Digest
	link     linkTy
	linkSig  cryptoffi.Sig
}

type chainSepNone struct {
	tag byte
}

type chainSepSome struct {
	tag      byte
	epoch    epochTy
	prevLink linkTy
	data     []byte
}

const (
	chainSepNoneTag = 0
	chainSepSomeTag = 1
)

func firstLink() linkTy {
	pre := rpcffi.Encode(&chainSepNone{tag: chainSepNoneTag})
	return cryptoffi.Hash(pre)
}

func nextLink(epoch epochTy, prevLink, data []byte) []byte {
	pre := rpcffi.Encode(&chainSepSome{tag: chainSepSomeTag, epoch: epoch, prevLink: prevLink, data: data})
	return cryptoffi.Hash(pre)
}

func (c *epochChain) put(updates []*mapEntry, dig merkle.Digest, sk cryptoffi.PrivateKey) {
	chainLen := uint64(len(c.epochs))
	var prevLink linkTy
	if chainLen == 0 {
		prevLink = firstLink()
	} else {
		prevLink = c.epochs[chainLen-1].link
	}
	link := nextLink(chainLen, prevLink, dig)
	// no need for server sig domain sep since there's only one msg type.
	sig := sk.Sign(link)
	epoch := &epochInfo{updates: updates, prevLink: prevLink, dig: dig, link: link, linkSig: sig}
	c.epochs = append(c.epochs, epoch)
}

type Server struct {
	mu    *sync.Mutex
	sigSk cryptoffi.PrivateKey
	vrfSk *cryptoffi.VRFPrivateKey
	chain *epochChain
	// keyMap stores (VRF(uid, version), Hash(pk)).
	keyMap *merkle.Tree
	// uidVer stores the next version # for a uid.
	uidVer map[uint64]uint64
	// fullKeyMap stores (VRF(uid, ver), pk).
	fullKeyMap map[string][]byte
}

type PutArgs struct {
	uid uint64
	pk  []byte
}

type histMembProof struct {
	label     []byte
	vrfProof  []byte
	pk        merkle.Val
	merkProof merkle.Proof
}

type histNonMembProof struct {
	label     []byte
	vrfProof  []byte
	merkProof merkle.Proof
}

type histProof struct {
	sigLn   *SignedLink
	membs   []*histMembProof
	nonMemb *histNonMembProof
}

func compMapLabel(uid uint64, ver uint64, sk *cryptoffi.VRFPrivateKey) ([]byte, []byte) {
	l := &mapLabel{uid: uid, ver: ver}
	lByt := rpcffi.Encode(l)
	h, p := sk.Hash(lByt)
	return h, p
}

func (s *Server) getHistProof(uid uint64) *histProof {
	// get signed link.
	numEpochs := uint64(len(s.chain.epochs))
	lastInfo := s.chain.epochs[numEpochs-1]
	sigLn := &SignedLink{epoch: numEpochs - 1, prevLink: lastInfo.prevLink, dig: lastInfo.dig, sig: lastInfo.linkSig}

	// get memb proofs for all existing versions.
	var membs []*histMembProof
	nextVer := s.uidVer[uid]
	for ver := uint64(0); ver < nextVer; ver++ {
		label, vrfProof := compMapLabel(uid, ver, s.vrfSk)
		getReply := s.keyMap.Get(label)
		primitive.Assert(!getReply.Error)
		primitive.Assert(getReply.ProofTy)
		pk, ok := s.fullKeyMap[string(label)]
		primitive.Assert(ok)
		newMemb := &histMembProof{label: label, vrfProof: vrfProof, pk: pk, merkProof: getReply.Proof}
		membs = append(membs, newMemb)
	}

	// get non-memb proof for next version.
	nextLabel, nextVrfProof := compMapLabel(uid, nextVer, s.vrfSk)
	nextReply := s.keyMap.Get(nextLabel)
	primitive.Assert(!nextReply.Error)
	primitive.Assert(!nextReply.ProofTy)
	nonMemb := &histNonMembProof{label: nextLabel, vrfProof: nextVrfProof, merkProof: nextReply.Proof}

	return &histProof{sigLn: sigLn, membs: membs, nonMemb: nonMemb}
}

func (s *Server) Put(args *PutArgs, reply *histProof) error {
	s.mu.Lock()
	// add to keyMap.
	ver, _ := s.uidVer[args.uid]
	label, _ := compMapLabel(args.uid, ver, s.vrfSk)
	val := cryptoffi.Hash(args.pk)
	dig, _, err0 := s.keyMap.Put(label, val)
	primitive.Assert(!err0)

	// update supporting stores.
	// assume that we'll run out of mem before running out of versions.
	s.uidVer[args.uid] = std.SumAssumeNoOverflow(ver, 1)
	s.fullKeyMap[string(label)] = args.pk

	// sign new dig.
	updates := []*mapEntry{{labelHash: label, valHash: val}}
	s.chain.put(updates, dig, s.sigSk)

	// get history proof.
	*reply = *s.getHistProof(args.uid)
	s.mu.Unlock()
	return nil
}

func (s *Server) Get(uid *uint64, reply *histProof) error {
	s.mu.Lock()
	*reply = *s.getHistProof(*uid)
	s.mu.Unlock()
	return nil
}

type AuditUpd struct {
	// epoch set by caller, not by server.
	epoch   epochTy
	updates []*mapEntry
	linkSig []byte
}

func (s *Server) Audit(epoch *uint64, reply *AuditUpd) error {
	s.mu.Lock()
	if *epoch >= uint64(len(s.chain.epochs)) {
		s.mu.Unlock()
		return errors.New("Audit")
	}
	inf := s.chain.epochs[*epoch]
	reply.updates = inf.updates
	reply.linkSig = inf.linkSig
	s.mu.Unlock()
	return nil
}

func (s *Server) Links(unused *struct{}, reply *[]*SignedLink) error {
	s.mu.Lock()
	var sigLinks []*SignedLink
	for ep, inf := range s.chain.epochs {
		epoch := uint64(ep)
		sigLn := &SignedLink{epoch: epoch, prevLink: inf.prevLink, dig: inf.dig, sig: inf.linkSig}
		sigLinks = append(sigLinks, sigLn)
	}
	*reply = sigLinks
	s.mu.Unlock()
	return nil
}

func newServer() (*Server, cryptoffi.PublicKey, *cryptoffi.VRFPublicKey) {
	mu := new(sync.Mutex)
	sigPk, sigSk := cryptoffi.GenerateKey()
	vrfPk, vrfSk := cryptoffi.VRFGenerateKey()
	c := &epochChain{}
	m := &merkle.Tree{}
	ver := make(map[uint64]uint64)
	fm := make(map[string][]byte)
	return &Server{mu: mu, sigSk: sigSk, vrfSk: vrfSk, chain: c, keyMap: m, uidVer: ver, fullKeyMap: fm}, sigPk, vrfPk
}
