package merkle

import (
	"bytes"
	"fmt"
	"github.com/zeebo/blake3"
)

const (
	ErrNone      uint64 = 0
	ErrFound     uint64 = 1
	ErrNotFound  uint64 = 2
	ErrBadInput  uint64 = 3
	ErrPathProof uint64 = 4
	HashLen             = 32
	// Branch on a byte. 2 ** 8 (bits in byte) = 256.
	NumChildren = 256
)

func HashOne(d []byte) []byte {
	hasher := blake3.New()
	hasher.Write(d)
	return hasher.Sum(nil)[:HashLen]
}

func HashSum(h *blake3.Hasher) []byte {
	return h.Sum(nil)[:HashLen]
}

func HashSlice2D(b1 [][]byte) []byte {
	h := blake3.New()
	for _, b2 := range b1 {
		h.Write(b2)
	}
	return HashSum(h)
}

func HashNodes(nodeSl []*Node) []byte {
	h := blake3.New()
	for _, n := range nodeSl {
		h.Write(n.Hash())
	}
	return HashSum(h)
}

func CopySlice(b1 []byte) []byte {
	b2 := make([]byte, len(b1))
	copy(b2, b1)
	return b2
}

// "keys" of the tree.
// We use "Id" to differentiate this from the public keys that could be stored
// in the tree.
type Id struct {
	B []byte
}

// "vals" of the tree.
type Val struct {
	B []byte
}

type Node struct {
	Val      *Val
	hash     []byte
	Children []*Node
}

func NewNode() *Node {
	d := HashOne(nil)
	c := make([]*Node, NumChildren)
	return &Node{Val: nil, hash: d, Children: c}
}

func (n *Node) Hash() []byte {
	if n == nil {
		return HashOne(nil)
	}
	return n.hash
}

type Digest struct {
	B []byte
}

// General proof object.
// Binds an id down the tree to a particular node hash.
type PathProof struct {
	Id          *Id
	NodeHash    []byte
	Digest      *Digest
	ChildHashes [][][]byte
}

type MembProof struct {
	ChildHashes [][][]byte
}

type NonmembProof struct {
	ChildHashes [][][]byte
}

func (p *PathProof) Check() uint64 {
	proofLen := len(p.Id.B)
	posBott := p.Id.B[proofLen-1]
	if !bytes.Equal(p.NodeHash, p.ChildHashes[proofLen-1][posBott]) {
		return ErrPathProof
	}

	err := ErrNone
	for pathIdx := proofLen - 1; pathIdx >= 1; pathIdx-- {
		hChildren := HashSlice2D(p.ChildHashes[pathIdx])
		prevIdx := pathIdx - 1
		pos := p.Id.B[prevIdx]
		if !bytes.Equal(hChildren, p.ChildHashes[prevIdx][pos]) {
			err = ErrPathProof
		}
	}
	if err != ErrNone {
		return err
	}

	digest := HashSlice2D(p.ChildHashes[0])
	if !bytes.Equal(digest, p.Digest.B) {
		return ErrPathProof
	}
	return ErrNone
}

func (p *MembProof) Check(id *Id, val *Val, digest *Digest) uint64 {
	if len(id.B) != HashLen {
		return ErrBadInput
	}
	if len(p.ChildHashes) != HashLen {
		return ErrBadInput
	}
	pathProof := &PathProof{
		Id:          id,
		NodeHash:    HashOne(val.B),
		Digest:      digest,
		ChildHashes: p.ChildHashes,
	}
	return pathProof.Check()
}

func (p *NonmembProof) Check(id *Id, digest *Digest) uint64 {
	if HashLen <= len(p.ChildHashes) {
		return ErrBadInput
	}
	idPref := &Id{B: CopySlice(id.B)[:len(p.ChildHashes)]}
	pathProof := &PathProof{
		Id:          idPref,
		NodeHash:    HashOne(nil),
		Digest:      digest,
		ChildHashes: p.ChildHashes,
	}
	return pathProof.Check()
}

// Assumes recursive child hashes are already up-to-date.
func (n *Node) UpdateHash() {
	if n.Val != nil {
		n.hash = HashOne(n.Val.B)
	} else {
		n.hash = HashNodes(n.Children)
	}
}

type Tree struct {
	Root *Node
}

func NewTree() *Tree {
	return &Tree{Root: NewNode()}
}

func (t *Tree) Print() {
	qCurr := make([]*Node, 0)
	qCurr = append(qCurr, t.Root)
	qNext := make([]*Node, 0)
	for len(qCurr) > 0 {
		for len(qCurr) > 0 {
			top := qCurr[0]
			qCurr = qCurr[1:]

			if top == nil {
				fmt.Print("nil | ")
				continue
			} else if top.Val != nil {
				fmt.Print(top.Hash(), top.Val.B, " | ")
			} else {
				fmt.Print(top.Hash(), " | ")
			}

			for _, child := range top.Children {
				qNext = append(qNext, child)
			}
		}
		qCurr = qNext
		qNext = nil
		fmt.Println()
	}
}

func GetChildHashes(nodePath []*Node) [][][]byte {
	childHashes := make([][][]byte, len(nodePath))
	for pathIdx := 0; pathIdx < len(nodePath); pathIdx++ {
		treeChildren := nodePath[pathIdx].Children
		proofChildren := make([][]byte, NumChildren)
		childHashes[pathIdx] = proofChildren

		for childIdx := 0; childIdx < NumChildren; childIdx++ {
			proofChildren[childIdx] = CopySlice(treeChildren[childIdx].Hash())
		}
	}
	return childHashes
}

func (t *Tree) WalkTree(id *Id) ([]*Node, bool) {
	var nodePath []*Node
	nodePath = append(nodePath, t.Root)
	found := true
	for pathIdx := 0; pathIdx < HashLen && found; pathIdx++ {
		currNode := nodePath[pathIdx]
		pos := id.B[pathIdx]
		if currNode.Children[pos] == nil {
			found = false
		} else {
			nodePath = append(nodePath, currNode.Children[pos])
		}
	}
	return nodePath, found
}

func (t *Tree) WalkTreeAddLinks(id *Id) []*Node {
	var nodePath []*Node
	nodePath = append(nodePath, t.Root)
	for pathIdx := 0; pathIdx < HashLen; pathIdx++ {
		currNode := nodePath[pathIdx]
		pos := id.B[pathIdx]
		if currNode.Children[pos] == nil {
			currNode.Children[pos] = NewNode()
		}
		nodePath = append(nodePath, currNode.Children[pos])
	}
	return nodePath
}

func (t *Tree) Put(id *Id, v *Val) (*Digest, *MembProof, uint64) {
	if len(id.B) != HashLen {
		return nil, nil, ErrBadInput
	}

	nodePath := t.WalkTreeAddLinks(id)
	nodePath[HashLen].Val = v
	for pathIdx := HashLen; pathIdx >= 0; pathIdx-- {
		nodePath[pathIdx].UpdateHash()
	}

	digest := &Digest{B: CopySlice(nodePath[0].Hash())}
	proof := &MembProof{ChildHashes: GetChildHashes(nodePath[:HashLen])}
	return digest, proof, ErrNone
}

func (t *Tree) Get(id *Id) (*Val, *Digest, *MembProof, uint64) {
	if len(id.B) != HashLen {
		return nil, nil, nil, ErrBadInput
	}

	nodePath, found := t.WalkTree(id)
	if !found {
		return nil, nil, nil, ErrNotFound
	}

	val := &Val{B: CopySlice(nodePath[HashLen].Val.B)}
	digest := &Digest{B: CopySlice(nodePath[0].Hash())}
	proof := &MembProof{ChildHashes: GetChildHashes(nodePath[:HashLen])}
	return val, digest, proof, ErrNone
}

func (t *Tree) GetNil(id *Id) (*Digest, *NonmembProof, uint64) {
	if len(id.B) != HashLen {
		return nil, nil, ErrBadInput
	}

	nodePath, found := t.WalkTree(id)
	if found {
		return nil, nil, ErrFound
	}

	digest := &Digest{B: CopySlice(nodePath[0].Hash())}
	proof := &NonmembProof{ChildHashes: GetChildHashes(nodePath)}
	return digest, proof, ErrNone
}
