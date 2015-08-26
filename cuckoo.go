// Copyright © 2014 Lawrence E. Bakst. All rights reserved.

// Package cuckoo implements a cuckoo hash table.
// With the correct options this data structure can achieve 5X more storage efficiency
// over Go's builtin map with similar performance. See the "README.md" file for all the details.
// Edit the file "kv_default.go" to define the types for you Key and Value.
package cuckoo

import (
	_ "bytes"
	_ "encoding/binary"
	"fmt"
	"github.com/alecthomas/binary"
	_ "github.com/tildeleb/cuckoo/jenkins264"
	_ "github.com/tildeleb/cuckoo/jk3"
	_ "github.com/tildeleb/cuckoo/murmur3"
	"github.com/tildeleb/cuckoo/primes"
	"github.com/tildeleb/hashland/aeshash" // no longer self contained package
	"hash"
	_ "math"
	"math/rand"
	"unsafe"
)

type Container interface {
	Lookup(key Key) (Value, bool)
	Delete(key Key) (Value, bool)
	Insert(key Key, val Value) (ok bool)
	Map(iter func(key Key, val Value) (stop bool))
}

var zeroKey Key
var zeroVal Value

// For historical reasons this is called a Bucket but should really be called an element
type Bucket struct {
	key Key
	val Value
}

// Counters. All public but we now have an API to access them.
type Counters struct {
	BucketSize    int  // size of a single bucket (1 slot) in bytes
	BucketsSize   int  // size of a single bucket * slots
	Elements      int  // number of elements currently residing in the data structure
	Inserts       int  // number of time insert has been called
	Attempts      int  // number of attempts to insert all elements
	Iterations    int  // number of iterations through all the hash tables to attemps an insert
	Deletes       int  // number of times delete has been called
	Lookups       int  // number of lookups
	Fails         int  // number of times that insert failed
	Bumps         int  // number of evicted buckets
	MaxPathLen    int  // longest chain of bumps
	Aborts        int  // number of times an insert had to aborted
	MaxAttempts   int  // highest number of attempts
	MaxIterations int  // highest number of interations
	MinLevel      int  // lowest level achieved
	Limited       bool // were inserts limited by a load factor
}

// Per table stats, again all public.
type TableCounters struct {
	Size     int // c.Buckets * c.Slots
	Elements int // number of elements currently residing in this hash table
	Bumps    int // number of evicted buckets
}

// These two constants seem to work well for many cases, but not all
const (
	InitialStartLevel  = 2000
	InitialLowestLevel = -8000
)

// Configuration info for the cucko hash is collected in this structure.
// All fields are exported/public.
type Config struct {
	MaxLoadFactor float64 // don't allow more than MaxElements = Tables * Buckets * Slots elements
	StartLevel    int     // starting value for level which is decremented for each insertion attempt
	LowestLevel   int     // This is usually a negative number and defines how far level can be decremented
	Tables        int     // number of hash tables
	Buckets       int     // number of buckets
	Slots         int     // number of slots
	Size          int     // Size = Tables * Buckets * Slots
	MaxElements   int     // maximum number of elements the data structure can hold
	HashName      string  // name of hashing function used
}

// The main data structure for cuckoo hash.
// Most fields are private but the counters are public.
type Cuckoo struct {
	tbs           [][]Buckets     // indexed defineBuckets defined in kv_array.go or kv_slice.go
	r             uint64          // reciprocal of Buckets
	n             uint64          // Size
	Config                        // config data
	Counters                      // stats
	TableCounters []TableCounters // per table stats
	hashno        int             // hash function
	seeds         []uint64        // seeds used per table
	hf            []hash.Hash64   // one for each table + the last one reserved for fingerprints
	hs            []uint64        // hash sums for each table and fingerprint
	//b				[]byte			// used for result of marshalled data
	buf            *buf            // for marshalling data
	encoder        *binary.Encoder // encoder for serializing Key
	rnd            func() float64  // random numbers for eviction
	eseed          int64
	emptyKey       Key   // empty key
	emptyValue     Value // if empty key store value lives here and not in a hash table
	emptyKeyValid  bool  // something store here
	ekiz           bool  // empty key is zero
	grow           bool  // are we allowed to add a hash table as needed?
	NumericKeySize int   // if key is numeric what is size in bytes
}

// Simple struct and a couple of methods that satisfy the io.Writer interface.
// buf saves the data in a slice that can be accessed without a copy.
// Used to serialize the key.
type buf struct {
	b    []byte
	base [4096]byte
	i    int
}

func (b *buf) Reset() {
	b.i = 0
}

func newBuf(size int) (b *buf) {
	buf := buf{}
	//buf.base = make([]byte, size, size)
	//buf.base = buf.base[0:0] // makes printing buf cleaner
	return &buf
}

// capture io.Writer data in a slice
func (b *buf) Write(p []byte) (n int, err error) {
	b.b = b.base[b.i : b.i+len(p)]
	copy(b.b, p)
	b.i += len(p)
	//fmt.Printf("Write: len(b.b)=%d, len(p)=%d, % #X\n", len(b.b), len(p), p)
	//fmt.Printf("b=%#v\n", b)
	return len(p), nil
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
	case "MaxPathLen":
		return c.MaxPathLen
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
	rf := c.rnd()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

// Add a hash function to a slice of hash functions.
func (c *Cuckoo) addHash() {
	c.seeds = append(c.seeds, uint64(len(c.seeds)+1))
	c.hf = append(c.hf, getHash(c.HashName, uint64(c.seeds[len(c.seeds)-1])))
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

// Create a new cuckoo hash of size  = tables * buckets * slots.
// Don't allow more than size * loadFactor elements to be stored.
// Use hashName as the hash function.
// If specified, use emptyKey as the key that signifies that an element is unused.
// However, usually the default, the Go zero initization, is what you want.
func New(tables, buckets, slots int, loadFactor float64, hashName string, emptyKey ...Key) *Cuckoo {
	var bs Buckets
	var b Bucket
	//var akey Key

	if len(bs) > 0 && len(bs) != slots {
		fmt.Printf("New: slot mismatch compiled slots=%d, requested slots=%d\n", len(bs), slots)
		return nil
	}

	c := &Cuckoo{}
	if buckets < 0 {
		pbuckets := primes.NextPrime(-buckets)
		//fmt.Printf("buckets=%d, pbuckets=%d\n", buckets, pbuckets)
		buckets = pbuckets
	}
	//fmt.Printf("unsafe.Sizeof(akey)=%d\n", unsafe.Sizeof(akey))
	/*
		c.b = make([]byte, unsafe.Sizeof(akey), unsafe.Sizeof(akey))
		c.b = c.b[:]
	*/
	c.hashno = setHash(hashName)
	c.buf = newBuf(2048)
	c.encoder = binary.NewEncoder(c.buf)
	c.grow = true
	c.Tables, c.Buckets, c.Slots = tables, buckets, slots
	c.n = uint64(buckets)
	c.r = uint64(4294967296) / c.n // reciprocal of buckets
	c.StartLevel, c.LowestLevel = InitialStartLevel, InitialLowestLevel
	c.MinLevel = c.StartLevel
	c.Size = tables * buckets * slots
	c.MaxLoadFactor = loadFactor
	c.HashName = hashName // "m332"
	c.MaxElements = int(float64(c.Size) * c.MaxLoadFactor)
	if len(emptyKey) > 0 {
		c.emptyKey = emptyKey[0]
	}
	c.ekiz = c.emptyKey == zeroKey
	c.rnd = rand.Float64
	c.BucketSize = int(unsafe.Sizeof(b))
	c.BucketsSize = int(unsafe.Sizeof(bs))
	c.TableCounters = make([]TableCounters, tables)

	c.seeds = make([]uint64, tables, tables)
	c.seeds = c.seeds[0:0]
	c.hf = make([]hash.Hash64, tables, tables)
	c.hf = c.hf[0:0]
	c.hs = make([]uint64, len(c.hf))
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
	switch size {
	case 4:
		c.buf.b = c.buf.base[0:4]
	case 8:
		c.buf.b = c.buf.base[0:8]
	default:
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

// Set if hash tables can be added dynamically if an insert fails.
func (c *Cuckoo) SetEvictionSeed(seed int64) {
	c.eseed = seed
	rand.Seed(seed)
}

/*
   seed := int64(0)
   // fixed pattern or different values each time
   if *ranf {
       seed = time.Now().UTC().UnixNano()
   } else {
       seed = int64(0)
   }
   rand.Seed(seed)
*/

func (c *Cuckoo) _calcHash(hf hash.Hash64, seed uint64, key Key) (h uint64) {
	if c.NumericKeySize == 4 && c.hashno == aes {
		ui32tob(c.buf.b, key)
		h = aeshash.Hash32(uint32(key), seed)
		return
	}
	// ok we have to copy the key now as all the other hash functions want a slice of bytes.
	switch c.NumericKeySize {
	case 4:
		c.buf.b = c.buf.base[0:4]
		c.buf.b[0], c.buf.b[1], c.buf.b[2], c.buf.b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
	case 8:
		c.buf.b = c.buf.base[0:8]
		c.buf.b[0], c.buf.b[1], c.buf.b[2], c.buf.b[3], c.buf.b[4], c.buf.b[5], c.buf.b[6], c.buf.b[7] =
			byte(key), byte(key>>8), byte(key>>16), byte(key>>24), byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
	default:
		c.buf.Reset()
		if err := c.encoder.Encode(&key); err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		c.buf.b = c.buf.base[0:c.buf.i]
	}
	if c.HashName == "" {
		panic("hash interface")
		hf.Reset()
		hf.Write(c.buf.b)
		h1 := uint64(hf.Sum64())
		h = h1 % uint64(c.Buckets)
		//fmt.Printf("c.hs[%d]=0x%x, Sum32(b)=0x%x\n", k, h1, murmur3.Sum32(b, c.seeds[k]))
	} else {
		//if c.HashName == "aes" {
		h = aeshash.Hash(c.buf.b, seed) % uint64(c.Buckets)
		//} else {
		//h = jenkins264.Hash(c.buf.b, seed) % uint64(c.Buckets)
		//}
		/*
				h = uint64(murmur3.Sum32(c.b, uint32(seed))) % uint64(c.Buckets)
			case "j332":
				h = jenkins3.Sum32(c.b, seed) % uint32(c.Buckets)
		*/
	}
	return
}

// the following 5 functions/methods can be inlined.
func ui32tob(b []byte, key Key) {
	b[0], b[1], b[2], b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
}

func ui64tob(b []byte, key Key) {
	b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24), byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
}

func ui64tob1(b []byte, key Key) {
	b[0], b[1], b[2], b[3] = byte(key), byte(key>>8), byte(key>>16), byte(key>>24)
}

func ui64tob2(b []byte, key Key) {
	b[4], b[5], b[6], b[7] = byte(key>>32), byte(key>>40), byte(key>>48), byte(key>>56)
}

// Given a key and a hash function to use, calculate the hash for the specified table.
// To do this we have to serialize the key
// To get this to inline the optimization for NumericKeySize == 4 was moved to _calcHash
func (c *Cuckoo) calcHash(hf hash.Hash64, seed uint64, key Key) uint64 {
	// speed up a common key case
	if c.NumericKeySize == 8 && c.hashno == aes {
		//fmt.Printf("8")
		//ui64tob1(c.buf.b, key)
		//ui64tob2(c.buf.b, key)
		return aeshash.Hash64(uint64(key), seed)
	}
	//fmt.Printf("%d ", c.NumericKeySize)
	return c._calcHash(hf, seed, key)
}

// Given key calculate the hash for the specified table
func (c *Cuckoo) calcHashForTable(t int, key Key) uint64 {
	return c.calcHash(c.hf[t], c.seeds[t], key)
}

// end inlined functions

/*
func  (c *Cuckoo) calcHashForTable(t int, key Key) {
	c.hs[t] = c.calcHash(c.hf[t], c.seeds[t], key)
}
*/

// Calculate hashes for key for all hash tables. No longer used.
func (c *Cuckoo) calcHashes(key Key) {
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
		//ha := c.calcHashForTable(t, key)
		//ba := ha % uint32(c.Buckets)

		// this was a test to see if pre-calculating the reciprocal would be faster than MOD
		// it is by 10% for L1 fit and 3% for L2 fit, however switching to assembly might make it better than that.
		// L1 fit 11.345 total vs 12.113 total
		// L2 fit 1:00.74 total vs 1:02.49 total

		/*
			b := h - ((c.r * h) >> 32) * c.n
			if b > c.n {
				b -= c.n
			}
		*/
		h := uint64(c.calcHashForTable(t, key))
		b := h % uint64(c.Buckets)

		/*
			bb := uint32(b)
			if ba != bb {
				fmt.Printf("ba=%d, bb=%d\n", ba, bb)
			}
		*/

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
func (c *Cuckoo) Delete(key Key) (Value, bool) {
	c.Deletes++

	//fmt.Printf("key=%v, c.emptyKey=%v\n", key, c.emptyKey)
	if key == c.emptyKey {
		if c.emptyKeyValid {
			c.Elements--
			c.emptyKeyValid = false
			return c.emptyValue, true
		} else {
			//fmt.Printf("Delete: can't find emptyKey %v\n", key)
			return zeroVal, false
		}
	}

	//c.calcHashes(key)
	for t, _ := range c.tbs {
		b := c.calcHashForTable(t, key) % uint64(c.Buckets)
		//b := c.hs[t]
		/*
			h := uint64(c.calcHashForTable(t, key))
			b := h - ((c.r * h) >> 32) * c.n
			if b > c.n {
				b -= c.n
			}
		*/
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
				return c.tbs[t][b][s].val, true
			}
		}
	}
	//fmt.Printf("Delete: can't find %v\n", key)
	return zeroVal, false
}

// Internal version of insert routine.
// Given key, value, and a starting level insert the KV pair. Return ok and level needed to insert.
func (c *Cuckoo) insert(key Key, val Value, ilevel int) (ok bool, level int) {
	var k Key
	var v Value
	var bumps int

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
		//c.calcHashes(kx)
		//fmt.Printf("Insert: level=%d, key=%d, ", level, kx)
		//phv()
		k = kx // was :=
		v = vx // was :=
		for t, _ := range c.tbs {
			//phv()
			//b := c.hs[t]

			//ha := c.calcHashForTable(t, k)
			//ba := ha % uint32(c.Buckets)
			/*
				b := h - ((c.r * h) >> 32) * c.n
				if b > c.n {
					b -= c.n
				}
			*/

			//h := uint64(v)

			//ui32tob(c.buf.b, k)
			//h := uint64(jenkins3.Sum32(c.buf.b, uint32(c.seeds[t])))

			//ui32tob(c.buf.b, k)
			//h := uint64(murmur3.Sum32(c.buf.b, uint32(c.seeds[t])))
			//h := aeshash.Hash64(uint64(k), c.seeds[t])
			h := uint64(c.calcHashForTable(t, k))
			//fmt.Printf("h1=%#x, h=%#x\n", h1, h)
			//h := uint64(k)
			b := h % uint64(c.Buckets)

			//fmt.Printf("Insert: next table, h=%#x, level=%d, table=%d, bucket=%d, key=%d, value=%d\n", h, level, t, b, k, v)
			for s, _ := range c.tbs[t][b] {
				c.Attempts++
				pk := c.tbs[t][b][s].key
				if pk == c.emptyKey || pk == k { // added replacement semantics
					c.tbs[t][b][s].key, c.tbs[t][b][s].val = k, v
					if pk == c.emptyKey || pk == k {
						//fmt.Printf("Insert: h=%#x, level=%d, table=%d, bucket=%d, slot=%d, pk=%d, key=%d, value=%d\n", h, level, t, b, s, pk, k, v)
					}
					c.Elements++
					c.TableCounters[t].Elements++
					return true
				}
			}
			// no slots available in this table available, pick a random key to move to the next table
			bumps++
			c.Bumps++
			c.TableCounters[t].Bumps++
			victim := c.rbetween(0, c.Slots-1)
			//fmt.Printf("insert: level=%d, bump value=%d for value=%d, table=%d, bucket=%d, slot=%d\n", level, c.tbs[t][b][victim].val, val, t, b, victim)
			bucket := c.tbs[t][b][victim]
			c.tbs[t][b][victim].key = k
			c.tbs[t][b][victim].val = v
			k = bucket.key
			v = bucket.val
			//c.calcHashes(k) ??? XXX ???
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
			// fine point, on failure to insert the key not inserted may not be the original key
			// so keep interating until the original key is not found to prevent random data loss
			_, found := c.Lookup(key)
			//fmt.Printf("key %d found=%v\n", key, found)
			if !found {
				//fmt.Printf("insert: aborted at level=%d, aborts=%d\n", level, c.Aborts)
				return false
			}
		}
		return ins(k, v) // try to insert again, tail recursively
	}

	// function starts here
	//fmt.Printf("Insert: level=%d, key=%d, value=%d\n", level, key, val)
	k = key
	v = val
	sva, svi := c.Attempts, c.Iterations
	level = ilevel
again:
	if c.Elements >= c.MaxElements {
		//fmt.Printf("insert: limited at %v\n", key)
		c.Limited = true
		return false, 0
	}
	if k == c.emptyKey {
		if c.emptyKeyValid {
			panic("emptyKeyValid")
		} else {
			c.Inserts++
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
	if c.Attempts-sva > c.MaxAttempts {
		c.MaxAttempts = c.Attempts - sva
	}
	if c.Iterations-svi > c.MaxIterations {
		c.MaxIterations = c.Iterations - svi
	}
	if level < c.MinLevel {
		c.MinLevel = level
	}
	if bumps > c.MaxPathLen {
		c.MaxPathLen = bumps
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

func (c *Cuckoo) Map(iter func(c *Cuckoo, key Key, val Value) (stop bool)) {
	if c.emptyKeyValid {
		iter(c, c.emptyKey, c.emptyValue)
	}

	for _, vt := range c.tbs {
		for _, vb := range vt {
			for _, vs := range vb {
				if vs.key != c.emptyKey {
					if iter(c, vs.key, vs.val) {
						return
					}
				}
			}
		}
	}
}

// doesn't print the value if c.emptyKeyValid is true
func (c *Cuckoo) Print() {
	for t, vt := range c.tbs {
		for b, vb := range vt {
			fmt.Printf("[%d][%d]: ", t, b)
			cnt := 0
			for _, vs := range vb {
				if vs.key != c.emptyKey {
					cnt++
				}
			}
			fmt.Printf("%d\n", cnt)
		}
	}
}

/*
func init() {
	var k Key
	var v Value = 1// "foobar"

	fmt.Printf("Key=%T\n", k)
	fmt.Printf("Value=%T\n", v)
}
*/
