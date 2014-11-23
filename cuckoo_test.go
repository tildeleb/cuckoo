// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package cuckoo_test

import . "github.com/tildeleb/cuckoo"
import . "github.com/tildeleb/cuckoo/dstest"
//import "flag"
//import "fmt"
//import "math"
import "math/rand"
import "runtime"
import "testing"

var r = rand.Float64
var b = int(0)
var n = int(2e6)
const hashName = "m332"

type KeySet struct {
	Keys   		[]Key
	Vals   		[]Value
	M    		map[Key]Value
	AllocBytes	uint64
}
var ks *KeySet

func CreateKeysValuesMap(b, n int) *KeySet {
	var v Value
    var msb, msa runtime.MemStats
    var ks KeySet

	ks.Keys = make([]Key, n, n)
	ks.Vals = make([]Value, n, n)

	runtime.ReadMemStats(&msb)
	ks.M = make(map[Key]Value)
	for i := b; i < b + n; i++ {
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

const ef = 1.0
const add = 32.0
const lf = 0.99
const tables = 2
const slots = 8

func TestBasic(t *testing.T) {
	const ef = 1.0
	const add = 32.0
	const lf = 0.9
	const flf = 0.9
	const tables = 4
	const slots = 8
	const n = 1000000

	c := New(tables, -int(float64(n)*ef+add)/(tables * slots), slots, lf, hashName)
	//t.Logf("Config=%#v\n", c.Config)
	if c == nil {
		t.Logf("New failed probably because slots don't match")
		t.FailNow()
	}
	c.SetNumericKeySize(4)
   	_ = Fill(c, tables, n/(tables*slots), slots, 1, flf, false, false, false)
 	ok := Verify(c, 1, n/(tables*slots))
 	if !ok {
 		t.FailNow()
 	}
   	//t.Logf("Stats=%#v\n", c.CuckooStat)
}

func TestMemoryEfficiency(t *testing.T) {
    var msb, msa runtime.MemStats

	runtime.ReadMemStats(&msb)
	c := New(tables, -int(float64(n)*ef+add)/(tables * slots), slots, lf, hashName)
	if c == nil {
		t.Logf("New failed probably because slots don't match")
		t.FailNow()
	}
	c.SetNumericKeySize(4)
	for k, v := range ks.M {
		c.Insert(k, v)
	}
	runtime.ReadMemStats(&msa)

	t.Logf("Cuckoo Hash LoadFactor:       %0.2f", c.LoadFactor())
	t.Logf("Cuckoo Hash memory allocated: %0.0f MiB", float64(msa.Alloc - msb.Alloc)/float64(1<<20))
	t.Logf("Go map memory allocated:      %0.0f MiB", float64(ks.AllocBytes)/float64(1<<20))
}


func benchmarkCuckooInsert(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	//t.Logf("BenchmarkCuckooInsert: N=%d, ef=%f, add=%f, lf=%f, tables=%d, slots=%d\n", b.N, ef, add, lf, tables, slots)
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	c.SetNumericKeySize(4)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
}


func benchmarkCuckooSearch(ef, add, lf float64, tables, slots int, hash string, b *testing.B) {
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	c.SetNumericKeySize(4)
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
	c := New(tables, -int(float64(b.N)*ef+add)/(tables * slots), slots, lf, hash)
	c.SetNumericKeySize(4)
	for i := 0; i < len(ks.Keys); i++ {
		c.Insert(ks.Keys[i%n], ks.Vals[i%n])
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Delete(ks.Keys[i%n])
	}
}


func BenchmarkCuckoo2T2SInsert(b *testing.B) {
	benchmarkCuckooInsert(ef, add, lf, tables, slots, "m332", b)
}

func BenchmarkCuckoo2T2SSearch(b *testing.B) {
	benchmarkCuckooSearch(ef, add, lf, tables, slots, "m332", b)
}

func BenchmarkCuckoo2T2SDelete(b *testing.B) {
	benchmarkCuckooDelete(ef, add, lf, tables, slots, "m332", b)
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
