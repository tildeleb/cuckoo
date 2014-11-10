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
var ibase = flag.Int("base", 0, "base of fill series")
var maxLevel = flag.Int("l", 2, "max level")
var lf = flag.Float64("lf", 0.96, "maximum load factor")
var flf = flag.Float64("flf", 1.0, "fill load factor")

var mr int

func _fill(c *cuckoo.Cuckoo, tables, buckets, slots, ibase int, verbose, r bool) (percent float64, base, max int, failed bool) {
	/// base = 0
	if ibase == 0 {
		if r {
			base = rbetween(1, 1<<29)
		}
	} else {
		base = ibase
	}
	amt := float64(tables * buckets * slots)
	amt *= *flf
	max = int(amt)
	amax := base + int(amt)
	//fmt.Printf("_fill: base=%d, max=%d\n", base, max)
	percent = float64(1.0)
	cnt := 1
	if verbose {
		fmt.Printf("_fill: base=%d, n=%d\n", base+1, amax - base)
	}
	svi := amax
	level := 0
	for i := base+1; i <= amax; i++ {
		ok, l := c.Insert(cuckoo.Key(i), cuckoo.Value(uint32(cnt)))
		level = l
		if !ok {
			percent = float64(c.Inserts)/float64(c.Elements)
			if true {
				//fmt.Printf("\nFill: failed @ %d/%d, remain=%d, bumps=%d, %d/%d=%0.4f, level=%d, bpi=%0.2f\n", i, max, max - i, c.Bumps, c.Inserts, c.Elements, percent, level, float64(c.Bumps)/float64(c.Inserts))
			}
			max = i - 1
			failed = true
			svi = i -1
			break
		} else {
			//fmt.Printf("%d ", level)
		}
		cnt++
	}
	if verbose {
		remain := amax - svi
		fmt.Printf("\nFill: fail=%v @ %d/%d, remain=%d, bumps=%d, %d/%d=%0.4f, bpi=%0.2f\n",
			failed, svi, amax, amax - svi, c.Bumps, c.Inserts, c.Elements, percent, float64(c.Bumps)/float64(c.Inserts))
		if remain > mr {
			mr = remain
		}
	}
	//fmt.Printf("\n")
	if level == -8000 {
		panic("_fill")
	}

	//avg := tot / float64(trials)
	//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f\n", tables, buckets, slots, trials, avg)
	return percent, base, max, failed
}

func fill(c *cuckoo.Cuckoo, tables, buckets, slots, ibase int, verbose, r bool) (percent float64, base, max int, failed bool) {
	percent, base, max, failed = _fill(c, tables, buckets, slots, ibase, verbose, r)
	if verbose {
		for k, v := range c.TableStats {
			fmt.Printf("table[%d]: %d/%d=%0.4f\n", k, v.Elements, v.Size, float64(v.Elements)/float64(v.Size))
		}
	}
	return
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

func trials(tables, buckets, slots, trials int, lf float64, ibase int, verbose, r bool) (avg float64, max int, fails int) {
	durations := make([]time.Duration, 5)
	labels := []string{"init", "fill", "verify", "delete", "verify"}
	tot := float64(0)
	fails = 0
	for t := 0; t < trials; t++ {
		start := time.Now()
		c := cuckoo.New(tables, buckets, slots, lf)
		stop := time.Now()
		durations[0] = tdiff(start, stop)

		start = time.Now()
		percent, base, amax, failed := fill(c, tables, buckets, slots, ibase, verbose, r)
		stop = time.Now()
		max = amax
		durations[1] = tdiff(start, stop)

		tot += percent
		if failed {
			fails++
		}

		start = time.Now()
		c.Verify(base+1, max - base)
		stop = time.Now()
		durations[2] = tdiff(start, stop)

		start = time.Now()
		ok := delete(c, base+1, max - base, verbose)
		if !ok {
			panic("delete failed")
		}
		stop = time.Now()
		durations[3] = tdiff(start, stop)

		// print information about operational rates
		if verbose {
			for k, v := range labels {
				f2 := hrff.Float64{float64(max) * (float64(time.Second) / float64(durations[k])), "ops/sec"}
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
    				avg, _, f := trials(t, b, s, *ntrials, *lf, 0, *verbose, *ranb)
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
		avg, max, fails := trials(*ntables, *nbuckets, *nslots, *ntrials, *lf, *ibase, *verbose, *ranb)
		fmt.Printf("trials: tables=%d, buckets=%d, slots=%d, size=%d, max=%d, trials=%d, fails=%d, avg=%0.4f\n", *ntables, *nbuckets, *nslots, tot, max, *ntrials, fails, avg)
		fmt.Printf("mr=%d\n", mr)
	}
}