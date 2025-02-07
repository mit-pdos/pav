package kt

import (
	"github.com/goose-lang/primitive"
	"github.com/mit-pdos/pav/cryptoffi"
	"github.com/mit-pdos/pav/cryptoutil"
	"github.com/mit-pdos/pav/merkle"
	"sync"
)

type Server struct {
	mu    *sync.Mutex
	sigSk *cryptoffi.SigPrivateKey
	vrfSk *cryptoffi.VrfPrivateKey
	// keyMap stores (mapLabel, mapVal) entries.
	keyMap *merkle.Tree
	// epochHist stores info about prior epochs.
	epochHist    []*servEpochInfo
	commitSecret []byte
	// pkCommOpens stores pk commitment openings for a particular mapLabel.
	pkCommOpens map[string]*CommitOpen
	// uidVerRepo provides the authoritative view on the number of versions
	// registered per uid. this corresponds to entries in keyMap.
	// uid is map idx, and # reg versions is len(sl)-1,
	// except when len(sl) == 0, in which case it's 0 versions.
	// it also caches the expensive vrf computations
	// to make a label from a uid and version.
	uidVerRepo map[uint64][]*vrfCache
}

type servEpochInfo struct {
	// updates stores (mapLabel, mapVal) keyMap updates.
	updates map[string][]byte
	dig     []byte
	sig     []byte
}

type vrfCache struct {
	// label is the vrf hash.
	label []byte
	proof []byte
}

func (s *Server) Put(uid uint64, pk []byte) (*SigDig, *Memb, *NonMemb) {
	s.mu.Lock()
	// get lat label and make bound label.
	labels := getLabels(s.uidVerRepo, uid, s.vrfSk)
	boundVer := uint64(len(labels))
	latLabel := labels[boundVer-1]
	boundLabel, boundLabelProof := compMapLabel(uid, boundVer, s.vrfSk)
	s.uidVerRepo[uid] = append(labels, &vrfCache{label: boundLabel, proof: boundLabelProof})

	// make mapVal.
	nextEpoch := uint64(len(s.epochHist))
	open := genCommitOpen(pk)
	s.pkCommOpens[string(latLabel.label)] = open
	mapVal := compMapVal(nextEpoch, open)

	// add to key map.
	dig, latestProof, err1 := s.keyMap.Put(latLabel.label, mapVal)
	primitive.Assert(!err1)
	latest := &Memb{LabelProof: latLabel.proof, EpochAdded: nextEpoch, PkOpen: open, MerkProof: latestProof}

	// update histInfo.
	upd := make(map[string][]byte)
	upd[string(latLabel.label)] = mapVal
	newHist, sigDig := updEpochHist(s.epochHist, nextEpoch, upd, dig, s.sigSk)
	s.epochHist = newHist

	// make bound.
	_, _, boundProofTy, boundProof, err1 := s.keyMap.Get(boundLabel)
	primitive.Assert(!err1)
	primitive.Assert(!boundProofTy)
	bound := &NonMemb{LabelProof: boundLabelProof, MerkProof: boundProof}
	s.mu.Unlock()
	return sigDig, latest, bound
}

// Get returns a complete history proof for the uid.
// if the uid is not yet registered, it returns a nil memb proof for
// for the latest version.
func (s *Server) Get(uid uint64) (*SigDig, []*MembHide, bool, *Memb, *NonMemb) {
	s.mu.Lock()
	dig := getDig(s.epochHist)
	labels := getLabels(s.uidVerRepo, uid, s.vrfSk)
	hist := getHistVers(s.keyMap, labels)
	isReg, latest := getLatestVer(s.keyMap, labels, s.pkCommOpens)
	bound := getBoundVer(s.keyMap, labels)
	s.mu.Unlock()
	return dig, hist, isReg, latest, bound
}

func (s *Server) SelfMon(uid uint64) (*SigDig, *NonMemb) {
	s.mu.Lock()
	dig := getDig(s.epochHist)
	labels := getLabels(s.uidVerRepo, uid, s.vrfSk)
	bound := getBoundVer(s.keyMap, labels)
	s.mu.Unlock()
	return dig, bound
}

// Audit returns an err on fail.
func (s *Server) Audit(epoch uint64) (*UpdateProof, bool) {
	s.mu.Lock()
	if epoch >= uint64(len(s.epochHist)) {
		s.mu.Unlock()
		return &UpdateProof{Updates: make(map[string][]byte)}, true
	}
	info := s.epochHist[epoch]
	s.mu.Unlock()
	return &UpdateProof{Updates: info.updates, Sig: info.sig}, false
}

func NewServer() (*Server, cryptoffi.SigPublicKey, *cryptoffi.VrfPublicKey) {
	mu := new(sync.Mutex)
	sigPk, sigSk := cryptoffi.SigGenerateKey()
	vrfPk, vrfSk := cryptoffi.VrfGenerateKey()
	m := merkle.NewTree()
	// commit empty tree as init epoch.
	hist, _ := updEpochHist(nil, 0, make(map[string][]byte), m.Digest(), sigSk)
	opens := make(map[string]*CommitOpen)
	labelCache := make(map[uint64][]*vrfCache)
	return &Server{mu: mu, sigSk: sigSk, vrfSk: vrfSk, keyMap: m, epochHist: hist, pkCommOpens: opens, uidVerRepo: labelCache}, sigPk, vrfPk
}

// compMapLabel rets mapLabel (VRF(uid || ver)) and a VRF proof.
func compMapLabel(uid uint64, ver uint64, sk *cryptoffi.VrfPrivateKey) ([]byte, []byte) {
	l := &MapLabelPre{Uid: uid, Ver: ver}
	lByt := MapLabelPreEncode(make([]byte, 0), l)
	h, p := sk.Hash(lByt)
	return h, p
}

// compMapVal rets mapVal (epoch || Hash(pk || rand)).
func compMapVal(epoch uint64, pkOpen *CommitOpen) []byte {
	openByt := CommitOpenEncode(make([]byte, 0), pkOpen)
	commit := cryptoutil.Hash(openByt)
	v := &MapValPre{Epoch: epoch, PkCommit: commit}
	return MapValPreEncode(make([]byte, 0), v)
}

// genCommitOpen generates a commitment opening for val.
func genCommitOpen(val []byte) *CommitOpen {
	// from 8.12 of [Boneh-Shoup] v0.6, a 512-bit rand space provides statistical
	// hiding for this sha256-based commitment scheme.
	// [Boneh-Shoup]: https://toc.cryptobook.us
	r := cryptoffi.RandBytes(2 * cryptoffi.HashLen)
	return &CommitOpen{Val: val, Rand: r}
}

// updEpochHist updates hist at a particular epoch with some new entries
// and a new dig, signing the update with sk.
func updEpochHist(hist []*servEpochInfo, epoch uint64, upd map[string][]byte, dig []byte, sk *cryptoffi.SigPrivateKey) ([]*servEpochInfo, *SigDig) {
	preSig := &PreSigDig{Epoch: epoch, Dig: dig}
	preSigByt := PreSigDigEncode(make([]byte, 0), preSig)
	sig := sk.Sign(preSigByt)
	newInfo := &servEpochInfo{updates: upd, dig: dig, sig: sig}
	return append(hist, newInfo), &SigDig{Epoch: epoch, Dig: dig, Sig: sig}
}

// getLabels gets labels for all existing registered versions of a uid
// and a bound version. it doesn't mutate the provided repo.
func getLabels(uidVerRepo map[uint64][]*vrfCache, uid uint64, sk *cryptoffi.VrfPrivateKey) []*vrfCache {
	labels, ok := uidVerRepo[uid]
	if ok {
		primitive.Assert(len(labels) >= 1)
		return labels
	} else {
		label, proof := compMapLabel(uid, 0, sk)
		return []*vrfCache{{label: label, proof: proof}}
	}
}

func getDig(hist []*servEpochInfo) *SigDig {
	numEpochs := uint64(len(hist))
	lastInfo := hist[numEpochs-1]
	return &SigDig{Epoch: numEpochs - 1, Dig: lastInfo.dig, Sig: lastInfo.sig}
}

// getHistVers returns membership proofs for the history of versions
// up until the latest.
func getHistVers(keyMap *merkle.Tree, labels []*vrfCache) []*MembHide {
	numRegVers := uint64(len(labels)) - 1
	if numRegVers == 0 {
		return nil
	}
	// latest registered ver not included in hist.
	var hist = make([]*MembHide, 0, numRegVers-1)
	for ver := uint64(0); ver < numRegVers-1; ver++ {
		label := labels[ver]
		mapVal, _, proofTy, proof, err0 := keyMap.Get(label.label)
		primitive.Assert(!err0)
		primitive.Assert(proofTy)
		hist = append(hist, &MembHide{LabelProof: label.proof, MapVal: mapVal, MerkProof: proof})
	}
	return hist
}

// getLatestVer returns whether a version is registered, and if so,
// a membership proof for the latest version.
func getLatestVer(keyMap *merkle.Tree, labels []*vrfCache, opens map[string]*CommitOpen) (bool, *Memb) {
	numRegVers := uint64(len(labels)) - 1
	if numRegVers == 0 {
		return false, &Memb{PkOpen: &CommitOpen{}}
	} else {
		label := labels[numRegVers-1]
		mapVal, _, proofTy, proof, err0 := keyMap.Get(label.label)
		primitive.Assert(!err0)
		primitive.Assert(proofTy)
		valPre, _, err1 := MapValPreDecode(mapVal)
		primitive.Assert(!err1)
		open, ok0 := opens[string(label.label)]
		primitive.Assert(ok0)
		return true, &Memb{LabelProof: label.proof, EpochAdded: valPre.Epoch, PkOpen: open, MerkProof: proof}
	}
}

// getBoundVer returns a non-membership proof for the boundary version.
func getBoundVer(keyMap *merkle.Tree, labels []*vrfCache) *NonMemb {
	boundVer := uint64(len(labels)) - 1
	label := labels[boundVer]
	_, _, proofTy, proof, err0 := keyMap.Get(label.label)
	primitive.Assert(!err0)
	primitive.Assert(!proofTy)
	return &NonMemb{LabelProof: label.proof, MerkProof: proof}
}
