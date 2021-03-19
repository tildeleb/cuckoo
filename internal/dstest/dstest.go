// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.

// small step towards creating a package that can test data structures
package dstest

import "fmt"
import "math/rand"
import c "leb.io/cuckoo"

//var Mr int
//var Ll int

// basic data structures methods and a method to get stats, what do we do about level?
type DSTester interface {
	Insert(key c.Key, value c.Value) (ok bool)
	InsertL(key c.Key, value c.Value) (ok bool, rlevel int)
	Lookup(key c.Key) (v c.Value, ok bool)
	Delete(key c.Key) (c.Value, bool)
	GetCounter(stat string) int
	GetTableCounter(t int, stat string) int
}

//var r = rand.Float64

// return information about what happened during a fill
type FillStats struct {
	Load        float64
	Base        int
	Total       int
	Thresh      int
	Used        int
	Remaining   int
	LowestLevel int
	Fails       int
	Failed      bool
	Limited     bool
}

type DSTest struct {
	Seed      int64      // seed used to control fill stream
	Mr        int        // Max remaining
	Ll        int        // Lowest level
	FillStats            // stats
	I         DSTester   // functions
	R         *rand.Rand // random number generator with no lock
}

func NewTester(i DSTester, ll int, seed int64) *DSTest {
	d := DSTest{Seed: seed, Ll: ll, I: i}
	d.I = i
	//d.Remaining = 1<<32
	d.R = rand.New(rand.NewSource(int64(seed))) // no lock
	return &d
}

func (d *DSTest) rbetween(a int, b int) int {
	//rf := r()
	rf := d.R.Float64()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	ret := int(r3)
	//fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f, ret=%d\n", a, b, rf, diff, r2, r3, ret)
	return ret
}

func (d *DSTest) _fill(tables, buckets, slots, ibase int, flf float64, verbose, printLevels, progress, r bool) *FillStats {
	var fs FillStats
	base := 0
	if r {
		base = d.rbetween(1, 1<<29)
		//fmt.Printf("random generated base=%d\n", base)
	} else {
		base = ibase
	}
	fs.Base = base
	fs.Total = tables * buckets * slots
	amt := float64(tables * buckets * slots)
	amt *= flf
	max := int(amt)
	fs.Used = max
	fs.Thresh = max
	amax := base + max
	//fmt.Printf("_fill: base=%d, amax=%d, max=%d\n", base, amax, max)
	fs.Load = float64(1.0)
	cnt := 1
	svi := amax
	lowestLevel := 1 << 31
	onep := (amax - base) / 100
	thresh := 0

	if verbose {
		fmt.Printf("    fill: base=%d, amax=%d, n=%d\n", base, amax, amax-base)
	}
	if progress {
		fmt.Printf("F: ")
	}

	for i := base; i < amax; i++ {
		//fmt.Printf("%d\n", i)
		ok, l := d.I.InsertL(c.Key(i), c.Value(uint64(cnt)))
		if l < lowestLevel && l != 0 {
			lowestLevel = l
		}
		if !ok {
			// Two reasons we fail, load constraint (fs.Limited) or other (fs.Failed)
			if l == 0 {
				fs.Limited = true
			} else {
				fs.Failed = true
			}
			if verbose {
				if printLevels {
					fmt.Printf("%d/%d\n", l, lowestLevel)
				}
				fmt.Printf("    fill: %d/%d, remain=%d, MaxPathLen=%d, bumps=%d, %d/%d=%0.4f, level=%d, bpi=%0.2f\n",
					i, amax, amax-i, d.I.GetCounter("MaxPathLen"), d.I.GetCounter("bumps"), d.I.GetCounter("elements"), d.I.GetCounter("size"),
					fs.Load, l, float64(d.I.GetCounter("bumps"))/float64(d.I.GetCounter("inserts")))
			}
			fs.Used = i - base
			fs.LowestLevel = lowestLevel
			svi = i
			break
		} else {
			//fmt.Printf("%d\n", i)
			if printLevels {
				fmt.Printf("%d/%d ", l, lowestLevel)
			}
		}
		if progress && cnt >= thresh {
			pcnt := cnt / onep
			if pcnt%10 == 0 {
				fmt.Printf("%d", pcnt/10)
			} else {
				fmt.Printf("%%")
			}
			fmt.Printf("%d: MaxPathLen=%d\n", cnt/onep, d.I.GetCounter("MaxPathLen"))
			thresh += onep
		}
		cnt++
	}
	if progress {
		fmt.Printf("\n")
	}
	fs.LowestLevel = lowestLevel
	fs.Load = float64(d.I.GetCounter("elements")) / float64(d.I.GetCounter("size"))
	fs.Remaining = amax - svi
	if verbose {
		fmt.Printf("    fill: fail=%v @ %d/%d, remain=%d, MaxPathLen=%d, bumps=%d, %d/%d=%0.4f, bpi=%0.2f\n",
			fs.Failed, svi, amax, amax-svi,
			d.I.GetCounter("MaxPathLen"), d.I.GetCounter("bumps"), d.I.GetCounter("inserts"), d.I.GetCounter("elements"),
			fs.Load, float64(d.I.GetCounter("bumps"))/float64(d.I.GetCounter("inserts")))
	}
	if fs.Remaining > d.Mr {
		d.Mr = fs.Remaining
	}
	if fs.LowestLevel < d.Ll {
		d.Ll = fs.LowestLevel
	}
	if printLevels && !verbose {
		fmt.Printf("\n")
	}
	//fmt.Printf("\n")
	/*
		if level == -8000 {
			panic("_fill")
		}
	*/
	//avg := tot / float64(trials)
	//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f\n", tables, buckets, slots, trials, avg)
	return &fs
}

func (d *DSTest) Fill(tables, buckets, slots, ibase int, flf float64, verbose, pl, progress bool, r bool) *FillStats {
	fs := d._fill(tables, buckets, slots, ibase, flf, verbose, pl, progress, r)
	if verbose {
		for i := 0; i < tables; i++ {
			fmt.Printf("    fill: table[%d]: %d/%d=%0.4f\n", i,
				d.I.GetTableCounter(i, "elements"), d.I.GetTableCounter(i, "size"), float64(d.I.GetTableCounter(i, "elements"))/float64(d.I.GetTableCounter(i, "size")))
		}
	}
	return fs
}

// test lookup by looking for a sequence of keys and making sure the values match the keys
func (d *DSTest) Verify(base, n int, progress bool) bool {
	//fmt.Printf("Verify: base=%d, n=%d \n", base, n)
	if progress {
		fmt.Printf("V: ")
	}
	onep := n / 100
	thresh := onep
	cnt := 0
	if false {
		fmt.Printf("    verify: base=%d, base+n=%d, n=%d\n", base, base+n, n)
	}
	//fmt.Printf("    verify: base=%d, base+n=%d, n=%d\n", base, base+n, n)
	for i := base; i < base+n; i++ {
		cnt++
		v, ok := d.I.Lookup(c.Key(i))
		if !ok {
			fmt.Printf("Verify: lookup FAILED i=%d, cnt=%d\n", i, cnt)
			return false
		}
		//fmt.Printf("Verify: check i=%d, cnt=%d == v=%d\n", i, cnt, uint32(v))
		if uint32(cnt) != uint32(v) {
			fmt.Printf("Verify: FAIL i=%d, cnt=%d != v=%d\n", i, cnt, uint64(v))
			return false
		}
		if progress && cnt > thresh {
			fmt.Printf("%%")
			thresh += onep
		}
	}
	if progress {
		fmt.Printf("\n")
	}
	//fmt.Printf("Verify: OK\n")
	return true
}

func (d *DSTest) Delete(base, n int, verbose, progress bool) bool {
	//fmt.Printf("delete from=%d, n=%d\n", base, n)

	if progress {
		fmt.Printf("D: ")
	}
	onep := n / 100
	thresh := onep
	cnt := 0
	for i := base; i < base+n; i++ {
		if _, b := d.I.Delete(c.Key(i)); !b {
			return false
		}
		cnt++
		if progress && cnt > thresh {
			fmt.Printf("%%")
			thresh += onep
		}
	}
	if progress {
		fmt.Printf("\n")
	}
	return true
}
