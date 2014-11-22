// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// Package cuckoo implements a cuckoo hash table. With the write options this data structure can achieve 5X more storage efficiency
// as Go's builtin map with similar performance. See the README.MD file for all the details.
// Edit the file kv_default.go to 
package cuckoo

import "fmt"
import _ "math"
import "hash"
import "bytes"
import "math/rand"
import "encoding/binary"
import "leb/cuckoo/murmur3"
import "leb/cuckoo/primes"
import "unsafe"

var zeroKey	Key
var zeroVal Value

// For historical reasons this is called a Bucket but should really be called an element
type Bucket struct {
	key		Key
	val		Value
}

// Counters
type Counters struct {
	BucketSize		int		// size of a single bucket (1 slot) in bytes
	Elements		int		// number of elements currently residing in the data structure
	Inserts			int		// number of time insert has been called
	Attempts		int		// number of attempts to insert all elements
	Iterations		int		// number of iterations through all the hash tables to attemps an insert
	Deletes			int		// number of times delete has been called
	Lookups			int		// number of lookups
	Fails			int		// number of times that insert failed
	Bumps			int		// number of evicted buckets
	Aborts			int		// number of times an insert had to aborted
	MaxAttempts		int		// highest number of attempts
	MaxIterations	int		// highest number of interations
	Limited			bool	// were inserts limited by a load factor
}

// Per table stats
type TableCounters struct {
	Size			int		// c.Buckets * c.Slots
	Elements		int		// number of elements currently residing in this hash table
	Bumps			int		// number of evicted buckets
}

// These two constants seem to work well for many cases, but not all
const (
	InitialStartLevel = 2000
	InitialLowestLevel = -8000
)
type Config struct {
	MaxLoadFactor	float64	// don't allow more than MaxElements = Tables * Buckets * Slots elements
	StartLevel		int		// starting value for level which is decremented for each insertion attempt
	LowestLevel		int		// This is usually a negative number and defines how far level can be decremented
	Tables			int		// number of hash tables
	Buckets			int		// number of buckets
	Slots			int		// number of slots
	Size			int		// Size = Tables * Buckets * Slots
	MaxElements		int		// maximum number of elements the data structure can hold
	HashName		string	// name of hashing function used
}

// main data structure for cuckoo hash
type Cuckoo struct {
	tbs				[][]Buckets		// alignment lives here, your data stored here.
	Config							// config data
	Counters						// stats
	TableCounters	[]TableCounters	// per table stats
	seeds			[]uint32		// seeds used per table
	hf				[]hash.Hash32	// one for each table + the last one reserved for fingerprints
	hs				[]uint32		// hash sums for each table and fingerprint
	b				[]byte			// used for result of marshalled data
	buf				*bytes.Buffer	// for marshalling data
	r				func() float64	// random numbers for eviction
	emptyKey		Key				// empty key
	emptyValue		Value			// if empty key store value lives here and not in a hash table
	emptyKeyValid	bool			// something store here
	ekiz			bool			// empty key is zero
	grow			bool
	NumericKeySize	int				// if key is numeric what is size in bytes
}

// Get the value of some of the counters, need to finish them all XXX
func (c *Cuckoo) GetCounter(s string) int {
	switch s {
	case "bumps":
		return c.Bumps
	case "inserts":
		return c.Inserts
	case "elements":
		return c.Elements
	case "size":
		return c.Size
	default:
		panic("GetCounter")
	}
}

// Get the value of some of the table counters
func (c *Cuckoo) GetTableCounter(t int, s string) int {
	if t > len(c.TableCounters) {
		panic("GetTableCounter")
	}
	switch s {
	case "size":
		return c.TableCounters[t].Size
	case "elements":
		return c.TableCounters[t].Elements
	case "bumps":
		return c.TableCounters[t].Bumps
	default:
		panic("GetTableCounter")
	}
}

// This function used to select a victim bucket to be evicted.
func (c *Cuckoo) rbetween(a int, b int) int {
	rf := c.r()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

// Select a hash function.
func getHash(hashName string, seed int) hash.Hash32 {
	switch hashName {
	case "m332":
		return murmur3.New(uint32(seed))
	default:
		s := fmt.Sprintf("cuckoo: unknown hash function %q\n", hashName)
		panic(s)
	}
}

// Add a hash function to a slice of hash functions.
func (c *Cuckoo) addHash() {
	c.seeds = append(c.seeds, uint32(len(c.seeds) + 1))
	c.hf = append(c.hf, getHash(c.HashName, int(c.seeds[len(c.seeds) - 1])))
	c.hs = append(c.hs, 0)
	c.TableCounters = append(c.TableCounters, TableCounters{Size: c.Buckets * c.Slots})
	//fmt.Printf("c.seeds=%#v\n", c.seeds)
	//fmt.Printf("c.hf=%#v\n", c.hf)
	//fmt.Printf("c.TableStats=%#v\n", c.TableStats)
/*
	if len(c.seeds) > 1 {
		c.seeds[0], c.seeds[len(c.seeds) - 1] = c.seeds[len(c.seeds) - 1], c.seeds[0]
		c.hf[0], c.hf[len(c.hf) - 1] = c.hf[len(c.hf) - 1], c.hf[0]
		c.hs[0], c.hs[len(c.hs) - 1] = c.hs[len(c.hs) - 1], c.hs[0]
	}
*/
}

// Dynamicall exapnd the data structure by adding a hash table. Called from Insert and friends.
func (c *Cuckoo) addTable(growFactor int) {
	c.Tables++
	c.Size = c.Tables * c.Buckets * c.Slots
	c.MaxElements = int(float64(c.Size) * c.MaxLoadFactor)
	newTable := make([]Buckets, c.Buckets, c.Buckets)
	for b, _ := range newTable {
		if len(newTable[b]) == 0 {
			newTable[b] = makeSlots(newTable[b], c.Slots)
			for s, _ := range newTable[b] {
				newTable[b][s].val = 0
			}
		}
	}
	c.tbs = append(c.tbs, newTable)
	c.addHash()
	// perhaps reset the stats ???
}

// Create a new cuckoo hash.
func New(tables, buckets, slots int, loadFactor float64, hashName string, emptyKey ...Key) *Cuckoo {
	var b Buckets
	var akey Key

	if len(b) > 0 && len(b) != slots {
		fmt.Printf("New: slot mismatch compiled slots=%d, requested slots=%d\n", len(b), slots)
		return nil
	}

	c := &Cuckoo{}
	if buckets < 0 {
		pbuckets := primes.NextPrime(-buckets)
		//fmt.Printf("buckets=%d, pbuckets=%d\n", buckets, pbuckets)
		buckets = pbuckets
	}
	//fmt.Printf("unsafe.Sizeof(akey)=%d\n", unsafe.Sizeof(akey))
	c.b = make([]byte, unsafe.Sizeof(akey), unsafe.Sizeof(akey))
	c.b = c.b[:]
	c.buf = new(bytes.Buffer)
	c.grow = true
	c.Tables, c.Buckets, c.Slots =  tables, buckets, slots
	c.StartLevel, c.LowestLevel = InitialStartLevel, InitialLowestLevel
	c.Size = tables * buckets * slots
	c.MaxLoadFactor = loadFactor
	c.HashName = hashName // "m332"
	c.MaxElements = int(float64(c.Size) * c.MaxLoadFactor)
	if len(emptyKey) > 0 {
		c.emptyKey = emptyKey[0]
	}
	c.ekiz = c.emptyKey == zeroKey
	c.r = rand.Float64
	c.BucketSize = int(unsafe.Sizeof(b))
	c.TableCounters = make([]TableCounters, tables)

	c.seeds = make([]uint32, tables, tables)
	c.seeds = c.seeds[0:0]
	c.hf = make([]hash.Hash32, tables, tables)
	c.hf = c.hf[0:0]
	c.hs = make([]uint32, len(c.hf))
	c.hs = c.hs[0:0]
	c.TableCounters = c.TableCounters[0:0]
	for i := 0; i < tables; i++ {
		c.addHash()
	}
	//fmt.Printf("c.seeds=%#v\n", c.seeds)
	//fmt.Printf("c.hf=%#v\n", c.hf)
	//fmt.Printf("New: c.Config=%#v\n", c.Config)

	// init the table
	c.tbs = make([][]Buckets, tables, tables)
	for t, _ := range c.tbs {
		c.tbs[t] = make([]Buckets, buckets, buckets)
		c.TableCounters[t].Size = buckets * slots
		for b, _ := range c.tbs[t] {
			c.tbs[t][b] = makeSlots(c.tbs[t][b], slots)
			//c.tbs[t][b] = make(Buckets, slots, slots)
			// the following not needed
			for s, _ := range c.tbs[t][b] {
				c.tbs[t][b][s].val = 0
			}
		}
	}
	return c
}

// If the Key is a numeric data type set the length here.
func (c *Cuckoo) SetNumericKeySize(size int) {
	if size != 4 {
		panic("SetNumericKeySize")
	}
	c.NumericKeySize = size
}

// Get the current load factor.
func (c *Cuckoo) GetLoadFactor() float64 {
	return float64(c.Elements) / float64(c.Size)
}

// Set the starting value for level, used by Insert and friends.
func (c *Cuckoo) SetStartLevel(sl int) {
	c.StartLevel = sl
}

// Set the lowest value level call decemnet to.
func (c *Cuckoo) SetLowestLevel(ll int) {
	c.LowestLevel = ll
}

// Set if hash tables can be added dynamically if an insert fails.
func (c *Cuckoo) SetGrow(b bool) {
	c.grow = b
}

// Given key calculate the hash for the specified table
func  (c *Cuckoo) calcHash(hf hash.Hash32, seed uint32, key Key) (h uint32) {
	// speed up some common key cases
	switch c.NumericKeySize {
	case 4:
		c.b[0], c.b[1], c.b[2], c.b[3] = byte(key&0xFF), byte((key>>8)&0xFF), byte((key>>16)&0xFF), byte((key>>24)&0xFF)
	default:
		//b := make([]byte, 4)
		c.buf.Reset()
		err := binary.Write(c.buf, binary.LittleEndian, int32(key))
		if err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		if l, err2 := c.buf.Read(c.b); l != 4 {
			fmt.Printf("l=%d, err=%q\n", l, err2)
			panic("Insert: Read")
		}
	}
	if false {
		hf.Reset()
		hf.Write(c.b)
		h1 := hf.Sum32()
		h = h1 % uint32(c.Buckets)
		//fmt.Printf("c.hs[%d]=0x%x, Sum32(b)=0x%x\n", k, h1, murmur3.Sum32(b, c.seeds[k]))
	} else {
		h = murmur3.Sum32(c.b, seed) % uint32(c.Buckets)
	}
	return
}

// Given key calculate the hash for the specified table
func  (c *Cuckoo) calcHashForTable(t int, key Key) uint32 {
	return c.calcHash(c.hf[t], c.seeds[t], key)
}

/*
func  (c *Cuckoo) calcHashForTable(t int, key Key) {
	c.hs[t] = c.calcHash(c.hf[t], c.seeds[t], key)
}
*/

// Calculate hashes for key for all hash tables. No longer used.
func  (c *Cuckoo) calcHashes(key Key) {
	// calculate hashes for each table
	for k, v := range c.hf {
		c.hs[k] = c.calcHash(v, c.seeds[k], key)
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}
}


// Given key return the value and a "ok" bool indicating success or failure.
func (c *Cuckoo) Lookup(key Key) (Value, bool) {
	c.Lookups++

	if key == c.emptyKey {
		if c.emptyKeyValid {
			return c.emptyValue, true
		} else {
			return zeroVal, false
		}
	}

	//c.calcHashes(key)
	for t, _ := range c.tbs {
		b := c.calcHashForTable(t, key) % uint32(c.Buckets)
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

// Given key delete the bucket. Return the value found and a bool "ok" indicating success
func (c *Cuckoo) Delete(key Key) (bool, Value) {
	c.Deletes++

	//fmt.Printf("key=%v, c.emptyKey=%v\n", key, c.emptyKey)
	if key == c.emptyKey {
		if c.emptyKeyValid {
			c.Elements--
			c.emptyKeyValid = false
			return true, c.emptyValue
		} else {
			//fmt.Printf("Delete: can't find emptyKey %v\n", key)
			return false, zeroVal
		}
	}

	//c.calcHashes(key)
	for t, _ := range c.tbs {
		b := c.calcHashForTable(t, key) % uint32(c.Buckets)
		//b := c.hs[t]
		for s, _ := range c.tbs[t][b] {
			//fmt.Printf("Delete: check key=%d, table=%d, bucket=%d, slot=%d, found key=%d\n", key, t, b, s, c.tbs[t][b][s].key)
			if c.tbs[t][b][s].key == key {
				//fmt.Printf("Delete: found key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", key, c.tbs[t][b][s].val, t, b, s)
				c.tbs[t][b][s].key = c.emptyKey 
				c.TableCounters[t].Elements--
				c.Elements--
				if c.Elements < 0 {
					panic("Delete")
				}
				return true, c.tbs[t][b][s].val
			}
		}
	}
	//fmt.Printf("Delete: can't find %v\n", key)
	return false, zeroVal
}

// Internal version of insert routine.
// Given key, value, and a starting level insert the KV pair. Return ok and level needed to insert.
func (c *Cuckoo) insert(key Key, val Value, ilevel int) (ok bool, level int) {
	var k Key
	var v Value

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
	ins = func(kx Key, vx Value) bool {
		c.calcHashes(kx)
		//fmt.Printf("Insert: level=%d, key=%d, ", level, kx)
		//phv()
		k = kx // was :=
		v = vx // was :=
		for t, _ := range c.tbs {
			//phv()
			b := c.hs[t]
			//fmt.Printf("Insert: next table, level=%d, key=%d, value=%d, table=%d, bucket=%d\n", level, k, v, t, b)
			for s, _ := range c.tbs[t][b] {
				c.Attempts++
				pk := c.tbs[t][b][s].key
				if  pk == c.emptyKey || pk == k  {	// added replacement semantics
					c.tbs[t][b][s].key, c.tbs[t][b][s].val = k, v
					if pk == k {
						//fmt.Printf("Insert: level=%d, pk=%d, key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", level, pk, k, v, t, b, s)
					}
					c.Elements++
					c.TableCounters[t].Elements++
					return true
				}
			}
			// no slots available in this table available, pick a random key to move to the next table
			c.Bumps++
			c.TableCounters[t].Bumps++
			victim := c.rbetween(0, c.Slots-1)
			//fmt.Printf("insert: level=%d, bump value=%d for value=%d, table=%d, bucket=%d, slot=%d\n", level, c.tbs[t][b][victim].val, val, t, b, victim)
			bucket := c.tbs[t][b][victim]
			c.tbs[t][b][victim].key = k
			c.tbs[t][b][victim].val = v
			k = bucket.key
			v = bucket.val
			c.calcHashes(k)
			//fmt.Printf("insert: level=%d, new key=%d, val=%d\n", level, k, v)
		}
		// could not find any space for key in any table moving left to right
		// try again starting with leftmost table
		c.Iterations++
		level--

		// skip 0 because it's used as a signal that Insert failed because of load factor constraint
		if level == 0 {
			level = -1
		}
		if level <= c.LowestLevel {
			fmt.Printf("cukcoo: Insert FAILED, val=%v, key=%v\n", k, v)
			return false
		}
		if level <= 0 {
			c.Aborts++
			// sublte bug, on failure to insert the key not inserted may not be the original key
			// so keep interating until the original key is not found to prevent data loss
			_, found := c.Lookup(key)
			//fmt.Printf("key %d found=%v\n", key, found)
			if !found {
				//fmt.Printf("insert: aborted at level=%d, aborts=%d\n", level, c.Aborts)
				return false
			}
		}
		return ins(k, v)
	}

	// function starts here
	//fmt.Printf("Insert: level=%d, key=%d, value=%d\n", level, key, val)
	k = key
	v = val
	sva, svi := c.Attempts, c.Iterations
	level = ilevel
again:
	if c.Elements >= c.MaxElements {
		fmt.Printf("insert: limited at %v\n", key)
		c.Limited = true
		return false, 0
	}
	if k == c.emptyKey {
		if c.emptyKeyValid {
			panic("emptyKeyValid")
		} else {
			c.Elements++
			c.emptyKeyValid = true
			c.emptyValue = v
		}
		return true, level
	}
	ok = ins(k, v)
	if ok {
		c.Inserts++
	} else {
		if c.grow {
			fmt.Printf("insert: add a table, level=%d, key=%v, val=%v\n", level, k, v)
			c.addTable(0)
			goto again
		}
	}
	if c.Attempts - sva > c.MaxAttempts {
		c.MaxAttempts = c.Attempts - sva
	}
	if c.Iterations - svi > c.MaxIterations {
		c.MaxIterations = c.Iterations - svi
	}
	//fmt.Printf("%d/%d ", c.Attempts - sva, c.Iterations - svi)
	return
}

// Given key, value insert a KV pair and return ok.
func (c *Cuckoo) Insert(key Key, val Value) (ok bool) {
	ok, _ = c.insert(key, val, c.StartLevel)
	return
}

// Given key, value insert a KV pair and return ok and level needed to insert
func (c *Cuckoo) InsertL(key Key, val Value) (ok bool, rlevel int) {
	ok, rlevel = c.insert(key, val, c.StartLevel)
	return
}

/*
func init() {
	var k Key
	var v Value = 1// "foobar"

	fmt.Printf("Key=%T\n", k)
	fmt.Printf("Value=%T\n", v)
}
*/
