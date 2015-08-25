// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package cuckoo_test

import . "github.com/tildeleb/cuckoo"
import . "github.com/tildeleb/cuckoo/dstest"
import "leb.io/hrff"

//import "flag"
import "fmt"

//import "math"
import "math/rand"
import "runtime"
import "testing"

var r = rand.Float64
var b = int(0)
var n = int(2e6)

const hashName = "aes" // aes" "j264"

type KeySet struct {
	Keys       []Key
	Vals       []Value
	M          map[Key]Value
	AllocBytes uint64
}

var ks *KeySet

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

func CreateKeysValuesMap(b, n int) *KeySet {
	var v Value
	var msb, msa runtime.MemStats
	var ks KeySet

	ks.Keys = make([]Key, n, n)
	ks.Vals = make([]Value, n, n)

	runtime.ReadMemStats(&msb)
	ks.M = make(map[Key]Value)
	for i := b; i < b+n; i++ {
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
	ks = CreateKeysValuesMap(b, n)
}

const ef = 1.01
const add = 32.0
const lf = 1.0
const flf = 1.0
const tables = 2
const slots = 8

func TestBasic(t *testing.T) {
	const ef = 1.01
	const add = 32.0
	const lf = 1.0
	const flf = 0.9
	const tables = 4
	const slots = 8
	const n = 1000000

	c := New(tables, -int(float64(n)*ef+add)/(tables*slots), slots, lf, hashName)
	//t.Logf("Config=%#v\n", c.Config)
	if c == nil {
		t.Logf("New failed probably because slots don't match")
		t.FailNow()
	}
	c.SetNumericKeySize(8)
	_ = Fill(c, tables, n/(tables*slots), slots, 1, flf, false, false, false, false)
	ok := Verify(c, 1, n/(tables*slots), false)
	if !ok {
		t.FailNow()
	}
	//t.Logf("Stats=%#v\n", c.CuckooStat)
}

func TestMemoryEfficiency(t *testing.T) {
	const ef = 1.01
	const add = 32.0
	const lf = 1.0
	const flf = 1.0
	const tables = 2
	const slots = 8
	const n = 1000000
	var msb, msa runtime.MemStats

	runtime.ReadMemStats(&msb)
	c := New(tables, -int(float64(n)*ef+add)/(tables*slots), slots, lf, hashName)
	if c == nil {
		t.Logf("New failed probably because slots don't match")
		t.FailNow()
	}
	c.SetNumericKeySize(8)
	//for k, v := range ks.M {
	//	c.Insert(k, v)
	//}
	fs := Fill(c, tables, n/(tables*slots), slots, 1, flf, false, false, false, true)
	runtime.ReadMemStats(&msa)

	//dump_mstats(&msb, true, false, false)
	//fmt.Printf("\n")
	//dump_mstats(&msa, true, false, false)
	//fmt.Printf("msb=%#v\n", msb)
	//fmt.Printf("msa=%#v\n", msa)

	t.Logf("Cuckoo Hash LoadFactor:       %0.2f", c.GetLoadFactor())
	t.Logf("Cuckoo Hash memory allocated: %0.0f MiB", float64(msa.Alloc-msb.Alloc)/float64(1<<20))
	t.Logf("Go map memory allocated:      %0.0f MiB", float64(ks.AllocBytes)/float64(1<<20))
	//t.Logf("stats=%#v\n", fs)
	fs.Fails = fs.Fails
	//fmt.Printf("Config=%#v\n", c.Config)
	//fmt.Printf("Counters=%#v\n\n", c.Counters)
}

func benchmarkCuckooInsert(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	//t.Logf("BenchmarkCuckooInsert: N=%d, ef=%f, add=%f, lf=%f, tables=%d, slots=%d\n", b.N, ef, add, lf, tables, slots)
	c := New(tables, -int(float64(b.N)*ef+add)/(tables*slots), slots, lf, hash)
	//fmt.Printf("Config=%#v\n", c.Config)
	c.SetNumericKeySize(8)
	//fmt.Printf("N=%d\n", b.N)
	b.ResetTimer()

	if true {
		for i := 0; i < b.N; i++ {
			c.Insert(ks.Keys[i%n], ks.Vals[i%n])
		}
	} else {
		fs := Fill(c, tables, b.N, slots, 1, flf, false, false, false, true)
		fs.Fails = fs.Fails
		//b.Logf("stats=%#v\n", fs)
	}
	//fmt.Printf("Config=%#v\n", c.Config)
	//fmt.Printf("Counters=%#v\n\n", c.Counters)
	b.ReportAllocs()
}

func benchmarkCuckooSearch(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	c := New(tables, -int(float64(b.N)*ef+add)/(tables*slots), slots, lf, hash)
	c.SetNumericKeySize(8)
	for i := 0; i < b.N; i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Lookup(ks.Keys[i%n])
	}
}

func benchmarkCuckooDelete(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	c := New(tables, -int(float64(b.N)*ef+add)/(tables*slots), slots, lf, hash)
	c.SetNumericKeySize(8)
	for i := 0; i < b.N; i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Delete(ks.Keys[i%n])
	}
}

func BenchmarkCuckoo2T2SInsert(b *testing.B) {
	benchmarkCuckooInsert(ef, add, lf, tables, slots, hashName, b)
}

func BenchmarkCuckoo2T2SSearch(b *testing.B) {
	benchmarkCuckooSearch(ef, add, lf, tables, slots, hashName, b)
}

func BenchmarkCuckoo2T2SDelete(b *testing.B) {
	benchmarkCuckooDelete(ef, add, lf, tables, slots, hashName, b)
}

/*
func BenchmarkCuckoo4T4SInsert(b *testing.B) {
	benchmarkCuckooInsert(1.0, 32.0, 0.99, 4, 4, "m332", b)
}

func BenchmarkCuckoo4T4SSearch(b *testing.B) {
	benchmarkCuckooSearch(1.0, 32.0, 0.99, 4, 4, "m332", b)
}

func BenchmarkCuckoo4T4SDelete(b *testing.B) {
	benchmarkCuckooDelete(1.0, 32.0, 0.99, 4, 4, "m332", b)
}
*/

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

func BenchmarkGoMapDelete(b *testing.B) {
	m := make(map[Key]Value)

	for i := 0; i < len(ks.Keys); i++ {
		m[ks.Keys[i%n]] = ks.Vals[i%n]
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		delete(m, ks.Keys[i%n])
	}
}

// Demonstrate how to create a cuckoo table and insert, lookup, and delete elemebts
func Example() {
	const tables = 4
	const buckets = 11
	const slots = 8
	//const hashName = "m332"
	var lf = 0.95 // has to be a var or we get an err
	var cnt int

	var countf = func(c *Cuckoo, key Key, val Value) (stop bool) {
		cnt++
		return
	}

	c := New(tables, buckets, slots, lf, hashName)
	if c == nil {
		fmt.Printf("Example: New failed probably because slots don't match")
	}
	c.SetNumericKeySize(8)

	n := int(float64(tables*buckets*slots) * lf)

	// insert
	for i := 0; i < n; i++ {
		k, v := Key(i), Value(i)
		ok := c.Insert(k, v)
		if !ok {
			fmt.Printf("Example: Insert failed")
			return
		}
	}

	// lookup
	for i := 0; i < n; i++ {
		k := Key(i)
		v, ok := c.Lookup(k)
		if !ok {
			fmt.Printf("Example: Lookup failed")
			return
		}
		if v != Value(i) {
			fmt.Printf("Example: Values don't match %v vs %v\n", v, Value(i))
		}
	}

	// iterate
	c.Map(countf)
	s := fmt.Sprintf("cnt=%d vs %d\n", cnt, c.Counters.Elements)
	if cnt != c.Counters.Elements {
		panic(s)
	}

	// delete
	for i := 0; i < n; i++ {
		k := Key(i)
		v, ok := c.Delete(k)
		if !ok {
			fmt.Printf("Example: Delete failed")
			return
		}
		if v != Value(i) {
			fmt.Printf("Example: Values don't match %v vs %v\n", v, Value(i))
		}
	}

	// iterate
	cnt = 0
	c.Map(countf)
	if cnt != 0 {
		panic("cnt 2")
	}

	fmt.Printf("Example: Passed\n")
	// Output:
	// Example: Passed
}
