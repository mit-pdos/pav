package main

import (
	"fmt"
	"github.com/mit-pdos/gokv/grove_ffi"
	"github.com/mit-pdos/secure-chat/full2"
	"github.com/mit-pdos/secure-chat/full2/fc_ffi"
	"github.com/tchajed/goose/machine"
	"sync"
	"time"
)

func main() {
	c := fc_ffi.Init()
	skA, pkA, err := c.MakeKeys()
	machine.Assume(err == full2.ErrNone)
	skB, pkB, err := c.MakeKeys()
	machine.Assume(err == full2.ErrNone)
	var pks = make([]*fc_ffi.VerifierT, 2)
	pks[full2.AliceNum] = pkA
	pks[full2.BobNum] = pkB

	addr := grove_ffi.MakeAddress("0.0.0.0:6060")
	var retA uint64
	var retB uint64
	aEvent := make(chan struct{})
	bEvent := make(chan struct{})
	serverStartup := 10 * time.Millisecond

	var wg sync.WaitGroup
	go func() {
		s := full2.MakeServer()
		s.Start(addr)
	}()
	wg.Add(1)
	go func() {
		time.Sleep(serverStartup)
		a := full2.MakeAlice(addr, skA, pks)
		a.One()
		aEvent <- struct{}{}

		<-bEvent
		retA = a.Two()
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		time.Sleep(serverStartup)
		<-aEvent
		b := full2.MakeBob(addr, skB, pks)
		retB = b.One()
		bEvent <- struct{}{}
		wg.Done()
	}()
	wg.Wait()

	fmt.Println("retA:", retA)
	fmt.Println("retB:", retB)
	machine.Assert(retA == retB && retA != 0)
}
