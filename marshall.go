// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// Package cuckoo implements a cuckoo hash table.
// With the correct options this data structure can achieve 5X more storage efficiency
// over Go's builtin map with similar performance. See the "README.md" file for all the details.
// Edit the file "kv_default.go" to define the types for you Key and Value.
package cuckoo

import (
	_ "bytes"
	_ "hash"
	_ "leb.io/aeshash" // no longer self contained package
	_ "leb.io/cuckoo/jenkins264"
	_ "leb.io/cuckoo/jk3"
	_ "leb.io/cuckoo/murmur3"
	_ "math"
)

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

/*
	if c.hashno == aes {
		h = aeshash.Hash(c.buf.b, seed) % uint64(c.Buckets)
	} else {
		if c.hashno == j264 {
			h = jenkins264.Hash(c.buf.b, seed) % uint64(c.Buckets)
		} else {
			panic("_calcHash")
			panic("hash interface")
*/
