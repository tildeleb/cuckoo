// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// This program provides a test interface to the cuckoo hash tables.
// The only test it currently knows how to do is crete the table,
// fill it with values, verify the values are in the table, and
// then verify they are not in the table
package main

import (
	"flag"
	"fmt"
	"leb.io/cuckoo"
	"leb.io/cuckoo/dstest"
	"leb.io/cuckoo/primes"
	"leb.io/cuckoo/siginfo"
	"leb.io/hrff"
	"log"
	_ "math"
	"math/rand"
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

var auto = flag.Bool("a", false, "automatic")
var fo = flag.Bool("fo", false, "fill only")
var dg = flag.Bool("dg", false, "dont't add hash tables automatically")
var ranf = flag.Bool("rr", true, "random run")
var ranb = flag.Bool("rb", true, "random base")
var hash = flag.String("h", "aes", "name of hash function (aes or j264)")
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
var verbose = flag.Bool("v", false, "verbose")

var cp = flag.String("cp", "", "write cpu profile to file")
var mp = flag.String("mp", "", "write memory profile to this file")

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

func trials(tables, buckets, slots, trials int, lf float64, ibase int, verbose, r bool) (cs *cuckoo.Counters, avg float64, rmax int, fails int) {
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

	dstest.Ll = *startLevel
	cs = &acs

	tot := float64(0)
	fails = 0
	for t := 0; t < trials; t++ {
		fmt.Printf("t=%d, fails=%d\n", t, fails)
		// init
		//fmt.Printf("trials: init\n")
		start := time.Now()
		c := cuckoo.New(tables, buckets, slots, lf, *hash)
		if c == nil {
			panic("New failed")
		}
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
			fmt.Printf("trials: cucko hash table size=%H\n", sz)
		}
		durations[0] = tdiff(start, stop)
		print(0, tables*buckets*slots)

		// fill
		//fmt.Printf("trials: fill\n")

		runtime.ReadMemStats(&msb)
		//dump_mstats(&msa, true, false, false)
		//fmt.Printf("\n")
		start = time.Now()
		fs := dstest.Fill(c, tables, buckets, slots, ibase, *flf, verbose, *pl, *pr, r)
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
		fmt.Printf("fs=%#v\n", fs)
		if fs.Failed {
			fails++
			fmt.Printf("fails=%d\n", fails)
		}
		if *fo {
			continue
		}

		// verify
		//fmt.Printf("trials: verify base=%d, n=%d\n", fs.Base, c.Elements)
		start = time.Now()
		dstest.Verify(c, fs.Base, c.Elements, *pr)
		stop = time.Now()
		durations[2] = tdiff(start, stop)
		print(2, fs.Used)
		savElements := c.Elements

		// delete
		//fmt.Printf("trials: delete\n")
		start = time.Now()
		ok := dstest.Delete(c, fs.Base, c.Elements, verbose, *pr)
		if !ok || c.Elements != 0 {
			s := fmt.Sprintf("Delete failed ok=%v, c.Elements=%d", ok, c.Elements)
			fmt.Printf(s)
			//panic(s)
		}
		stop = time.Now()
		durations[3] = tdiff(start, stop)
		print(3, fs.Used)

		c.Elements = savElements
		statAdd(cs, &c.Counters)

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
		if *pt {
			fmt.Printf("trials: trial=%d, fails=%d, L=%v, F=%v, Remaining=%d, Aborts=%d, LowestLevel=%d, MaxAttemps=%d, MaxIterations=%d, bpi=%0.2f, api=%0.2f, ipi=%0.4f, lf=%0.2f (%d/%d)\n",
				t, fails, fs.Limited, fs.Failed, fs.Remaining, c.Aborts, fs.LowestLevel, c.MaxAttempts, c.MaxIterations, bpi, api, ipi, float64(c.Elements)/float64(c.Size), c.Elements, c.Size)
		}
		if verbose {
			fmt.Printf("\n")
		}
	}
	avg = tot / float64(trials)
	fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f max=%d, fails=%d\n", tables, buckets, slots, trials,
		avg, rmax, fails)
	fmt.Printf("trials: cs=%#v\n", cs)
	return // avg, max, fails
}

func f() {
	*pt = !*pt
}

func runTrials() {
	seed := int64(0)
	// fixed pattern or different values each time
	if *ranf {
		seed = time.Now().UTC().UnixNano()
	} else {
		seed = int64(0)
	}
	rand.Seed(seed)

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
	c, avg, max, fails := trials(*ntables, nb, *nslots, *ntrials, *lf, *ibase, *verbose, *ranb)
	bpi := float64(c.Bumps) / float64(c.Inserts)
	api := float64(c.Attempts) / float64(c.Inserts)
	ipi := float64(c.Iterations) / float64(c.Inserts)

	fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, size=%d, max=%d, trials=%d, fails=%d, avg=%0.4f\n", *ntables, nb, *nslots, tot, max, *ntrials, fails, avg)
	fmt.Printf("trials: MaxRemaining=%d, LowestLevel=%d, Aborts=%d, bpi=%0.2f, api=%0.2f, ipi=%0.4f\n", dstest.Mr, dstest.Ll, c.Aborts, bpi, api, ipi)
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
