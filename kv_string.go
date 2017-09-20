// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.
// +build string

package cuckoo

import "hash"

type Key string
type Value string

func (c *Cuckoo) _calcHash(hf hash.Hash64, seed uint64, key Key) (h uint64) {
	// ok we have to copy the key now as all the other hash functions want a slice of bytes.
	switch c.NumericKeySize {
	case 4:
		panic("_calcHash: 4")
	case 8:
		panic("_calcHash: 8")
	default:
		c.buf.Reset()
		if err := c.encoder.Encode(&key); err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		c.buf.b = c.buf.base[0:c.buf.i]
	}
	if c.hfb != nil {
		h = c.hfb(c.buf.b, seed) % uint64(c.Buckets)
	} else {
		hf.Reset()
		hf.Write(c.buf.b)
		h1 := uint64(hf.Sum64())
		h = h1 % uint64(c.Buckets)
	}
	return
}

// Given a key and a hash function to use, calculate the hash for the specified table.
// To do this we have to serialize the key
// To get this to inline the optimization for NumericKeySize == 4 was moved to _calcHash ???
// check to see this this inlines with SSA
func (c *Cuckoo) calcHash(hf hash.Hash64, seed uint64, key Key) uint64 {
	// speed up a common key case
	//fmt.Printf("%d ", c.NumericKeySize)
	return c._calcHash(hf, seed, key)
}
