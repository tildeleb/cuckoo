// -build kuint32 vuint32 array

package cuckoo

type Key uint32
type Value uint32

const Slots = 8

type Buckets	[Slots]Bucket		// slots
func makeSlots(b Buckets, slots int) Buckets {
	return b
}
