package kt

import (
	"github.com/goose-lang/primitive"
	"github.com/goose-lang/std"
	"github.com/mit-pdos/pav/advrpc"
	"sync"
)

const (
	aliceUid uint64 = 0
	bobUid   uint64 = 1
)

func testAllFull(servAddr uint64, adtrAddrs []uint64) {
	testAll(setup(servAddr, adtrAddrs))
}

func testAll(setup *setupParams) {
	aliceCli := newClient(aliceUid, setup.servAddr, setup.servSigPk, setup.servVrfPk)
	alice := &alice{cli: aliceCli}
	bobCli := newClient(bobUid, setup.servAddr, setup.servSigPk, setup.servVrfPk)
	bob := &bob{cli: bobCli}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Add(1)
	// alice does a bunch of puts.
	go func() {
		alice.run()
		wg.Done()
	}()
	// bob does a get at some time in the middle of alice's puts.
	go func() {
		bob.run()
		wg.Done()
	}()
	wg.Wait()

	// alice self monitor. in real world, she'll come on-line at times and do this.
	selfMonEp, err0 := alice.cli.SelfMon()
	primitive.Assume(!err0.err)
	// this last self monitor will be our history bound.
	primitive.Assume(bob.epoch <= selfMonEp)

	// sync auditors. in real world, this'll happen periodically.
	updAdtrsAll(setup.servAddr, setup.adtrAddrs)

	// alice and bob audit. ordering irrelevant across clients.
	doAudits(alice.cli, setup.adtrAddrs, setup.adtrPks)
	doAudits(bob.cli, setup.adtrAddrs, setup.adtrPks)

	// final check. bob got the right key.
	isReg, alicePk := GetHist(alice.hist, bob.epoch)
	primitive.Assert(isReg == bob.isReg)
	if isReg {
		primitive.Assert(std.BytesEqual(alicePk, bob.alicePk))
	}
}

type alice struct {
	cli  *Client
	hist []*HistEntry
}

func (a *alice) run() {
	for i := uint64(0); i < uint64(20); i++ {
		primitive.Sleep(5_000_000)
		pk := []byte{byte(i)}
		epoch, err0 := a.cli.Put(pk)
		primitive.Assume(!err0.err)
		a.hist = append(a.hist, &HistEntry{Epoch: epoch, HistVal: pk})
	}
}

type bob struct {
	cli     *Client
	epoch   uint64
	isReg   bool
	alicePk []byte
}

func (b *bob) run() {
	primitive.Sleep(120_000_000)
	isReg, pk, epoch, err0 := b.cli.Get(aliceUid)
	primitive.Assume(!err0.err)
	b.epoch = epoch
	b.isReg = isReg
	b.alicePk = pk
}

func updAdtrsAll(servAddr uint64, adtrAddrs []uint64) {
	servCli := advrpc.Dial(servAddr)
	adtrs := mkRpcClients(adtrAddrs)
	var epoch uint64
	for {
		upd, err := callServAudit(servCli, epoch)
		if err {
			break
		}
		updAdtrsOnce(upd, adtrs)
		epoch++
	}
}
