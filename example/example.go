// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package main

import "fmt"
import _ "math"
import "time"
import "flag"
import "math/rand"
import "leb/cuckoo"
import "leb/cuckoo/primes"
import "github.com/tildeleb/hrff"

var r = rand.Float64

func rbetween(a int, b int) int {
	rf := r()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

func tdiff(begin, end time.Time) time.Duration {
    d := end.Sub(begin)
    return d
}

var auto = flag.Bool("a", false, "automatic")
var verbose = flag.Bool("v", false, "verbose")
var ranf = flag.Bool("rr", true, "random run")
var ranb = flag.Bool("rb", false, "random base")
var ntables = flag.Int("t", 8, "tables")
var nbuckets = flag.Int("b", 10, "buckets")
var nslots = flag.Int("s", 8, "slots")
var ntrials = flag.Int("nt", 100, "number of trials")
var ibase = flag.Int("base", 1, "base of fill series, -1 for random")
var maxLevel = flag.Int("l", 2, "max level")
var lf = flag.Float64("lf", 0.96, "maximum load factor")
var flf = flag.Float64("flf", 1.0, "fill load factor")

var mr int

func Verify(c *cuckoo.Cuckoo, base, n int) bool {
	//fmt.Printf("Verify: base=%d, n=%d \n", base, n)
	cnt := 0
	for i := base; i < base + n; i++ {
		cnt++
		v, ok := c.Lookup(cuckoo.Key(i))
		if !ok {
			fmt.Printf("Verify: lookup FAILED i=%d, cnt=%d\n", i, cnt)
			return false
		}
		//fmt.Printf("Verify: check i=%d, cnt=%d == v=%d\n", i, cnt, uint32(v))
		if uint32(cnt) != uint32(v) {
			fmt.Printf("Verify: FAIL i=%d, cnt=%d != v=%d\n", i, cnt, uint32(v))
			return false
		}
	}
	//fmt.Printf("Verify: OK\n")
	return true
}

type FillStats struct {
	Load		float64
	Base		int
	Total 		int
	Thresh		int
	Used		int
	Remaining	int
	Failed		bool
}

func _fill(c *cuckoo.Cuckoo, tables, buckets, slots, ibase int, verbose, r bool) *FillStats {
	var fs FillStats

	base := 0
	if r {
		base = rbetween(1, 1<<29)
	} else {
		base = ibase
	}
	fs.Base = base
	fs.Total = tables * buckets * slots
	amt := float64(tables * buckets * slots)
	amt *= *flf
	max := int(amt)
	fs.Thresh = max
	amax := base + max
	//fmt.Printf("_fill: base=%d, max=%d\n", base, max)
	fs.Load = float64(1.0)
	cnt := 1
	if verbose {
		fmt.Printf("_fill: base=%d, n=%d\n", base, max)
	}
	svi := amax
	level := 0
	for i := base; i < amax; i++ {
		//fmt.Printf("%d\n", i)
		ok, l := c.Insert(cuckoo.Key(i), cuckoo.Value(uint32(cnt)))
		level = l
		if !ok {
			if true {
				fmt.Printf("\nFill: failed @ %d/%d, remain=%d, bumps=%d, %d/%d=%0.4f, level=%d, bpi=%0.2f\n", i, max, max - i, c.Bumps, c.Inserts, c.Elements, fs.Load, level, float64(c.Bumps)/float64(c.Inserts))
			}
			fs.Used = i - 1
			fs.Failed = true
			svi = i
			break
		} else {
			//fmt.Printf("%d ", level)
		}
		cnt++
	}
	fs.Load = float64(c.Elements)/float64(c.Size)
	if verbose {
		fs.Remaining = amax - svi
		fmt.Printf("\nFill: fail=%v @ %d/%d, remain=%d, bumps=%d, %d/%d=%0.4f, bpi=%0.2f\n",
			fs.Failed, svi, amax, amax - svi, c.Bumps, c.Inserts, c.Elements, fs.Load, float64(c.Bumps)/float64(c.Inserts))
		if fs.Remaining > mr {
			mr = fs.Remaining
		}
	}
	//fmt.Printf("\n")
	if level == -8000 {
		panic("_fill")
	}

	//avg := tot / float64(trials)
	//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f\n", tables, buckets, slots, trials, avg)
	return &fs
}

func fill(c *cuckoo.Cuckoo, tables, buckets, slots, ibase int, verbose, r bool) *FillStats {
	fs := _fill(c, tables, buckets, slots, ibase, verbose, r)
	if verbose {
		for k, v := range c.TableStats {
			fmt.Printf("table[%d]: %d/%d=%0.4f\n", k, v.Elements, v.Size, float64(v.Elements)/float64(v.Size))
		}
	}
	return fs
}

func delete(c *cuckoo.Cuckoo, base, n int, verbose bool) bool {
	//fmt.Printf("verify from=%d, n=%d\n", base, n)
	for i := base; i < base + n; i++ {
		if b, _ := c.Delete(cuckoo.Key(i)); !b {
			return false
		}
	}
	return true
}

func statAdd(tot, add *cuckoo.CuckooStat) {
	tot.Elements += add.Elements
	tot.Inserts += add.Inserts
	tot.Attempts += add.Attempts
	tot.Deletes += add.Deletes
	tot.Lookups += add.Lookups
	tot.Bumps += add.Bumps
	tot.Aborts += add.Aborts
}

func trials(tables, buckets, slots, trials int, lf float64, ibase int, verbose, r bool) (cs *cuckoo.CuckooStat, avg float64, rmax int, fails int) {
	var acs cuckoo.CuckooStat

	cs = &acs
	durations := make([]time.Duration, 5)
	labels := []string{"init", "fill", "verify", "delete", "verify"}
	tot := float64(0)
	fails = 0
	for t := 0; t < trials; t++ {
		start := time.Now()
		c := cuckoo.New(tables, buckets, slots, lf)
		fmt.Printf("trials size=%h\n", c.Size * c.BucketSize)
		stop := time.Now()
		durations[0] = tdiff(start, stop)

		start = time.Now()
		fs := fill(c, tables, buckets, slots, ibase, verbose, r)
		fmt.Printf("cf=%#v\n", c.Config)
		fmt.Printf("cs=%#v\n", c.CuckooStat)
		fmt.Printf("fs=%#v\n", fs)
		stop = time.Now()
		fmt.Printf("c.CuckooStat=%#v\n", c.CuckooStat)
		rmax = fs.Thresh
		durations[1] = tdiff(start, stop)

		tot += fs.Load
		if fs.Failed {
			fails++
		}

		start = time.Now()
		Verify(c, fs.Base, c.Elements)
		stop = time.Now()
		durations[2] = tdiff(start, stop)

		start = time.Now()
		ok := delete(c, fs.Base, c.Elements, verbose)
		if !ok {
			panic("delete failed")
		}
		stop = time.Now()
		durations[3] = tdiff(start, stop)

		statAdd(cs, &c.CuckooStat)

		// print information about operational rates
		if verbose {
			for k, v := range labels {
				f2 := hrff.Float64{float64(fs.Used) * (float64(time.Second) / float64(durations[k])), "ops/sec"}
				fmt.Printf("%s: %v %h\n", v, durations[k], f2)
			}
			fmt.Printf("\n")
		}
	}
	avg = tot / float64(trials)
	//fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, trials=%d, fails=%d, avg=%0.2f\n", tables, buckets, slots, trials, fails, avg)
	return // avg, max, fails
}


func main () {
	flag.Parse()
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

    st := 0
    ss := 0
    fails := 0

    //verbose := false
    if *ntrials == 1 {
    	*verbose = true
    }
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
    	tot := *ntables * *nbuckets * *nslots
		cs, avg, max, fails := trials(*ntables, *nbuckets, *nslots, *ntrials, *lf, *ibase, *verbose, *ranb)
		fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, size=%d, max=%d, trials=%d, fails=%d, avg=%0.4f\n", *ntables, *nbuckets, *nslots, tot, max, *ntrials, fails, avg)
		fmt.Printf("cs=%#v\n", cs)
		fmt.Printf("mr=%d\n", mr)
	}
}