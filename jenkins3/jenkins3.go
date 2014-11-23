// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package jenkins3

// This package is a transliteration of Jenkins lookup3.c


import (
	_ "fmt"
	"hash"
	"unsafe"
)

type Digest struct {
	hash	uint32
	seed	uint32
	pc		uint32
	pb		uint32
	clen	int
	tail	[]byte
}

// The size of an jenkins3 32 bit hash in bytes.
const Size = 4

// Make sure interfaces are correctly implemented. Stolen from another implementation.
// I did something similar in another package to verify the interface but didn't know you could elide the variable in a var.
// What a cute wart it is.
var (
	//_ hash.Hash   = new(Digest)
	_ hash.Hash32 = new(Digest)
)

func hashword(k []uint32, length int, seed uint32) uint32 {
	var a, b, c uint32
	var rot = func(x, k uint32) uint32 {
		return x << k | x >> (32 - k)
	}
	var mix = func() {
		a -= c;  a ^= rot(c, 4); c += b;
		b -= a;  b ^= rot(a, 6);  a += c;
		c -= b;  c ^= rot(b, 8);  b += a;
		a -= c;  a ^= rot(c,16);  c += b;
		b -= a;  b ^= rot(a,19);  a += c;
		c -= b;  c ^= rot(b, 4);  b += a;
	}
	var final = func() {
		c ^= b; c -= rot(b,14);
		a ^= c; a -= rot(c,11);
		b ^= a; b -= rot(a,25);
		c ^= b; c -= rot(b,16);
		a ^= c; a -= rot(c,4);
		b ^= a; b -= rot(a,14);
		c ^= b; c -= rot(b,24);
	}
	ul := uint32(len(k))
	a = 0xdeadbeef + ul<<2 + seed
	b, c = a, a

	i := 0
	//length := 0
	for length = len(k); length > 3; length -= 3 {
		a += k[i + 0]
		b += k[i + 1]
		c += k[i + 2]
		mix()
		i += 3
	}

	switch(length) {
	case 3:
		c += k[i + 2]
		fallthrough
	case 2:
		b += k[i + 1]
		fallthrough
	case 1:
		a += k[i + 0]
		final()
  	case 0:
		break
	}
	return c
}


func stu(s string) (u []uint32) {
	l := (len(s) + 3) / 4
	d := make([]uint32, l)
	b := ([]byte)(s)
	for i := 0; i < l; i += 4 {
		t := *(*uint32)(unsafe.Pointer(&b[i*4]))
		d = append(d, t)
	}
	return d
}


/*
 * hashlittle2: return 2 32-bit hash values
 *
 * This is identical to hashlittle(), except it returns two 32-bit hash
 * values instead of just one.  This is good enough for hash table
 * lookup with 2^^64 buckets, or if you want a second hash if you're not
 * happy with the first, or if you want a probably-unique 64-bit ID for
 * the key.  *pc is better mixed than *pb, so use *pc first.  If you want
 * a 64-bit value do something like "*pc + (((uint64_t)*pb)<<32)".
 */

func jenkins364(k []byte, pc, pb uint32) (rpc, rpb uint32) {
	var a, b, c uint32
	var rot = func(x, k uint32) uint32 {
		return x << k | x >> (32 - k)
	}
	var mix = func() {
		a -= c;  a ^= rot(c, 4); c += b;
		b -= a;  b ^= rot(a, 6);  a += c;
		c -= b;  c ^= rot(b, 8);  b += a;
		a -= c;  a ^= rot(c,16);  c += b;
		b -= a;  b ^= rot(a,19);  a += c;
		c -= b;  c ^= rot(b, 4);  b += a;
	}
	var final = func() {
		c ^= b; c -= rot(b,14);
		a ^= c; a -= rot(c,11);
		b ^= a; b -= rot(a,25);
		c ^= b; c -= rot(b,16);
		a ^= c; a -= rot(c,4);
		b ^= a; b -= rot(a,14);
		c ^= b; c -= rot(b,24);
	}
	ul := uint32(len(k))
	//fmt.Printf("s=%q, k=%v, len(s)=%d, len(k)=%d\n", s, k, len(s), len(k))

	/* Set up the internal state */
	a = 0xdeadbeef + ul + pc
	b, c = a, a
	c += pb

	length := 0
	for length = len(k); length > 12; length -= 12 {
		//fmt.Printf("k=%q, length=%d\n", k, length)
		a += *(*uint32)(unsafe.Pointer(&k[0]))
		b += *(*uint32)(unsafe.Pointer(&k[4]))
		c += *(*uint32)(unsafe.Pointer(&k[8]))
		mix()
		k = k[12:]
	}
	//fmt.Printf("k=%q, length=%d\n", k, length)

    /* handle the last (probably partial) block */
    /* 
     * "k[2]&0xffffff" actually reads beyond the end of the string, but
     * then masks off the part it's not allowed to read.  Because the
     * string is aligned, the masked-off tail is in the same word as the
     * rest of the string.  Every machine with memory protection I've seen
     * does it on word boundaries, so is OK with this.  But VALGRIND will
     * still catch it and complain.  The masking trick does make the hash
     * noticably faster for short strings (like English words).
     */

 	//fmt.Printf("length now=%d\n", length)
	switch length {
    case 12:
    	a += *(*uint32)(unsafe.Pointer(&k[0]))
    	b += *(*uint32)(unsafe.Pointer(&k[4]))
    	c += *(*uint32)(unsafe.Pointer(&k[8]))
    case 11:
    	c += uint32(k[10])<<16
    	fallthrough
    case 10:
    	c += uint32(k[9])<<8
    	fallthrough
    case 9:
    	c += uint32(k[8])
    	fallthrough
    case 8:
    	a += *(*uint32)(unsafe.Pointer(&k[0]))
    	b += *(*uint32)(unsafe.Pointer(&k[4]))
    	break
    case 7:
    	b += uint32(k[6])<<16
    	fallthrough
    case 6:
    	b += uint32(k[5])<<8
    	fallthrough
    case 5:
    	b += uint32(k[4])
    	fallthrough
    case 4:
    	a += *(*uint32)(unsafe.Pointer(&k[0]))
    	break
    case 3:
    	a +=  uint32(k[2])<<16
    	fallthrough
    case 2:
    	a +=  uint32(k[1])<<8
    	fallthrough
    case 1:
    	a += uint32(k[0])
    	break
    case 0:
    	//fmt.Printf("case 0\n")
    	return c, b  /* zero length strings require no mixing */
    }
	final()
	return c, b
}

func hashlittle2(s string, pc, pb uint32) (rpc, rpb uint32) {
	k := ([]byte)(s)
	rpc, rpb = jenkins364(k, pc, pb)
	return
}

// Sum32 returns the 32 bit hash of data given the seed.
// This is code is what I started with before I added the hash.Hash and hash.Hash32 interfaces.
func Sum32(data []byte, seed uint32) uint32 {
	rpc, _ := jenkins364(data, seed, seed)
	return rpc
}

// New returns a new hash.Hash32 interface that computes the a 32 bit murmur3 hash.
func New(seed uint32) hash.Hash32 {
	d := new(Digest)
	d.seed = seed
	d.Reset()
	return d
}

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

// Accept a byte stream p used for calculating the hash. For now this call is lazy and the actual hash calculations take place in Sum() and Sum32().
func (d *Digest) Write(p []byte) (nn int, err error) {
	l := len(p)
	d.clen += l
	d.tail = append(d.tail, p...)
	return l, nil
}

// Return the current hash as a byte slice.
func (d *Digest) Sum(b []byte) []byte {
	d.pc, d.pb = jenkins364(d.tail, d.pc, d.pb)
	d.hash = d.pc
	h := d.pc
	return append(b, byte(h>>24), byte(h>>16), byte(h>>8), byte(h))
}

// Return the current hash as a 32 bit unsigned type.
func (d *Digest) Sum32() uint32 {
	d.pc, d.pb = jenkins364(d.tail, d.pc, d.pb)
	d.hash = d.pc
	return d.hash
}