// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package cuckoo_test

import . "leb/cuckoo"
//import "flag"
//import "fmt"
//import "math"
import "math/rand"
import "runtime"
import "testing"

var r = rand.Float64
var n = int(2e6)
const hashName = "m332"

type KeySet struct {
	Keys   		[]Key
	Vals   		[]Value
	M    		map[Key]Value
	AllocBytes	uint64
}
var ks *KeySet

func CreateKeysValuesMap(n int) *KeySet {
	var v Value
    var msb, msa runtime.MemStats
    var ks KeySet

	ks.Keys = make([]Key, n, n)
	ks.Vals = make([]Value, n, n)

	runtime.ReadMemStats(&msb)
	ks.M = make(map[Key]Value)
	for i := 0; i < n; i++ {
		k := Key(rand.Uint32())
		ks.M[k] = v
		ks.Keys[i] = k
		ks.Vals[i] = v
	}
	runtime.ReadMemStats(&msa)
	ks.AllocBytes = msa.Alloc - msb.Alloc
	return &ks
}


func init() {
	ks = CreateKeysValuesMap(n)
}


func TestMemoryEfficiency(t *testing.T) {
	const ef = 1.0
	const lf = 0.99
	const tables = 4
	const slots = 4
    var msb, msa runtime.MemStats

    runtime.ReadMemStats(&msb)
	c := New(tables, -int(float64(n)*ef+32.0)/(tables * slots), slots, lf, hashName)
    for k, v := range ks.M {
            c.Insert(k, v)
    }
    runtime.ReadMemStats(&msa)

    t.Logf("Cuckoo Hash LoadFactor:       %0.2f", c.LoadFactor())
    t.Logf("Cuckoo Hash memory allocated: %0.0f MiB", float64(msa.Alloc - msb.Alloc)/float64(1<<20))
    t.Logf("Go map memory allocated:      %0.0f MiB", float64(ks.AllocBytes)/float64(1<<20))
}


func benchmarkCuckooInsert(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	//fmt.Printf("BenchmarkCuckooInsert: N=%d, ef=%f, add=%f, lf=%f, tables=%d, slots=%d\n", b.N, ef, add, lf, tables, slots)
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
}


func benchmarkCuckooSearch(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	//c := New(tables, -int(float64(b.N)*ef+32.0)/(tables * slots), slots, lf)
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	for i := 0; i < len(ks.Keys); i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Lookup(ks.Keys[i%n])
	}
}

func benchmarkCuckooDelete(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	//c := New(tables, -int(float64(b.N)*ef+32.0)/(tables * slots), slots, lf)
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	for i := 0; i < len(ks.Keys); i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Delete(ks.Keys[i%n])
	}
}

func BenchmarkCuckooInsert2Tables2Slots(b *testing.B) {
	benchmarkCuckooInsert(1.2, 32.0, 0.80, 2, 2, "m332", b)
}


func BenchmarkCuckooSearch2Tables2Slots(b *testing.B) {
	benchmarkCuckooSearch(1.2, 32.0, 0.80, 2, 2, "m332", b)
}


func BenchmarkCuckooInsert4Tables4Slots(b *testing.B) {
	benchmarkCuckooInsert(1.2, 32.0, 0.99, 4, 4, "m332", b)
}


func BenchmarkCuckooSearch4Tables4Slots(b *testing.B) {
	benchmarkCuckooSearch(1.2, 32.0, 0.99, 4, 4, "m332", b)
}


func BenchmarkGoMapInsert(b *testing.B) {
	m := make(map[Key]Value)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		m[ks.Keys[i%n]] = ks.Vals[i%n]
	}
}


func BenchmarkGoMapSearch(b *testing.B) {
	m := make(map[Key]Value)

	for i := 0; i < len(ks.Keys); i++ {
		m[ks.Keys[i%n]] = ks.Vals[i%n]
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = m[ks.Keys[i%n]]
	}
}

/*
func rbetween(a int, b int) int {
	rf := r()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

func fill(c *Cuckoo, tables, buckets, slots, trials int, verbose, r bool) (float64, int) {
	tot := float64(0)
	fails := 0
	for t := 0; t < trials; t++ {
		base := 0
		if r {
			base = rbetween(1, 1<<29)
		}
		cnt := 1
		maxi := base + tables * buckets * slots
		percent := float64(1.0)
		for i := base+1; i <= base + tables * buckets * slots; i++ {
			if !c.Insert(Key(i), Value(uint32(cnt))) {
				percent = float64(c.Inserts)/float64(c.Elements)
				if verbose {
					fmt.Printf("failed: %d/%d, bumps=%d, %d/%d=%0.4f\n", i, maxi, c.Bumps, c.Inserts, c.Elements, percent)
				}
				maxi = i - 1
				fails++
				break
			}
			cnt++
		}
		tot += percent
		if verbose {
			for k, v := range c.Stats {
				fmt.Printf("table[%d]: %d/%d=%0.4f\n", k, v.Elements, v.Size, float64(v.Elements)/float64(v.Size))
			}
		}
		c.Verify(base+1, maxi - base)
	}
	avg := tot / float64(trials)
	//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f\n", tables, buckets, slots, trials, avg)
	return avg, fails
}



func BenchmarkBasic(b *testing.B) {
    b.StopTimer()
	c := New(4, b.N, 4)
    b.StartTimer()
   	fill(c, 4, b.N, 4, 1, true, false)
    b.ReportAllocs()
}
*/

/*
func BenchmarkCuckooInsert(b *testing.B) {
	//fmt.Printf("BenchmarkCuckooInsert: N=%d\n", b.N)
	c := New(tables, -int(float64(b.N)*ef+32.0)/(tables * slots), slots, lf)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Insert(gkeys[i%n], gvals[i%n])
	}
}

func BenchmarkCuckooSearch(b *testing.B) {
	c := New(tables, -int(float64(b.N)*ef+32.0)/(tables * slots), slots, lf)
	for i := 0; i < len(gkeys); i++ {
		c.Insert(gkeys[i%n], gvals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Lookup(gkeys[i%n])
	}
}
*/
