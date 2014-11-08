// © Copyright 2014 Lawrence E. Bakst All Rights Reserved
// THIS SOURCE CODE IS THE PROPRIETARY INTELLECTUAL PROPERTY AND CONFIDENTIAL
// INFORMATION OF LAWRENCE E. BAKST AND IS PROTECTED UNDER U.S. AND
// INTERNATIONAL LAW. ANY USE OF THIS SOURCE CODE WITHOUT THE
// AUTHORIZATION OF LAWRENCE E. BAKST IS STRICTLY PROHIBITED.

// This package implements the 32 bit version of the MurmurHash3 hash code.
// With the exception of the interface check, this version was developed independtly.
// However, the "spaolacci" implementation with it's bmixer interface is da bomb, although
// this version is slightly faster.
//
// https://en.wikipedia.org/wiki/MurmurHash
// https://github.com/spaolacci/murmur3
package murmur3

import _ "fmt"
import "hash"
import "unsafe"

const (
	c1	uint32 = 0xcc9e2d51
	c2	uint32 = 0x1b873593
	r1	uint32 = 15
	r2	uint32 = 13
	m	uint32 = 5
	n	uint32 = 0xe6546b64
)

type Digest struct {
	hash	uint32
	seed	uint32
	clen	int
	tail	[]byte
}

// The size of an murmur3 32 bit hash in bytes.
const Size = 4

// Make sure interfaces are correctly implemented. Stolen from another implementation.
// I did something similar in another package to verify the interface but didn't know you could elide the variable in a var.
// What a cute wart it is.
var (
	_ hash.Hash   = new(Digest)
	_ hash.Hash32 = new(Digest)
)

// New returns a new hash.Hash32 interface that computes the a 32 bit murmur3 hash.
func New(seed uint32) hash.Hash32 {
	d := new(Digest)
	d.seed = seed
	d.Reset()
	return d
}

// The following methods implment the hash.Hash abd hash.Hash32 interfaces

// Reset the hash state.
func (d *Digest) Reset() {
	d.hash = d.seed
	d.clen = 0
	d.tail = nil
}

// Return the size of the resulting hash.
func (d *Digest) Size() int { return Size }

// Return the blocksize of the hash which in this case is 1 byte.
func (d *Digest) BlockSize() int { return 1 }

func (d *Digest) murmur332Blocks() {
	nblocks := len(d.tail) / 4
	if nblocks <= 0 {
		return
	}
	for i := 0; i < nblocks; i++ {
		k := *(*uint32)(unsafe.Pointer(&d.tail[i*4]))
		//k = uint32(d.tail[i*4+0])<<0 | uint32(d.tail[i*4+1])<<8 | uint32(d.tail[i*4+2])<<16 | uint32(d.tail[i*4+3])<<24
		k *= c1
		k = (k << r1) | (k >> (32 - r1))
		k *= c2
		d.hash ^= k
		d.hash = ((d.hash << r2) | (d.hash >> (32 - r2))) * m + n
	}
	d.tail = d.tail[nblocks*4:]
}

func (d *Digest) murmur332Tail() {
	hash := d.hash
	k1 := uint32(0)
	l := len(d.tail) & 3; switch (l) {
	case 3:
		k1 ^= uint32(d.tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(d.tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(d.tail[0])
		k1 *= c1
		k1 = (k1 << r1) | (k1 >> (32 - r1))
		k1 *= c2
		hash ^= k1
	case 0:
		break
	default:
		panic("murmur332Tail")
	}
	hash ^= uint32(d.clen)
	hash ^= hash >> 16
	hash *= 0x85ebca6b
	hash ^= hash >> 13
	hash *= 0xc2b2ae35
	hash ^= hash >> 16
	d.hash = hash
}


// Accept a byte stream p used for calculating the hash. For now this call is lazy and the actual hash calculations take place in Sum() and Sum32().
func (d *Digest) Write(p []byte) (nn int, err error) {
	l := len(p)
	d.clen += l
	d.tail = append(d.tail, p...)
	return l, nil
}

// Return the current hash as a byte slice.
func (d *Digest) Sum(b []byte) []byte {
	d.murmur332Blocks()
	d.murmur332Tail()
	h := d.hash
	return append(b, byte(h>>24), byte(h>>16), byte(h>>8), byte(h))
}

// Return the current hash as a 32 bit unsigned type.
func (d *Digest) Sum32() uint32 {
	d.murmur332Blocks()
	d.murmur332Tail()
	return d.hash
}

// Sum32 returns the 32 bit hash of data given the seed.
// This is code is what I started with before I added the hash.Hash and hash.Hash32 interfaces.
func Sum32(data []byte) uint32 {
	hash := uint32(0)
	nblocks := len(data) / 4
	for i := 0; i < nblocks; i++ {
		k := *(*uint32)(unsafe.Pointer(&data[i*4]))
		//k := uint32(data[i*4+0])<<0 | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		k *= c1
		k = (k << r1) | (k >> (32 - r1))
		k *= c2
		hash ^= k
		hash = ((hash << r2) | (hash >> (32 - r2))) * m + n
	}

	l := nblocks * 4; k1 := uint32(0); switch (len(data) & 3) {
	case 3:
		k1 ^= uint32(data[l+2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(data[l+1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(data[l+0])
		k1 *= c1
		k1 = (k1 << r1) | (k1 >> (32 - r1))
		k1 *= c2
		hash ^= k1
	}
 
	hash ^= uint32(len(data))
	hash ^= hash >> 16
	hash *= 0x85ebca6b
	hash ^= hash >> 13
	hash *= 0xc2b2ae35
	hash ^= hash >> 16
	return hash
}
