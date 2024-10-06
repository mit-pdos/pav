package kt2

type PreSigDig struct {
	Epoch uint64
	Dig   []byte
}

type SigDig struct {
	Epoch uint64
	Dig   []byte
	Sig   []byte
}

type MapLabelPre struct {
	Uid uint64
	Ver uint64
}

type PkCommOpen struct {
	Pk []byte
	R  []byte
}

type MapValPre struct {
	Epoch  uint64
	PkComm []byte
}

type MembProof struct {
	Label      []byte
	VrfProof   []byte
	EpochAdded uint64
	CommOpen   *PkCommOpen
	MerkProof  [][][]byte
}

type NonMembProof struct {
	Label     []byte
	VrfProof  []byte
	MerkProof [][][]byte
}

type UpdateProof struct {
	Updates map[string][]byte
	Sig     []byte
}

type ServerPutArg struct {
	Uid uint64
	Pk  []byte
}

type ServerPutReply struct {
	Dig    *SigDig
	Latest *MembProof
	Bound  *NonMembProof
}

type ServerGetArg struct {
	Uid uint64
}

type ServerGetReply struct {
	Dig   *SigDig
	Hist  []*MembProof
	Bound *NonMembProof
}

type ServerSelfMonArg struct {
	Uid uint64
}

type ServerSelfMonReply struct {
	Dig   *SigDig
	Bound *NonMembProof
}

type ServerAuditArg struct {
	Epoch uint64
}

type ServerAuditReply struct {
	P   *UpdateProof
	Err bool
}

type AdtrUpdateArg struct {
	P *UpdateProof
}

type AdtrUpdateReply struct {
	Err bool
}

type AdtrGetArg struct {
	Epoch uint64
}

type AdtrEpochInfo struct {
	Dig     []byte
	ServSig []byte
	AdtrSig []byte
}

type AdtrGetReply struct {
	X   *AdtrEpochInfo
	Err bool
}
