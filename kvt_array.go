// +build !slice

package cuckoo

const Slots = 16

type Buckets	[Slots]Bucket		// slots
func makeSlots(b Buckets, slots int) Buckets {
	return b
}
