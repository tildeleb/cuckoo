// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.
// +build slice

package cuckoo

type Slots []Bucket

func makeSlots(s Slots, slots int) Slots {
	return make(Slots, slots, slots)
}
