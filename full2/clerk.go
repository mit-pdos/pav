package full2

import (
	"github.com/mit-pdos/gokv/grove_ffi"
	"github.com/mit-pdos/gokv/urpc"
	"github.com/mit-pdos/secure-chat/full2/fc_ffi"
	"github.com/mit-pdos/secure-chat/full2/shared"
	"github.com/tchajed/goose/machine"
	"github.com/tchajed/marshal"
)

// Clerk only supports sequential calls to its methods.
type Clerk struct {
	cli     *urpc.Client
	log     []*shared.MsgT
	myNum   uint64
	privKey *fc_ffi.SignerT
	pubKeys []*fc_ffi.VerifierT
}

func (c *Clerk) Put(m *shared.MsgT) {
	log := append(c.log, m)
	logB := shared.EncodeMsgTSlice(log)
	var err shared.ErrorT
	sig, err := c.privKey.Sign(logB)
	// ECDSA_P256 gave diff len sigs, which complicates encoding.
	// ED25519 should have const len sigs.
	machine.Assume(uint64(len(sig)) == shared.SigLen)
	machine.Assume(err == shared.ErrNone)

	var b = make([]byte, 0)
	b = marshal.WriteInt(b, c.myNum)
	b = marshal.WriteBytes(b, sig)
	b = marshal.WriteBytes(b, logB)

	var r []byte
	err = c.cli.Call(shared.RpcPut, b, &r, 100)
	machine.Assume(err == urpc.ErrNone)
}

func (c *Clerk) Get() ([]*shared.MsgT, shared.ErrorT) {
	var r []byte
	err := c.cli.Call(shared.RpcGet, make([]byte, 0), &r, 100)
	machine.Assume(err == urpc.ErrNone)

	if len(r) < 8 {
		return nil, shared.ErrSome
	}
	sender, r2 := marshal.ReadInt(r)
	if !(0 <= sender && sender < shared.MaxSenders) {
		return nil, shared.ErrSome
	}
	if uint64(len(r2)) < shared.SigLen {
		return nil, shared.ErrSome
	}
	sig, data := marshal.ReadBytes(r2, shared.SigLen)

	pk := c.pubKeys[sender]
	if pk.Verify(sig, data) != shared.ErrNone {
		return nil, shared.ErrSome
	}

	log, _ := shared.DecodeMsgTSlice(data)
	if !shared.IsMsgTPrefix(c.log, log) {
		return nil, shared.ErrSome
	}
	c.log = log

	return log, shared.ErrNone
}

func MakeClerk(host grove_ffi.Address, myNum uint64, privKey *fc_ffi.SignerT, pubKeys []*fc_ffi.VerifierT) *Clerk {
	c := &Clerk{}
	c.cli = urpc.MakeClient(host)
	c.log = make([]*shared.MsgT, 0)
	c.myNum = myNum
	c.privKey = privKey
	c.pubKeys = pubKeys
	return c
}
