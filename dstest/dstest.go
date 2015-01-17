// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// small step towards creating a package that can test data structures
package dstest

import "fmt"
import "math/rand"
import . "github.com/tildeleb/cuckoo"

var Mr int
var Ll int

// basic data structures methods and a method to get stats, what do we do about level?
type DSTest interface {
	Insert(key Key, value Value) (ok bool)
	InsertL(key Key, value Value) (ok bool, rlevel int)
	Lookup(key Key) (v Value, ok bool)
	Delete(key Key) (Value, bool)
	GetCounter(stat string) int
	GetTableCounter(t int, stat string) int
}

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

// return information about what happened during a fill
type FillStats struct {
	Load		float64
	Base		int
	Total 		int
	Thresh		int
	Used		int
	Remaining	int
	LowestLevel	int
	Fails		int
	Failed		bool
	Limited		bool
}

func _fill(d DSTest, tables, buckets, slots, ibase int, flf float64, verbose, printLevels, progress, r bool) *FillStats {
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
	amt *= flf
	max := int(amt)
	fs.Used = max
	fs.Thresh = max
	amax := base + max
	//fmt.Printf("_fill: base=%d, max=%d\n", base, max)
	fs.Load = float64(1.0)
	cnt := 1
	svi := amax
	lowestLevel := 1<<31
	onep := (amax - base) / 100
	thresh := 0

	if verbose {
		fmt.Printf("    fill: base=%d, n=%d\n", base, max)
	}
	if progress {
		fmt.Printf("F: ")
	}

	for i := base; i < amax; i++ {
		//fmt.Printf("%d\n", i)
		ok, l := d.InsertL(Key(i), Value(uint64(cnt)))
		if  l < lowestLevel && l != 0 {
			lowestLevel = l
		}
		if !ok {
			if l == 0 {
				fs.Limited = true
			}
			if verbose {
				if printLevels {
					fmt.Printf("%d/%d\n", l, lowestLevel)
				}
				fmt.Printf("    fill: failed @ %d/%d, remain=%d, MaxPathLen=%d, bumps=%d, %d/%d=%0.4f, level=%d, bpi=%0.2f\n",
					i, amax, amax - i, d.GetCounter("MaxPathLen"), d.GetCounter("bumps"), d.GetCounter("elements"), d.GetCounter("size"), fs.Load, l, float64(d.GetCounter("bumps"))/float64(d.GetCounter("inserts")))
			}
			fs.Used = i - base
			fs.Failed = true
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
			pcnt := cnt/onep
			if pcnt%10 == 0 {
				fmt.Printf("%d", pcnt/10)
			} else {
				fmt.Printf("%%")
			}
			fmt.Printf("%d: MaxPathLen=%d\n", cnt/onep, d.GetCounter("MaxPathLen"))
			thresh += onep
		}
		cnt++
	}
	if progress {
		fmt.Printf("\n")
	}
	fs.LowestLevel = lowestLevel
	fs.Load = float64(d.GetCounter("elements"))/float64(d.GetCounter("size"))
	fs.Remaining = amax - svi
	if verbose {
		fmt.Printf("    fill: fail=%v @ %d/%d, remain=%d, MaxPathLen=%d, bumps=%d, %d/%d=%0.4f, bpi=%0.2f\n",
			fs.Failed, svi, amax, amax - svi, d.GetCounter("MaxPathLen"), d.GetCounter("bumps"), d.GetCounter("inserts"), d.GetCounter("elements"), fs.Load, float64(d.GetCounter("bumps"))/float64(d.GetCounter("inserts")))
	}
	if fs.Remaining > Mr {
		Mr = fs.Remaining
	}
	if fs.LowestLevel < Ll {
		Ll = fs.LowestLevel
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

func Fill(d DSTest, tables, buckets, slots, ibase int, flf float64, verbose, pl, progress bool, r bool) *FillStats {
	fs := _fill(d, tables, buckets, slots, ibase, flf, verbose, pl, progress, r)
	if verbose {
		for i := 0; i < tables; i++ {
			fmt.Printf("    fill: table[%d]: %d/%d=%0.4f\n", i, d.GetTableCounter(i, "elements"), d.GetTableCounter(i, "size"), float64(d.GetTableCounter(i, "elements"))/float64(d.GetTableCounter(i, "size")))
		}
	}
	return fs
}

// test lookup by looking for a sequence of keys and making sure the values match the keys
func Verify(d DSTest, base, n int, progress bool) bool {
	//fmt.Printf("Verify: base=%d, n=%d \n", base, n)
	if progress {
		fmt.Printf("V: ")
	}
	onep := n / 100
	thresh := onep
	cnt := 0
	for i := base; i < base + n; i++ {
		cnt++
		v, ok := d.Lookup(Key(i))
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

func Delete(d DSTest, base, n int, verbose, progress bool) bool {
	//fmt.Printf("delete from=%d, n=%d\n", base, n)

	if progress {
		fmt.Printf("D: ")
	}
	onep := n / 100
	thresh := onep
	cnt := 0
	for i := base; i < base + n; i++ {
		if _, b := d.Delete(Key(i)); !b {
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