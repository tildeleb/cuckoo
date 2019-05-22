// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.
// +build !string

// !kuint32,!vuint32

package cuckoo

import "hash"

type Key uint64
type Value uint64

func (c *Cuckoo) _calcHash(hf hash.Hash64, seed uint64, key Key) (h uint64) {
	// ok we have to copy the key now as all the other hash functions want a slice of bytes.
	switch c.NumericKeySize {
	case 4:
		c.buf.b = c.buf.base[0:4]
		c.buf.b[0], c.buf.b[1], c.buf.b[2], c.buf.b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
	case 8:
		c.buf.b = c.buf.base[0:8]
		c.buf.b[0], c.buf.b[1], c.buf.b[2], c.buf.b[3], c.buf.b[4], c.buf.b[5], c.buf.b[6], c.buf.b[7] =
			byte(key), byte(key>>8), byte(key>>16), byte(key>>24), byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
	default:
		c.buf.Reset()
		if err := c.encoder.Encode(&key); err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		c.buf.b = c.buf.base[0:c.buf.i]
	}
	if c.hfb != nil {
		h = c.hfb(c.buf.b, seed) % uint64(c.Nbuckets)
	} else {
		hf.Reset()
		hf.Write(c.buf.b)
		h1 := uint64(hf.Sum64())
		h = h1 % uint64(c.Nbuckets)
	}
	return
}

// inlined functions

// the following 5 functions/methods can be inlined.
func ui32tob(b []byte, key Key) {
	b[0], b[1], b[2], b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
}

func ui64tob(b []byte, key Key) {
	b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24), byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
}

func ui64tob1(b []byte, key Key) {
	b[0], b[1], b[2], b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
}

func ui64tob2(b []byte, key Key) {
	b[4], b[5], b[6], b[7] = byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
}

// Given a key and a hash function to use, calculate the hash for the specified table.
// To do this we have to serialize the key
// To get this to inline the optimization for NumericKeySize == 4 was moved to _calcHash ???
// check to see this this inlines with SSA
func (c *Cuckoo) calcHash(hf hash.Hash64, seed uint64, key Key) uint64 {
	// speed up a common key case
	//fmt.Printf("%d ", c.NumericKeySize)
	//fmt.Printf("calcHash: seed=%d, key=%v\n", seed, key)
	if c.hashno == aes {
		if c.NumericKeySize == 8 && c.hf64 != nil {
			//fmt.Printf("8 key=%v, h=%v\n", uint64(key), c.hf64(uint64(key), seed))
			//ui64tob1(c.buf.b, key)
			//ui64tob2(c.buf.b, key)
			return c.hf64(uint64(key), seed)
		} else {
			//fmt.Printf("4")
			if c.NumericKeySize == 4 && c.hf32 != nil {
				//ui32tob(c.buf.b, key)
				return c.hf32(uint32(key), seed)
			}
		}
	}
	return c._calcHash(hf, seed, key)
}

//type Value interface{}
