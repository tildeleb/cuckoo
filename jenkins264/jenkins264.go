// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
// See http://burtleburtle.net/bob/c/lookup3.c and http://burtleburtle.net/bob/hash/evahash.html

package jenkins264

import (
	_ "fmt"
	_ "hash"
	"unsafe"
)

// original mix function
func mix64(a, b, c uint64) (uint64, uint64, uint64) {
	a = a - b
	a = a - c
	a = a ^ (c >> 43)
	b = b - c
	b = b - a
	b = b ^ (a << 9)
	c = c - a
	c = c - b
	c = c ^ (b >> 8)
	a = a - b
	a = a - c
	a = a ^ (c >> 38)
	b = b - c
	b = b - a
	b = b ^ (a << 23)
	c = c - a
	c = c - b
	c = c ^ (b >> 5)
	a = a - b
	a = a - c
	a = a ^ (c >> 35)
	b = b - c
	b = b - a
	b = b ^ (a << 49)
	c = c - a
	c = c - b
	c = c ^ (b >> 11)
	a = a - b
	a = a - c
	a = a ^ (c >> 12)
	b = b - c
	b = b - a
	b = b ^ (a << 18)
	c = c - a
	c = c - b
	c = c ^ (b >> 22)
	return a, b, c
}

// restated mix function better for gofmt
func mix64alt(a, b, c uint64) (uint64, uint64, uint64) {
	a -= b - c ^ (c >> 43)
	b -= c - a ^ (a << 9)
	c -= a - b ^ (b >> 8)
	a -= b - c ^ (c >> 38)
	b -= c - a ^ (a << 23)
	c -= a - b ^ (b >> 5)
	a -= b - c ^ (c >> 35)
	b -= c - a ^ (a << 49)
	c -= a - b ^ (b >> 11)
	a -= b - c ^ (c >> 12)
	b -= c - a ^ (a << 18)
	c -= a - b ^ (b >> 22)
	return a, b, c
}

// the following functions can be inlined
func mix64a(a, b, c uint64) (uint64, uint64, uint64) {
	a -= b - c ^ (c >> 43)
	b -= c - a ^ (a << 9)
	return a, b, c
}

func mix64b(a, b, c uint64) (uint64, uint64, uint64) {
	c -= a - b ^ (b >> 8)
	a -= b - c ^ (c >> 38)
	return a, b, c
}

func mix64c(a, b, c uint64) (uint64, uint64, uint64) {
	b -= c - a ^ (a << 23)
	c -= a - b ^ (b >> 5)
	return a, b, c
}

func mix64d(a, b, c uint64) (uint64, uint64, uint64) {
	a -= b - c ^ (c >> 35)
	b -= c - a ^ (a << 49)
	return a, b, c
}

func mix64e(a, b, c uint64) (uint64, uint64, uint64) {
	c -= a - b ^ (b >> 11)
	a -= b - c ^ (c >> 12)
	return a, b, c
}

func mix64f(a, b, c uint64) (uint64, uint64, uint64) {
	b -= c - a ^ (a << 18)
	c -= a - b ^ (b >> 22)
	return a, b, c
}

// This makes a new slice of uint64 that points to the same slice passed in as []byte.
// We should check alignment for architectures that don't handle unaligned reads and
// fallback to a copy.
// Unclear is we guarentee the same hash for different endianess.
func sliceUI64(in []byte) []uint64 {
	return (*(*[]uint64)(unsafe.Pointer(&in)))[:len(in)/8]
}

// Jenkin's second generation 64 bit hash.
// Benchmarked with 24 byte key, inlining, store of hash in memory (cache miss every 4 hashes) and fast=true at:
// benchmark64: 26 Mhashes/sec
// benchmark64: 623 MB/sec
func Hash(k []byte, seed uint64) uint64 {
	var fast = true // fast is really much faster
	//fmt.Printf("k=%v\n", k)
	//fmt.Printf("length=%d, len(k)=%d\n", length, len(k))

	//The 64-bit golden ratio is 0x9e3779b97f4a7c13LL
	length := uint64(len(k))
	a := uint64(0x9e3779b97f4a7c13)
	b := a
	c := seed
	if fast {
		k64 := sliceUI64(k)
		cnt := 0
		for i := length; i >= 24; i -= 24 {
			a += k64[0+cnt]
			b += k64[1+cnt]
			c += k64[2+cnt]
			// inlining is slightly faster
			a, b, c = mix64a(a, b, c)
			a, b, c = mix64b(a, b, c)
			a, b, c = mix64c(a, b, c)
			a, b, c = mix64d(a, b, c)
			a, b, c = mix64e(a, b, c)
			a, b, c = mix64f(a, b, c)
			k = k[24:]
			cnt += 3
			length -= 24
		}
	} else {
		for i := length; i >= 24; i -= 24 {
			a += uint64(k[0]) | uint64(k[1])<<8 | uint64(k[2])<<16 | uint64(k[3])<<24 | uint64(k[4])<<32 | uint64(k[5])<<40 | uint64(k[6])<<48 | uint64(k[7])<<56
			b += uint64(k[8]) | uint64(k[9])<<8 | uint64(k[10])<<16 | uint64(k[11])<<24 | uint64(k[12])<<32 | uint64(k[13])<<40 | uint64(k[14])<<48 | uint64(k[15])<<56
			c += uint64(k[16]) | uint64(k[17])<<8 | uint64(k[18])<<16 | uint64(k[19])<<24 | uint64(k[20])<<32 | uint64(k[21])<<40 | uint64(k[22])<<48 | uint64(k[23])<<56
			a, b, c = mix64alt(a, b, c)
			k = k[24:]
			length -= 24
		}
	}
	c += length
	if len(k) > 23 {
		panic("Hash264")
	}
	switch length {
	case 23:
		c += uint64(k[22]) << 56
		fallthrough
	case 22:
		c += uint64(k[21]) << 48
		fallthrough
	case 21:
		c += uint64(k[20]) << 40
		fallthrough
	case 20:
		c += uint64(k[19]) << 32
		fallthrough
	case 19:
		c += uint64(k[18]) << 24
		fallthrough
	case 18:
		c += uint64(k[17]) << 16
		fallthrough
	case 17:
		c += uint64(k[16]) << 8
		fallthrough
	case 16:
		b += uint64(k[15]) << 56 // the first byte of c is reserved for the length
		fallthrough
	case 15:
		b += uint64(k[14]) << 48
		fallthrough
	case 14:
		b += uint64(k[13]) << 40
		fallthrough
	case 13:
		b += uint64(k[12]) << 32
		fallthrough
	case 12:
		b += uint64(k[11]) << 24
		fallthrough
	case 11:
		b += uint64(k[10]) << 16
		fallthrough
	case 10:
		b += uint64(k[9]) << 8
		fallthrough
	case 9:
		b += uint64(k[8])
		fallthrough
	case 8:
		a += uint64(k[7]) << 56
		fallthrough
	case 7:
		a += uint64(k[6]) << 48
		fallthrough
	case 6:
		a += uint64(k[5]) << 40
		fallthrough
	case 5:
		a += uint64(k[4]) << 32
		fallthrough
	case 4:
		a += uint64(k[3]) << 24
		fallthrough
	case 3:
		a += uint64(k[2]) << 16
		fallthrough
	case 2:
		a += uint64(k[1]) << 8
		fallthrough
	case 1:
		a += uint64(k[0])
	case 0:
		break
	default:
		panic("HashWords64")
	}
	a, b, c = mix64alt(a, b, c)
	return c
}
