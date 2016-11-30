// +build !hash-all

// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

package cuckoo

import (
	_ "fmt"
	"hash"
	_ "leb.io/aeshash"
	_ "leb.io/cuckoo/jenkins264"
)

const (
	aes  = 1
	j264 = 2
)

func setHash(hashName string) int {
	switch hashName {
	case "aes":
		return aes
	case "":
		fallthrough
	default:
		fallthrough
	case "j264":
		return j264
	}
}

// Select a hash function.
func getHash(hashName string, seed uint64) hash.Hash64 {
	switch hashName {
	default:
		return nil
	}
}

// 		s := fmt.Sprintf("cuckoo: unknown hash function %q\n", hashName)
//		panic(s)
