package cryptoffi

import (
	"crypto/sha256"
	"github.com/mit-pdos/pav/benchutil"
	"math/rand/v2"
	"testing"
	"time"
)

func TestBenchRand(t *testing.T) {
	var seed [32]byte
	rnd := rand.NewChaCha8(seed)
	data := make([]byte, 64)

	nOps := 100_000_000
	start := time.Now()
	for i := 0; i < nOps; i++ {
		rnd.Read(data)
	}
	total := time.Since(start)

	m0 := float64(total.Nanoseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "ns/op"},
		{N: m1, Unit: "total ms"},
	})
}

func TestBenchRandHash(t *testing.T) {
	var seed [32]byte
	rnd := rand.NewChaCha8(seed)
	data := make([]byte, 64)

	nOps := 10_000_000
	start := time.Now()
	for i := 0; i < nOps; i++ {
		rnd.Read(data)
		sha256.Sum256(data)
	}
	total := time.Since(start)

	m0 := float64(total.Nanoseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "ns/op"},
		{N: m1, Unit: "total ms"},
	})
}

func TestBenchVrfHash(t *testing.T) {
	_, sk := VrfGenerateKey()
	var seed [32]byte
	rnd := rand.NewChaCha8(seed)
	data := make([]byte, 32)

	nOps := 30_000
	start := time.Now()
	for i := 0; i < nOps; i++ {
		rnd.Read(data)
		sk.Hash(data)
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total ms"},
	})
}

func TestBenchVrfVerify(t *testing.T) {
	pk, sk := VrfGenerateKey()
	var seed [32]byte
	rnd := rand.NewChaCha8(seed)
	data := make([]byte, 32)

	nOps := 20_000
	start := time.Now()
	for i := 0; i < nOps; i++ {
		// note: random proofs each iteration seem to help
		// stabilize the results.
		rnd.Read(data)
		_, p := sk.Hash(data)
		pk.Verify(data, p)
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total ms"},
	})
}
