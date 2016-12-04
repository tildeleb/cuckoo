// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// This program provides a test interface to the cuckoo hash tables.
// The only test it currently knows how to do is crete the table,
// fill it with values, verify the values are in the table, and
// then verify they are not in the table
package main

import (
	cr "crypto/rand"
	"flag"
	"fmt"
	"leb.io/cuckoo"
	"leb.io/cuckoo/dstest"
	"leb.io/cuckoo/primes"
	"leb.io/cuckoo/siginfo"
	"leb.io/hrff"
	"log"
	_ "math"
	_ "math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
	"unsafe"
)

func tdiff(begin, end time.Time) time.Duration {
	d := end.Sub(begin)
	return d
}

var ranb = flag.Bool("rb", false, "ignore base, use random base value")
var ranr = flag.Bool("rr", false, "random run, seed for base values is random")
var rane = flag.Bool("re", false, "use a random seed for evictions source")
var seedb = flag.Int("sb", 0, "seed for base values")
var seede = flag.Int("se", 0, "seed for eviction values")

var auto = flag.Bool("a", false, "automatic")
var fo = flag.Bool("fo", false, "fill only")
var dg = flag.Bool("dg", false, "dont't add hash tables automatically")
var hash = flag.String("h", "aes", "name of hash function {aes, j264, j364}")
var ntables = flag.Int("t", 4, "tables")
var nbuckets = flag.Int("b", 31, "buckets")
var nslots = flag.Int("s", 8, "slots")
var ntrials = flag.Int("nt", 5, "number of trials")
var ibase = flag.Int("base", 1, "base of fill series, -1 for random")
var startLevel = flag.Int("sl", 2000, "starting level")
var lowLevel = flag.Int("ll", -8000, "lowest level")
var lf = flag.Float64("lf", 0.96, "maximum load factor")
var flf = flag.Float64("flf", 1.0, "fill load factor")

var pl = flag.Bool("pl", false, "print level of each insert")
var pt = flag.Bool("pt", false, "print summary for each trail")
var ps = flag.Bool("ps", false, "print stats at the end of all trails")
var pr = flag.Bool("pr", false, "print progress")
var pf = flag.Bool("pf", false, "print info on failure")
var verbose = flag.Bool("v", false, "verbose")

var cp = flag.String("cp", "", "write cpu profile to file")
var mp = flag.String("mp", "", "write memory profile to this file")

var bseed = int64(0) // seed for base values
var cseed = int64(0) // seed for evictions

func statAdd(tot, add *cuckoo.Counters) {
	tot.Elements += add.Elements
	tot.Inserts += add.Inserts
	tot.Attempts += add.Attempts
	tot.Deletes += add.Deletes
	tot.Lookups += add.Lookups
	tot.Bumps += add.Bumps
	tot.Aborts += add.Aborts
	tot.Iterations += add.Iterations
	tot.Fails += add.Fails
	tot.BucketSize = add.BucketSize
}

func hu(v uint64, u string) hrff.Int64 {
	return hrff.Int64{V: int64(v), U: u}
}

func hi(v int64, u string) hrff.Int64 {
	return hrff.Int64{V: int64(v), U: u}
}

/*
func rbetween(r int, a int, b int) int {
	rf := r.Float64()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	ret := int(r3)
	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f, ret=%d\n", a, b, rf, diff, r2, r3, ret)
	return ret
}
*/

func dump_mstats(m *runtime.MemStats, mstats, cstats, gc bool) {
	if mstats {
		fmt.Printf("Alloc=%h, TotalAlloc=%h, Sys=%h, Lookups=%h, Mallocs=%h, Frees=%h\n",
			hu(m.Alloc, "B"), hu(m.TotalAlloc, "B"), hu(m.Sys, "B"), hu(m.Lookups, ""), hu(m.Mallocs, ""), hu(m.Frees, ""))
		fmt.Printf("HeapAlloc=%h, HeapSys=%h, HeapIdle=%h, HeapInuse=%h, HeapReleased=%h, HeapObjects=%h\n",
			hu(m.HeapAlloc, "B"), hu(m.HeapSys, "B"), hu(m.HeapIdle, "B"), hu(m.HeapInuse, "B"), hu(m.HeapReleased, "B"), hu(m.HeapObjects, ""))
		fmt.Printf("StackInuse=%d, StackSys=%d, MSpanInuse=%d, MSpanSys=%d, MCacheSys=%d, BuckHashSys=%d\n", m.StackInuse, m.StackSys, m.MSpanInuse, m.MSpanSys, m.MCacheSys, m.BuckHashSys)
		fmt.Printf("NextGC=%d, LastGC=%d, PauseTotalNs=%d, NumGC=%d, EnableGC=%v, DebugGC=%v\n", m.NextGC, m.LastGC, m.PauseTotalNs, m.NumGC, m.EnableGC, m.DebugGC)
	}
	if cstats {
		for i, b := range m.BySize {
			if b.Mallocs == 0 {
				continue
			}
			fmt.Printf("BySize[%d]: Size=%d, Malloc=%d, Frees=%d\n", i, b.Size, b.Mallocs, b.Frees)
		}
	}
	if gc {
		for i := range m.PauseNs {
			fmt.Printf("PauseNs: ")
			fmt.Printf("%d, ", m.PauseNs[(int(m.NumGC)+255+i)%256])
			fmt.Printf("\n")
		}
	}
}

//func trials(tables, buckets, slots, trials int, lf float64, ibase int, verbose, r bool) (cs *cuckoo.Counters, avg float64, rmax int, fails int) {
func trials(tables, buckets, slots, trials int, eseed int64, lf float64, ibase int, verbose, ranr, ranb bool) (d *dstest.DSTest, cs *cuckoo.Counters, avg float64, rmax int, fails int) {
	var key cuckoo.Key
	var acs cuckoo.Counters
	var labels = []string{"init", "fill", "verify", "delete", "verify"}
	var durations = make([]time.Duration, 5)
	var msb, msa runtime.MemStats

	var print = func(i, used int) {
		if verbose {
			tmp := labels[i]
			f2 := hrff.Float64{float64(used) * (float64(time.Second) / float64(durations[i])), "ops/sec"}
			fmt.Printf("    %s: %v %h\n", tmp, durations[i], f2)
		}
	}

	cs = &acs
	tot := float64(0)
	fails = 0

	if ranr {
		bseed = time.Now().UTC().UnixNano()
		// fixed pattern or different values each time
		b := make([]byte, 8)
		_, err := cr.Read(b)
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		bseed = int64(uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
			uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])<<0)
	} else {
		bseed = int64(*seedb)
	}
	//r := rand.New(rand.NewSource(int64(bseed)))
	//rand.Seed(seed)

	td := dstest.NewTester(nil, *startLevel, 0) // ???
	for t := 0; t < trials; t++ {
		//fmt.Printf("t=%d, fails=%d\n", t, fails)
		// init
		//fmt.Printf("trials: init\n")
		start := time.Now()
		c := cuckoo.New(tables, buckets, slots, int64(*seede), lf, *hash)
		if c == nil {
			panic("New failed")
		}
		d = dstest.NewTester(c, *startLevel, bseed) // ???
		//d.I = c

		siz := int(unsafe.Sizeof(key))
		switch siz {
		case 4, 8:
			//fmt.Printf("Set SetNumericKeySize(%d)\n", siz)
			c.SetNumericKeySize(siz)
		}
		c.SetGrow(!*dg)
		c.StartLevel = *startLevel
		c.LowestLevel = *lowLevel
		stop := time.Now()
		if t == 0 {
			sz := hrff.Int64{int64(c.Size * c.BucketSize), "bytes"}
			fmt.Printf("trials: bseed=%#x, seede=%#x, cucko hash table size=%H\n", uint64(bseed), *seede, sz)
		}
		durations[0] = tdiff(start, stop)
		print(0, tables*buckets*slots)

		// fill
		//fmt.Printf("trials: fill\n")

		runtime.ReadMemStats(&msb)
		//dump_mstats(&msa, true, false, false)
		//fmt.Printf("\n")
		start = time.Now()
		fs := d.Fill(tables, buckets, slots, ibase, *flf, verbose, *pl, *pr, ranb)
		stop = time.Now()
		runtime.ReadMemStats(&msa)
		//dump_mstats(&msa, true, false, false)
		bpi := float64(c.Bumps) / float64(c.Inserts)
		api := float64(c.Attempts) / float64(c.Inserts)
		ipi := float64(c.Iterations) / float64(c.Inserts)

		rmax = fs.Thresh
		durations[1] = tdiff(start, stop)
		print(1, fs.Used)
		//c.Print() // xxx

		tot += fs.Load
		//fmt.Printf("fs=%#v\n", fs)
		if fs.Failed {
			fails++
			//fmt.Printf("fails=%d\n", fails)
		}
		if d.Limited {
			td.Limited = true
		}
		if *fo {
			continue
		}

		// verify
		//fmt.Printf("trials: verify base=%d, n=%d\n", fs.Base, c.Elements)
		start = time.Now()
		d.Verify(fs.Base, c.Elements, *pr)
		stop = time.Now()
		durations[2] = tdiff(start, stop)
		print(2, fs.Used)
		savElements := c.Elements

		// delete
		//fmt.Printf("trials: delete\n")
		start = time.Now()
		ok := d.Delete(fs.Base, c.Elements, verbose, *pr)
		if !ok || c.Elements != 0 {
			s := fmt.Sprintf("Delete failed ok=%v, c.Elements=%d\n", ok, c.Elements)
			fmt.Printf(s)
			//panic(s)
		}
		stop = time.Now()
		durations[3] = tdiff(start, stop)
		print(3, fs.Used)

		c.Elements = savElements
		statAdd(cs, &c.Counters)
		if d.Ll < td.Ll {
			td.Ll = d.Ll
			//fmt.Printf("setting td.Ll=%d\n", td.Ll)
		}
		//fmt.Printf("change d.Mr=%d, td.Mr=%d\n", d.Mr, td.Mr)
		if d.Mr > td.Mr {
			td.Mr = d.Mr
			//fmt.Printf("setting td.Mr=%d\n", td.Mr)
		}

		// print information about operational rates
		if false {
			for k, v := range labels {
				f2 := hrff.Float64{float64(fs.Used) * (float64(time.Second) / float64(durations[k])), "ops/sec"}
				fmt.Printf("    %s: %v %h\n", v, durations[k], f2)
			}
			fmt.Printf("\n")
		}
		if verbose {
			fmt.Printf("trials: cf=%#v\n", c.Config)
			fmt.Printf("trials: cs=%#v\n", c.Counters)
			fmt.Printf("trials: fs=%#v\n", fs)
		}
		if *pt || (*pf && fs.Failed) {
			fmt.Printf("trials: trial=%d, fails=%d, L=%v, F=%v, Remaining=%d, Aborts=%d, LowestLevel=%d, MaxAttemps=%d, MaxIterations=%d, bpi=%0.2f, api=%0.2f, ipi=%0.4f, lf=%0.2f (%d/%d)\n",
				t, fails, fs.Limited, fs.Failed, fs.Remaining, c.Aborts, fs.LowestLevel, c.MaxAttempts, c.MaxIterations, bpi, api, ipi, float64(c.Elements)/float64(c.Size), c.Elements, c.Size)
			fmt.Printf("trials: fs=%#v\n", fs)
		}

		bseed++
		if verbose {
			fmt.Printf("\n")
		}
	}
	avg = tot / float64(trials)
	/*
	   <<<<<<< HEAD
	   	fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f max=%d, fails=%d\n", tables, buckets, slots, trials,
	   		avg, rmax, fails)
	   	fmt.Printf("trials: cs=%#v\n", cs)
	   =======
	   	d = td
	   	//fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, trials=%d, fails=%d, avg=%0.2f\n", tables, buckets, slots, trials, fails, avg)
	*/
	return // avg, max, fails
}

func f() {
	*pt = !*pt
}

func runTrials() {
	//tables := []int{2, 3, 4, 5, 6, 7, 8}
	//slots := []int{1, 2, 3, 4, 5, 6, 7, 8}

	//st := 0
	//ss := 0
	siginfo.SetHandler(f)
	fails := 0
	//verbose := false
	if *ntrials == 1 {
		*verbose = true
	}
	/*
	       if *auto {
	       	maxAvg := float64(0)
	   		for _, b := range primes.Primes {
	       		for t := 1; t <= *ntables; t++ {
	       			for s := 1; s <= *nslots; s++ {
	   					//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d\n", t, b, s, *ntrials)
	       				_, avg, _, f := trials(t, b, s, *ntrials, *lf, 0, *verbose, *ranb)
	       				fails += f
	    					//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d\n", t, b, s, *ntrials)
	       				if avg > maxAvg {
	       					maxAvg = avg
	       					st = t
	       					ss = s
	       				}
	       			}
	       		}
	   	    	fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, fails=%d, avg=%0.4f\n", st, b, ss, *ntrials, fails, maxAvg)
	       		maxAvg = 0.0
	       	}
	       } else {
	*/
	nb := *nbuckets
	if nb < 0 {
		nb = primes.NextPrime(-nb)
	}
	tot := *ntables * nb * *nslots
	d, c, avg, max, fails := trials(*ntables, nb, *nslots, *ntrials, 0, *lf, *ibase, *verbose, *ranr, *ranb)

	//fmt.Printf("dbg: avg=%0.4f, max=%d, fails=%d\n", avg, max, fails)
	//fmt.Printf("dbg: d=%#v\n", d)
	//fmt.Printf("dbg: c=%#v\n", c)

	bpi := float64(c.Bumps) / float64(c.Inserts)
	api := float64(c.Attempts) / float64(c.Inserts)
	ipi := float64(c.Iterations) / float64(c.Inserts)

	fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, size=%d, max=%d, trials=%d, fails=%d, limited=%v, avg=%0.4f\n",
		*ntables, nb, *nslots, tot, max, *ntrials, fails, d.Limited, avg)
	fmt.Printf("trials: MaxRemaining=%d, LowestLevel=%d, Aborts=%d, bpi=%0.2f, api=%0.2f, ipi=%0.4f\n", d.Mr, d.Ll, c.Aborts, bpi, api, ipi)
	//fmt.Printf("trials: MaxRemaining=%d\n", dstest.Mr)
	//fmt.Printf("trials: LowestLevel=%d\n", dstest.Ll)
	if *ps {
		fmt.Printf("trials: c=%#v\n", c)
	}
	//	}
}

func main() {
	flag.Parse()
	if *mp != "" {
		f, err := os.Create(*mp)
		if err != nil {
			log.Fatal(err)
		}
		runTrials()
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}

	if *cp != "" {
		f, err := os.Create(*cp)
		if err != nil {
			log.Fatal(err)
		}
		runTrials()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		return
	}
	runTrials()
}
