// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.
// +build slice

package cuckoo

type Buckets []Bucket // slots
func makeSlots(b Buckets, slots int) Buckets {
	return make(Buckets, slots, slots)
}
