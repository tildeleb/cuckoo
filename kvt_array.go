// +build !slice

package cuckoo

const Slots = 8

type Buckets [Slots]Bucket // slots
func makeSlots(b Buckets, slots int) Buckets {
	return b
}
