// Copyright © 2014, 2015, 2016 Lawrence E. Bakst. All rights reserved.
// +build !noaes

package cuckoo

import (
	"errors"
	"hash"

	"leb.io/aeshash"
	"leb.io/cuckoo/internal/jenkins264"
	"leb.io/cuckoo/internal/jenkins3"
)

// Set hash function used
func (c *Cuckoo) setHash(hashName string) (int, error) {
	switch hashName {
	case "":
		fallthrough
	case "aes":
		c.hf64 = aeshash.Hash64
		c.hf32 = aeshash.Hash32
		c.hfb = aeshash.Hash
		c.hf = aeshash.NewAES(0)
		return aes, nil
	case "j364":
		c.hf64 = nil
		c.hf32 = nil
		c.hfb = jenkins3.HashBytes
		c.hf = jenkins3.New(uint32(0))
		return j364, nil
	case "j264":
		c.hf64 = nil
		c.hf32 = nil
		c.hfb = jenkins264.Hash
		return j264, nil
	default:
		// fallthrough generated bad error messaage Go bug
	}
	return 0, errors.New("cuckoo: invalid hash name")
}

// Get a hash function with a specific seed
func (c *Cuckoo) getHash(hashName string, seed uint64) hash.Hash64 {
	switch hashName {
	case "":
		fallthrough
	case "aes":
		return aeshash.NewAES(seed)
	case "j264":
		return jenkins264.New(seed)
	case "j364":
		return jenkins3.New(uint32(seed))
	}
	panic("getHash")
}
