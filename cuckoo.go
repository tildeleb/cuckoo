// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
package cuckoo

import "fmt"
import _ "math"
import "hash"
import "bytes"
import "math/rand"
import "encoding/binary"
import "leb/cuckoo/murmur3"
import "unsafe"

var StartLevel = 100 // was 3
var LowestLevel = -8000

var zeroVal Value

type bucket struct {
	key		Key
	val		Value
}

type CuckooStat struct {
	BucketSize		int
	Elements		int				// number of elements in the table
	Inserts			int
	Attempts		int
	Iterations		int
	Deletes			int
	Lookups			int
	Fails			int
	Bumps			int
	Aborts			int
	Limited			bool
}

// Per table stats
type TableStat struct {
	Size		int
	Elements	int
	Bumps		int
}

// xxx
type Config struct {
	LoadFactor		float64
	Tables			int
	Buckets			int
	Slots			int
	Size			int
	MaxElements		int
}

type Cuckoo struct {
	tbs				[][][]bucket	// alignment lives here, your data stored here.
	Config							// config data
	CuckooStat						// stats
	TableStats		[]TableStat		// per table stats
	seeds			[]uint32		// seeds used per table
	hf				[]hash.Hash32	// one for each table + the last one reserved for fingerprints
	hs				[]uint32		// hash sums for each table and fingerprint
	//stash			[]bucket		// store keys that fail insert
	r				func() float64	// random numbers for eviction
	emptyKey		Key				// empty key
	emptyValue		Value			// if empty key store value lives here and not in a hash table
	emptyKeyValid	bool			// something store here
}

func (c *Cuckoo) rbetween(a int, b int) int {
	rf := c.r()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

func (c *Cuckoo) getHashes(hashName string, n int) []hash.Hash32 {
	var h []hash.Hash32
	for i := 0; i < n; i++ {
		seed := i
		if seed == n {
			seed = 0
		}
		c.seeds[i] = uint32(seed)
		switch hashName {
		case "m332":
			h = append(h, murmur3.New(uint32(seed)))
		default:
			s := fmt.Sprintf("cuckoo: unknown hash function %q\n", hashName)
			panic(s)
		}
	}
	return h
}

func New(tables, buckets, slots int, loadFactor float64, emptyKey ...Key) *Cuckoo {
	var b bucket

	c := &Cuckoo{}
	c.Tables, c.Buckets, c.Slots =  tables, buckets, slots
	if len(emptyKey) > 0 {
		c.emptyKey = emptyKey[0]
	}
	c.r = rand.Float64
	c.Size = tables * buckets * slots
	c.BucketSize = int(unsafe.Sizeof(b))
	c.LoadFactor = loadFactor
	c.MaxElements = int(float64(c.Size) * loadFactor)
	//c.stash = make([]bucket, 8)
	c.TableStats = make([]TableStat, tables)
	c.tbs = make([][][]bucket, tables, tables)
	c.seeds = make([]uint32, tables)
	c.hf = c.getHashes("m332", tables)
	c.hs = make([]uint32, len(c.hf))
	for t, _ := range c.tbs {
		c.tbs[t] = make([][]bucket, buckets, buckets)
		c.TableStats[t].Size = buckets * slots
		for b, _ := range c.tbs[t] {
			c.tbs[t][b] = make([]bucket, slots, slots)
			for s, _ := range c.tbs[t][b] {
				c.tbs[t][b][s].val = 0
			}
		}
	}
	return c
}


func (c *Cuckoo) Lookup(key Key) (Value, bool) {
	buf := new(bytes.Buffer)
	b := make([]byte, 4)
	err := binary.Write(buf, binary.LittleEndian, int32(key))
	if err != nil {
		//fmt.Printf("Write: err=%q\n", err)
		panic("Lookup: binary.Write")
	}
	if l, err2 := buf.Read(b); l != 4 {
		fmt.Printf("l=%d, err=%q\n", l, err2)
		panic("Lookup: Read")
	}
	c.Lookups++

	if key == c.emptyKey {
		if c.emptyKeyValid {
			return c.emptyValue, true
		} else {
			return zeroVal, false
		}
	}

	// calculate hashes for each table
	for k, v := range c.hf {
		v.Reset()
		v.Write(b)
		c.hs[k] = v.Sum32() % uint32(c.Buckets)
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}
	for t, _ := range c.tbs {
		b := c.hs[t]
		for s, _ := range c.tbs[t][b] {
			//fmt.Printf("Lookup: key=%d, table=%d, bucket=%d, slot=%d, found key=%d\n", key, t, b, s, c.tbs[t][b][s].key)
			if c.tbs[t][b][s].key == key {
				//fmt.Printf("Lookup: key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", key, val, t, b, s)
				return c.tbs[t][b][s].val, true
			}
		}
	}
	return zeroVal, false
}

func (c *Cuckoo) Delete(key Key) (bool, Value) {
	buf := new(bytes.Buffer)
	b := make([]byte, 4)
	err := binary.Write(buf, binary.LittleEndian, int32(key))
	if err != nil {
		//fmt.Printf("Write: err=%q\n", err)
		panic("Delete: binary.Write")
	}
	if l, err2 := buf.Read(b); l != 4 {
		fmt.Printf("Delete: l=%d, err=%q\n", l, err2)
		panic("Delete: Read")
	}
	c.Deletes++

	if key == c.emptyKey {
		if c.emptyKeyValid {
			c.emptyKeyValid = false
			return true, c.emptyValue
		} else {
			return false, zeroVal
		}
	}

	// calculate hashes for each table
	for k, v := range c.hf {
		v.Reset()
		v.Write(b)
		c.hs[k] = v.Sum32() % uint32(c.Buckets)
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}
	for t, _ := range c.tbs {
		b := c.hs[t]
		for s, _ := range c.tbs[t][b] {
			//fmt.Printf("Delete: check key=%d, table=%d, bucket=%d, slot=%d, found key=%d\n", key, t, b, s, c.tbs[t][b][s].key)
			if c.tbs[t][b][s].key == key {
				//fmt.Printf("Delete: found key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", key, c.tbs[t][b][s].val, t, b, s)
				c.tbs[t][b][s].key = c.emptyKey 
				c.TableStats[t].Elements--
				c.Elements--
				if c.Elements < 0 {
					panic("Delete")
				}
				return true, c.tbs[t][b][s].val
			}
		}
	}
	return false, zeroVal
}

func (c *Cuckoo) insert(key Key, val Value, level int) (ok bool, rlevel int) {
	var slow = false
	b := make([]byte, 4)
	var ins func(kx Key, vx Value) bool // forwqrd declare the closure so we can call it recursively
	var _ = func() {
		fmt.Printf("<")
		for k, v := range c.hs {
			if k != 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%d", v)
		}
		fmt.Printf(">\n")
	}
	var calcHashes = func(key Key) {
		if slow {
			buf := new(bytes.Buffer)
			//b := make([]byte, 4)
			err := binary.Write(buf, binary.LittleEndian, int32(key))
			if err != nil {
				//fmt.Printf("Write: err=%q\n", err)
				panic("Insert: binary.Write")
			}
			if l, err2 := buf.Read(b); l != 4 {
				fmt.Printf("l=%d, err=%q\n", l, err2)
				panic("Insert: Read")
			}
		} else {
			b[0], b[1], b[2], b[3] = byte(key&0xFF), byte((key>>8)&0xFF), byte((key>>16)&0xFF), byte((key>>24)&0xFF)
		}
		// calculate hashes for each table
		for k, v := range c.hf {
			if true {
				v.Reset()
				v.Write(b)
				h1 := v.Sum32()
				c.hs[k] = h1 % uint32(c.Buckets)
				//fmt.Printf("c.hs[%d]=0x%x, Sum32(b)=0x%x\n", k, h1, murmur3.Sum32(b, c.seeds[k]))
			} else {
				c.hs[k] = murmur3.Sum32(b, c.seeds[k]) % uint32(c.Buckets)
			}
			//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
		}
	}
	ins = func(kx Key, vx Value) bool {
		calcHashes(kx)
		//fmt.Printf("Insert: level=%d, key=%d, ", level, kx)
		//phv()
		k := kx
		v := vx
		for t, _ := range c.tbs {
			//phv()
			b := c.hs[t]
			//fmt.Printf("Insert: next table, level=%d, key=%d, value=%d, table=%d, bucket=%d\n", level, k, v, t, b)
			for s, _ := range c.tbs[t][b] {
				c.Attempts++
				if c.tbs[t][b][s].key == 0 {
					c.tbs[t][b][s].key, c.tbs[t][b][s].val = k, v
					//fmt.Printf("Insert: level=%d, key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", level, k, v, t, b, s)
					c.Elements++
					c.TableStats[t].Elements++
					return true
				}
			}
			// no slots available in any table available, pick a random key to move to the next table
			c.Bumps++
			c.TableStats[t].Bumps++
			victim := c.rbetween(0, c.Slots-1)
			//fmt.Printf("insert: level=%d, bump value=%d for value=%d, table=%d, bucket=%d, slot=%d\n", level, c.tbs[t][b][victim].val, val, t, b, victim)
			bucket := c.tbs[t][b][victim]
			c.tbs[t][b][victim].key = k
			c.tbs[t][b][victim].val = v
			k = bucket.key
			v = bucket.val
			calcHashes(k)
			//fmt.Printf("insert: level=%d, new key=%d, val=%d\n", level, k, v)
		}
		c.Iterations++
		level--

		// skip 0 because it's used as a signal that Insert failed because of load constaint
		if level == 0 {
			level = -1
		}
		if level <= LowestLevel {
			fmt.Printf("cukcoo: Insert FAILED")
			return false
		}
		if level <= 0 {
			c.Aborts++
			// sublte bug, on failure to insert the key not inserted may not be the original key
			// so keep interating until the original key is not found to prevent data loss
			_, found := c.Lookup(key)
			//fmt.Printf("key %d found=%v\n", key, found)
			if !found {
				return false
			}
		}
		return ins(k, v)
	}

	// function starts here
	//fmt.Printf("Insert: level=%d, key=%d, value=%d\n", level, key, val)
	if c.Elements >= c.MaxElements {
		c.Limited = true
		return false, 0
	}
	if key == c.emptyKey {
		if c.emptyKeyValid {
			panic("emptyKeyValid")
		} else {
			c.Elements++
			c.emptyKeyValid = true
			c.emptyValue = val
		}
		return true, level
	}
	aok := ins(key, val)
	if aok {
		c.Inserts++
	}
	return aok, level
}

func (c *Cuckoo) Insert(key Key, val Value) (ok bool, rlevel int) {
	return c.insert(key, val, StartLevel)
}

/*
func init() {
	var k Key
	var v Value = 1// "foobar"

	fmt.Printf("Key=%T\n", k)
	fmt.Printf("Value=%T\n", v)
}
*/
