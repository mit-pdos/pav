package cryptoutil

import (
	"bytes"
	"github.com/mit-pdos/pav/cryptoffi"
	"testing"
)

func TestHasher(t *testing.T) {
	str := []byte("hello")
	var hr1 Hasher
	HasherWrite(&hr1, str)
	hash1 := HasherSum(hr1, nil)
	var hr2 Hasher
	hash2 := HasherSum(hr2, nil)
	hash3 := cryptoffi.Hash(str)
	hash4 := cryptoffi.Hash(nil)

	if !bytes.Equal(hash1, hash3) {
		t.Fatal()
	}
	if !bytes.Equal(hash2, hash4) {
		t.Fatal()
	}
	if bytes.Equal(hash1, hash2) {
		t.Fatal()
	}
	if uint64(len(hash2)) != cryptoffi.HashLen {
		t.Fatal()
	}
}
