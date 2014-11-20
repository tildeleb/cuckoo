// +build kuint32,vuint32,slice

package cuckoo

type Key uint32
type Value uint32

type Buckets	[]Bucket		// slots
func makeSlots(b Buckets, slots int) Buckets {
	return make(Buckets, slots, slots)
}
