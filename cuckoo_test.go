// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package cuckoo_test

import . "leb/cuckoo"
//import "flag"
import "fmt"
import "math/rand"
import "testing"

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
