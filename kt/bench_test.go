package kt

// NOTE: for server benches that strive for akd compat,
// look for "benchmark:" in source files to find where to remove signatures.

import (
	"log"
	"math/rand/v2"
	"net"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aclements/go-moremath/stats"
	"github.com/elastic/go-sysinfo"
	"github.com/mit-pdos/pav/benchutil"
	"github.com/mit-pdos/pav/cryptoffi"
)

const (
	defNSeed uint64  = 1_000_000
	nsPerUs  float64 = 1_000
)

func TestBenchLabelGenVer(t *testing.T) {
	pk, sk := cryptoffi.VrfGenerateKey()
	nOps := 50_000

	var totalGen time.Duration
	var totalVer time.Duration
	for i := 0; i < nOps; i++ {
		uid := rand.Uint64()
		ver := uint64(0)

		t0 := time.Now()
		_, p := compMapLabel(uid, ver, sk)

		t1 := time.Now()
		_, err := checkLabel(pk, uid, ver, p)
		if err {
			t.Fatal()
		}
		t2 := time.Now()

		totalGen += t1.Sub(t0)
		totalVer += t2.Sub(t1)
	}

	m0 := float64(totalGen.Microseconds()) / float64(nOps)
	m1 := float64(totalGen.Milliseconds())
	m2 := float64(totalVer.Microseconds()) / float64(nOps)
	m3 := float64(totalVer.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op(gen)"},
		{N: m1, Unit: "total(ms,gen)"},
		{N: m2, Unit: "us/op(ver)"},
		{N: m3, Unit: "total(ms,ver)"},
	})
}

func TestBenchPutGenVer(t *testing.T) {
	serv, _, vrfPk, _ := seedServer(defNSeed)
	nOps := 10_000
	nWarm := getWarmup(nOps)

	var totalGen time.Duration
	var totalVer time.Duration
	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			totalGen = 0
			totalVer = 0
		}
		uid := rand.Uint64()

		t0 := time.Now()
		dig, lat, bound, err := serv.Put(uid, mkRandVal())
		if err {
			t.Fatal()
		}

		t1 := time.Now()
		if checkMemb(vrfPk, uid, 0, dig.Dig, lat) {
			t.Fatal()
		}
		if checkNonMemb(vrfPk, uid, 1, dig.Dig, bound) {
			t.Fatal()
		}
		t2 := time.Now()

		totalGen += t1.Sub(t0)
		totalVer += t2.Sub(t1)
	}

	m0 := float64(totalGen.Microseconds()) / float64(nOps)
	m1 := float64(totalGen.Milliseconds())
	m2 := float64(totalVer.Microseconds()) / float64(nOps)
	m3 := float64(totalVer.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op(gen)"},
		{N: m1, Unit: "total(ms,gen)"},
		{N: m2, Unit: "us/op(ver)"},
		{N: m3, Unit: "total(ms,ver)"},
	})
}

func TestBenchPutScale(t *testing.T) {
	// need lots of clients to hit max workq rate.
	maxNCli := 200
	runner := newClientRunner(maxNCli)

	for nCli := 1; nCli <= maxNCli; nCli++ {
		serv, _, _, _ := seedServer(defNSeed)
		totalTime := runner.run(nCli, func() {
			serv.Put(rand.Uint64(), mkRandVal())
		})

		ops := int(runner.sample.Weight())
		tput := float64(ops) / totalTime.Seconds()
		benchutil.Report(nCli, []*benchutil.Metric{
			{N: tput, Unit: "op/s"},
			{N: runner.sample.Mean() / nsPerUs, Unit: "mean(us)"},
			{N: runner.sample.StdDev() / nsPerUs, Unit: "stddev"},
			{N: runner.sample.Quantile(0.99) / nsPerUs, Unit: "p99"},
		})
	}
}

func TestBenchPutBatch(t *testing.T) {
	cfgs := []batchCfg{
		{1, 50_000},
		{2, 50_000},
		{5, 10_000},
		{10, 10_000},
		{20, 10_000},
		{50, 5_000},
		{100, 3_000},
		{200, 1_500},
		{500, 500},
		{1_000, 300},
	}
	for _, c := range cfgs {
		total := putBatchHelper(c.batchSz, c.nBatches)
		m0 := float64(c.batchSz*c.nBatches) / total.Seconds()
		m1 := float64(total.Milliseconds())
		benchutil.Report(c.batchSz, []*benchutil.Metric{
			{N: m0, Unit: "op/s"},
			{N: m1, Unit: "total(ms)"},
		})
	}
}

func putBatchHelper(batchSz, nBatches int) time.Duration {
	serv, _, _, _ := seedServer(defNSeed)
	nWarm := getWarmup(nBatches)

	start := time.Now()
	for i := 0; i < nWarm+nBatches; i++ {
		if i == nWarm {
			start = time.Now()
		}
		reqs := make([]*WQReq, 0, batchSz)
		for i := 0; i < batchSz; i++ {
			reqs = append(reqs, &WQReq{Uid: rand.Uint64(), Pk: mkRandVal()})
		}
		serv.workQ.DoBatch(reqs)
	}
	return time.Since(start)
}

func TestBenchPutSize(t *testing.T) {
	serv, _, _, _ := seedServer(defNSeed)
	u := rand.Uint64()
	dig, lat, bound, err := serv.Put(u, mkRandVal())
	if err {
		t.Fatal()
	}
	p := &ServerPutReply{Dig: dig, Latest: lat, Bound: bound, Err: err}
	pb := ServerPutReplyEncode(nil, p)
	benchutil.Report(1, []*benchutil.Metric{
		{N: float64(len(pb)), Unit: "B"},
	})
}

func TestBenchPutCli(t *testing.T) {
	serv, sigPk, vrfPk, _ := seedServer(defNSeed)
	vrfPkB := cryptoffi.VrfPublicKeyEncode(vrfPk)
	servRpc := NewRpcServer(serv)
	servAddr := makeUniqueAddr()
	servRpc.Serve(servAddr)
	time.Sleep(time.Millisecond)
	nOps := 10_000
	nWarm := getWarmup(nOps)

	clients := make([]*Client, 0, nWarm+nOps)
	for i := 0; i < nWarm+nOps; i++ {
		u := rand.Uint64()
		c := NewClient(u, servAddr, sigPk, vrfPkB)
		clients = append(clients, c)
	}

	var start time.Time
	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			start = time.Now()
		}
		_, err := clients[i].Put(mkRandVal())
		if err.Err {
			t.Fatal()
		}
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total(ms)"},
	})
}

func TestBenchGetGenVer(t *testing.T) {
	serv, _, vrfPk, uids := seedServer(defNSeed)
	nOps := 10_000
	nWarm := getWarmup(nOps)
	var totalGen time.Duration
	var totalVer time.Duration

	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			totalGen = 0
			totalVer = 0
		}
		uid := uids[rand.Uint64N(defNSeed)]

		t0 := time.Now()
		dig, hist, isReg, lat, bound := serv.Get(uid)
		if !isReg {
			t.Fatal()
		}
		if len(hist) != 0 {
			t.Fatal()
		}

		t1 := time.Now()
		if checkHist(vrfPk, uid, dig.Dig, hist) {
			t.Fatal()
		}
		if checkMemb(vrfPk, uid, 0, dig.Dig, lat) {
			t.Fatal()
		}
		if checkNonMemb(vrfPk, uid, 1, dig.Dig, bound) {
			t.Fatal()
		}
		t2 := time.Now()

		totalGen += t1.Sub(t0)
		totalVer += t2.Sub(t1)
	}

	m0 := float64(totalGen.Microseconds()) / float64(nOps)
	m1 := float64(totalGen.Milliseconds())
	m2 := float64(totalVer.Microseconds()) / float64(nOps)
	m3 := float64(totalVer.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op(gen)"},
		{N: m1, Unit: "total(ms,gen)"},
		{N: m2, Unit: "us/op(ver)"},
		{N: m3, Unit: "total(ms,ver)"},
	})
}

func TestBenchGetScale(t *testing.T) {
	serv, _, _, uids := seedServer(defNSeed)
	maxNCli := runtime.NumCPU()
	runner := newClientRunner(maxNCli)

	for nCli := 1; nCli <= maxNCli; nCli++ {
		totalTime := runner.run(nCli, func() {
			u := uids[rand.Uint64N(defNSeed)]
			serv.Get(u)
		})

		ops := int(runner.sample.Weight())
		tput := float64(ops) / totalTime.Seconds()
		benchutil.Report(nCli, []*benchutil.Metric{
			{N: tput, Unit: "op/s"},
			{N: runner.sample.Mean() / nsPerUs, Unit: "mean(us)"},
			{N: runner.sample.StdDev() / nsPerUs, Unit: "stddev"},
			{N: runner.sample.Quantile(0.99) / nsPerUs, Unit: "p99"},
		})
	}
}

func TestBenchGetSize(t *testing.T) {
	serv, _, _, uids := seedServer(defNSeed)
	dig, hist, isReg, lat, bound := serv.Get(uids[0])
	if !isReg {
		t.Fatal()
	}
	p := &ServerGetReply{Dig: dig, Hist: hist, IsReg: isReg, Latest: lat, Bound: bound}
	pb := ServerGetReplyEncode(nil, p)
	benchutil.Report(1, []*benchutil.Metric{
		{N: float64(len(pb)), Unit: "B"},
	})
}

func TestBenchGetCli(t *testing.T) {
	serv, sigPk, vrfPk, uids := seedServer(defNSeed)
	vrfPkB := cryptoffi.VrfPublicKeyEncode(vrfPk)
	servRpc := NewRpcServer(serv)
	servAddr := makeUniqueAddr()
	servRpc.Serve(servAddr)
	time.Sleep(time.Millisecond)
	cli := NewClient(rand.Uint64(), servAddr, sigPk, vrfPkB)
	nOps := 10_000
	nWarm := getWarmup(nOps)

	var start time.Time
	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			start = time.Now()
		}
		uid := uids[rand.Uint64N(defNSeed)]
		isReg, _, _, err := cli.Get(uid)
		if err.Err {
			t.Fatal()
		}
		if !isReg {
			t.Fatal()
		}
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total(ms)"},
	})
}

func TestBenchSelfMonGenVer(t *testing.T) {
	serv, _, vrfPk, uids := seedServer(defNSeed)
	nOps := 20_000
	nWarm := getWarmup(nOps)
	var totalGen time.Duration
	var totalVer time.Duration

	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			totalGen = 0
			totalVer = 0
		}
		uid := uids[rand.Uint64N(defNSeed)]

		t0 := time.Now()
		dig, bound := serv.SelfMon(uid)

		t1 := time.Now()
		if checkNonMemb(vrfPk, uid, 1, dig.Dig, bound) {
			t.Fatal()
		}
		t2 := time.Now()

		totalGen += t1.Sub(t0)
		totalVer += t2.Sub(t1)
	}

	m0 := float64(totalGen.Microseconds()) / float64(nOps)
	m1 := float64(totalGen.Milliseconds())
	m2 := float64(totalVer.Microseconds()) / float64(nOps)
	m3 := float64(totalVer.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op(gen)"},
		{N: m1, Unit: "total(ms,gen)"},
		{N: m2, Unit: "us/op(ver)"},
		{N: m3, Unit: "total(ms,ver)"},
	})
}

func TestBenchSelfMonScale(t *testing.T) {
	serv, _, _, uids := seedServer(defNSeed)
	maxNCli := runtime.NumCPU()
	runner := newClientRunner(maxNCli)

	for nCli := 1; nCli <= maxNCli; nCli++ {
		totalTime := runner.run(nCli, func() {
			u := uids[rand.Uint64N(defNSeed)]
			serv.SelfMon(u)
		})

		ops := int(runner.sample.Weight())
		tput := float64(ops) / totalTime.Seconds()
		benchutil.Report(nCli, []*benchutil.Metric{
			{N: tput, Unit: "op/s"},
			{N: runner.sample.Mean() / nsPerUs, Unit: "mean(us)"},
			{N: runner.sample.StdDev() / nsPerUs, Unit: "stddev"},
			{N: runner.sample.Quantile(0.99) / nsPerUs, Unit: "p99"},
		})
	}
}

func TestBenchSelfMonSize(t *testing.T) {
	serv, _, _, uids := seedServer(defNSeed)
	dig, bound := serv.SelfMon(uids[0])
	p := &ServerSelfMonReply{Dig: dig, Bound: bound}
	pb := ServerSelfMonReplyEncode(nil, p)
	benchutil.Report(1, []*benchutil.Metric{
		{N: float64(len(pb)), Unit: "B"},
	})
}

func TestBenchSelfMonCli(t *testing.T) {
	serv, sigPk, vrfPk, _ := seedServer(defNSeed)
	vrfPkB := cryptoffi.VrfPublicKeyEncode(vrfPk)
	servRpc := NewRpcServer(serv)
	servAddr := makeUniqueAddr()
	servRpc.Serve(servAddr)
	time.Sleep(time.Millisecond)
	nOps := 20_000
	nWarm := getWarmup(nOps)

	clients := make([]*Client, 0, nWarm+nOps)
	wg := new(sync.WaitGroup)
	wg.Add(nWarm + nOps)
	for i := 0; i < nWarm+nOps; i++ {
		u := rand.Uint64()
		c := NewClient(u, servAddr, sigPk, vrfPkB)
		clients = append(clients, c)
		go func() {
			_, err := c.Put(mkRandVal())
			if err.Err {
				t.Error()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	var start time.Time
	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			start = time.Now()
		}
		_, err := clients[i].SelfMon()
		if err.Err {
			t.Fatal()
		}
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total(ms)"},
	})
}

type batchCfg struct {
	batchSz  int
	nBatches int
}

func TestBenchAuditScale(t *testing.T) {
	cfgs := []batchCfg{
		{1, 50_000},
		{2, 50_000},
		{5, 10_000},
		{10, 10_000},
		{20, 10_000},
		{50, 5_000},
		{100, 3_000},
		{200, 1_500},
		{500, 500},
		{1_000, 300},
	}
	for _, c := range cfgs {
		totalGen, totalVer := auditScaleHelper(t, c.batchSz, c.nBatches)
		m0 := float64(c.batchSz*c.nBatches) / totalGen.Seconds()
		m1 := float64(totalGen.Milliseconds())
		m2 := float64(c.batchSz*c.nBatches) / totalVer.Seconds()
		m3 := float64(totalVer.Milliseconds())
		benchutil.Report(c.batchSz, []*benchutil.Metric{
			{N: m0, Unit: "op/s(gen)"},
			{N: m1, Unit: "total(ms,gen)"},
			{N: m2, Unit: "op/s(ver)"},
			{N: m3, Unit: "total(ms,ver)"},
		})
	}
}

func auditScaleHelper(t *testing.T, batchSz, nBatches int) (time.Duration, time.Duration) {
	serv, _, _, _ := seedServer(defNSeed)
	aud, _ := NewAuditor()
	epoch := updAuditor(t, serv, aud, 0)
	nWarm := getWarmup(nBatches)

	var totalGen time.Duration
	var totalVer time.Duration
	for i := 0; i < nWarm+nBatches; i++ {
		if i == nWarm {
			totalGen = 0
			totalVer = 0
		}
		reqs := make([]*WQReq, 0, batchSz)
		for j := 0; j < batchSz; j++ {
			req := &WQReq{Uid: rand.Uint64(), Pk: mkRandVal()}
			reqs = append(reqs, req)
		}
		serv.workQ.DoBatch(reqs)

		for ; ; epoch++ {
			t0 := time.Now()
			p, err := serv.Audit(epoch)
			if err {
				break
			}
			t1 := time.Now()
			if err = aud.Update(p); err {
				t.Fatal()
			}
			t2 := time.Now()

			totalGen += t1.Sub(t0)
			totalVer += t2.Sub(t1)
		}
	}
	return totalGen, totalVer
}

func TestBenchAuditSize(t *testing.T) {
	serv, _, _, _ := seedServer(defNSeed)
	var epoch uint64
	for ; ; epoch++ {
		_, err := serv.Audit(epoch)
		if err {
			break
		}
	}

	serv.Put(rand.Uint64(), mkRandVal())
	upd, err := serv.Audit(epoch)
	if err {
		t.Fatal()
	}
	p := &ServerAuditReply{P: upd, Err: err}
	pb := ServerAuditReplyEncode(nil, p)

	epoch++
	_, err = serv.Audit(epoch)
	// prev updates should have all been processed in 1 epoch.
	if !err {
		t.Fatal()
	}

	benchutil.Report(1, []*benchutil.Metric{
		{N: float64(len(pb)), Unit: "B"},
	})
}

func TestBenchAuditCli(t *testing.T) {
	serv, sigPk, vrfPk, _ := seedServer(defNSeed)
	vrfPkB := cryptoffi.VrfPublicKeyEncode(vrfPk)
	servRpc := NewRpcServer(serv)
	servAddr := makeUniqueAddr()
	servRpc.Serve(servAddr)
	time.Sleep(time.Millisecond)
	nOps := 10_000
	nWarm := getWarmup(nOps)

	// after putting 1 key, a client knows about 1 epoch.
	clients := make([]*Client, 0, nWarm+nOps)
	wg := new(sync.WaitGroup)
	wg.Add(nWarm + nOps)
	for i := 0; i < nWarm+nOps; i++ {
		c := NewClient(rand.Uint64(), servAddr, sigPk, vrfPkB)
		clients = append(clients, c)

		go func() {
			_, err := c.Put(mkRandVal())
			if err.Err {
				t.Error()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	aud, audPk := NewAuditor()
	updAuditor(t, serv, aud, 0)
	audRpc := NewRpcAuditor(aud)
	audAddr := makeUniqueAddr()
	audRpc.Serve(audAddr)
	time.Sleep(time.Millisecond)

	var start time.Time
	for i := 0; i < nWarm+nOps; i++ {
		if i == nWarm {
			start = time.Now()
		}
		err := clients[i].Audit(audAddr, audPk)
		if err.Err {
			t.Fatal()
		}
	}
	total := time.Since(start)

	m0 := float64(total.Microseconds()) / float64(nOps)
	m1 := float64(total.Milliseconds())
	benchutil.Report(nOps, []*benchutil.Metric{
		{N: m0, Unit: "us/op"},
		{N: m1, Unit: "total(ms)"},
	})
}

func TestBenchServMem(t *testing.T) {
	serv, _, _ := NewServer()
	nTotal := 500_000_000
	nMeasure := 1_000_000

	for i := 0; i < nTotal; i += nMeasure {
		runtime.GC()
		proc, err := sysinfo.Self()
		if err != nil {
			t.Fatal(err)
		}
		mem, err := proc.Memory()
		if err != nil {
			t.Fatal(err)
		}
		mb := float64(mem.Resident) / float64(1_000_000)
		benchutil.Report(i, []*benchutil.Metric{
			{N: mb, Unit: "MB"},
		})

		reqs := make([]*WQReq, 0, nMeasure)
		for j := 0; j < nMeasure; j++ {
			req := &WQReq{Uid: rand.Uint64(), Pk: mkRandVal()}
			reqs = append(reqs, req)
		}
		serv.workQ.DoBatch(reqs)
	}
}

func updAuditor(t *testing.T, serv *Server, aud *Auditor, epoch uint64) uint64 {
	for ; ; epoch++ {
		p, err := serv.Audit(epoch)
		if err {
			break
		}
		if err = aud.Update(p); err {
			t.Fatal()
		}
	}
	return epoch
}

func seedServer(nSeed uint64) (*Server, cryptoffi.SigPublicKey, *cryptoffi.VrfPublicKey, []uint64) {
	serv, sigPk, vrfPk := NewServer()
	uids := make([]uint64, 0, nSeed)

	// use multiple epochs for akd bench parity.
	nEp := uint64(65_536)
	if nSeed < nEp {
		log.Fatal("nSeed too small")
	}
	for i := uint64(0); i < nEp; i++ {
		u := rand.Uint64()
		uids = append(uids, u)
		serv.workQ.Do(&WQReq{Uid: u, Pk: mkRandVal()})
	}

	reqs := make([]*WQReq, 0, nSeed-nEp)
	for i := uint64(0); i < nSeed-nEp; i++ {
		u := rand.Uint64()
		uids = append(uids, u)
		reqs = append(reqs, &WQReq{Uid: u, Pk: mkRandVal()})
	}
	serv.workQ.DoBatch(reqs)
	runtime.GC()
	return serv, sigPk, vrfPk, uids
}

type clientRunner struct {
	times  [][]startEnd
	sample *stats.Sample
}

type startEnd struct {
	start time.Time
	end   time.Time
}

func newClientRunner(maxNCli int) *clientRunner {
	var times [][]startEnd
	for i := 0; i < maxNCli; i++ {
		times = append(times, make([]startEnd, 0, 1_000_000))
	}
	sample := &stats.Sample{Xs: make([]float64, 0, 10_000_000)}
	return &clientRunner{times: times, sample: sample}
}

func (c *clientRunner) run(nCli int, work func()) time.Duration {
	// get data.
	for i := 0; i < nCli; i++ {
		c.times[i] = c.times[i][:0]
	}
	wg := new(sync.WaitGroup)
	wg.Add(nCli)
	for i := 0; i < nCli; i++ {
		go func() {
			cliStart := time.Now()
			for {
				s := time.Now()
				work()
				e := time.Now()
				c.times[i] = append(c.times[i], startEnd{start: s, end: e})

				if e.Sub(cliStart) >= 2*time.Second {
					wg.Done()
					break
				}
			}
		}()
	}
	wg.Wait()

	// clamp starts and ends to account for setup and takedown variance.
	starts := make([]time.Time, 0, nCli)
	ends := make([]time.Time, 0, nCli)
	for i := 0; i < nCli; i++ {
		ts := c.times[i]
		starts = append(starts, ts[0].start)
		ends = append(ends, ts[len(ts)-1].end)
	}
	init := slices.MaxFunc(starts, func(s, e time.Time) int {
		return s.Compare(e)
	})
	postWarm := init.Add(100 * time.Millisecond)
	end := slices.MinFunc(ends, func(s, e time.Time) int {
		return s.Compare(e)
	})
	total := end.Sub(postWarm)

	// extract sample from between warmup and ending time.
	*c.sample = stats.Sample{Xs: c.sample.Xs[:0]}
	for i := 0; i < nCli; i++ {
		times := c.times[i]
		low, _ := slices.BinarySearchFunc(times, postWarm,
			func(s startEnd, t time.Time) int {
				return s.start.Compare(t)
			})
		high, ok := slices.BinarySearchFunc(times, end,
			func(s startEnd, t time.Time) int {
				return s.end.Compare(t)
			})
		if ok {
			high++
		}
		if high-low < 100 {
			log.Fatal("clientRunner: clients don't have enough overlapping samples")
		}
		for j := low; j < high; j++ {
			d := float64(times[j].end.Sub(times[j].start).Nanoseconds())
			c.sample.Xs = append(c.sample.Xs, d)
		}
	}
	c.sample.Sort()
	return total
}

func mkRandVal() []byte {
	// ed25519 pk is 32 bytes.
	x := make([]byte, 32)
	randRead(x)
	return x
}

func getFreePort() (port uint64, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return uint64(l.Addr().(*net.TCPAddr).Port), nil
		}
	}
	return
}

func makeUniqueAddr() uint64 {
	port, err := getFreePort()
	if err != nil {
		panic("bad port")
	}
	// left shift to make IP 0.0.0.0.
	return port << 32
}

func getWarmup(nOps int) int {
	return int(float64(nOps) * float64(0.1))
}

func lePutUint64(b []byte, v uint64) {
	_ = b[7] // early bounds check to guarantee safety of writes below
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

func randRead(p []byte) {
	for len(p) >= 8 {
		lePutUint64(p, rand.Uint64())
		p = p[8:]
	}
	if len(p) > 0 {
		b := make([]byte, 8)
		lePutUint64(b, rand.Uint64())
		copy(p, b)
	}
}
