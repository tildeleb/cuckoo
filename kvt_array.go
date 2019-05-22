// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.
// +build !slice

package cuckoo

const Nslots = 8

type Slots [Nslots]Bucket // slots
func makeSlots(s Slots, slots int) Slots {
	return s
}
