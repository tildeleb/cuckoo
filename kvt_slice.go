// +build slice

package cuckoo

type Buckets	[]Bucket		// slots
func makeSlots(b Buckets, slots int) Buckets {
	return make(Buckets, slots, slots)
}