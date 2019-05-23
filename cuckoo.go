// Copyright © 2014-2017 Lawrence E. Bakst. All rights reserved.

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
	"hash"
	"leb.io/cuckoo/primes"
	"math"
	"math/rand"
	"unsafe"
)

const (
	aes  = 1
	j264 = iota
	j364 = iota
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
	SlotsSize     int  // size of a single bucket * slots
	Elements      int  // number of elements currently residing in the data structure
	Inserts       int  // number of time insert has been called
	Probes        int  // number of probes to find a free element
	Iterations    int  // number of iterations through all the hash tables in an attemp an insert
	Deletes       int  // number of times delete has been called
	Lookups       int  // number of lookups
	Aborts        int  // number of times an insert had to aborted
	Fails         int  // number of times that insert failed
	Bumps         int  // number of evicted buckets
	TableGrows    int  // number of hash tables added
	TraceCnt      int  // number of trance records out
	MaxPathLen    int  // longest chain of bumps
	MaxProbes     int  // highest number of probes
	MaxIterations int  // highest number of interations
	MinLevel      int  // lowest level achieved
	MinTraceCnt   int  // lowest trace count
	Limited       bool // were inserts limited by a load factor
}

// Per table stats, again all public.
type TableCounters struct {
	Size     int // c.Nbuckets * c.Nslots
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
	Ntables       int     // number of hash tables
	Nbuckets      int     // number of buckets
	Nslots        int     // number of slots
	Size          int     // Size = Tables * Buckets * Slots
	MaxElements   int     // maximum number of elements the data structure can hold
	HashName      string  // name of hashing function used
}

// A Table is a 2 dimensional matrix of buckets, the first index is the bucket number
// and the second index is the slot number
type Table struct {
	buckets       []Slots     // each indexed bucket contains a slice of Bucket, called Slots, defined in kv_array.go or kv_slice.go
	seed          uint64      // seed used per table to make a unique hash function
	hfs           hash.Hash64 // hash function to use, the design allows for different hash functions per table but that is not used
	TableCounters             // per Table stats
}

// The main data structure for cuckoo hash.
// Most fields are private but the counters and config are public.
//indexed Slots defined in kv_array.go or kv_slice.go
type Cuckoo struct {
	tbs           [][]Slots       // a slice  of Tables, each table having a slice of Slots, each slot holding a Bucket
	TableCounters []TableCounters // per table stats
	seeds         []uint64        // seeds used per table
	hfs           []hash.Hash64   // one for each table + the last one reserved for fingerprints
	hs            []uint64        // hash sums for each table and fingerprint, no longer used
	r             uint64          // reciprocal of Buckets
	n             uint64          // Size
	rot           int             // table rotator
	fp            bool            // first pass of table insert
	Config                        // config data
	Counters                      // stats

	hashno int         // hash function
	hf     hash.Hash64 // generic hash function
	hf32   func(data uint32, seed uint64) uint64
	hf64   func(data, seed uint64) uint64
	hfb    func(data []byte, seed uint64) uint64

	//b				[]byte			// used for result of marshalled data
	buf     *buf            // for marshalling data
	encoder *binary.Encoder // encoder for serializing Key
	//rnd				func() float64	// random numbers for eviction
	rnd            *rand.Rand // random numbers used for eviction
	eseed          int64      // seed for evictions
	emptyKey       Key        // empty key
	emptyValue     Value      // if empty key store value lives here and not in a hash table
	emptyKeyValid  bool       // something store here
	ekiz           bool       // empty key is zero
	grow           bool       // are we allowed to add a hash table as needed?
	Trace          bool       // produce a trace on stdout
	NumericKeySize int        // if key is numeric what is size in bytes
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

/*
	BucketSize    int  // size of a single bucket (1 slot) in bytes
	BucketsSize   int  // size of a single bucket * slots
	Elements      int  // number of elements currently residing in the data structure
	Inserts       int  // number of time insert has been called
	Attempts      int  // number of attempts to insert all elements
	Iterations    int  // number of iterations through all the hash tables to attemps an insert
	Deletes       int  // number of times delete has been called
	Lookups       int  // number of lookups
	Aborts        int  // number of times an insert had to aborted
	Fails         int  // number of times that insert failed
	Bumps         int  // number of evicted buckets
	TableGrows    int  // number of hash tables added
	MaxPathLen    int  // longest chain of bumps
	MaxAttempts   int  // highest number of attempts
	MaxIterations int  // highest number of interations
	MinLevel      int  // lowest level achieved
	Limited       bool // were inserts limited by a load factor
*/

func (c *Counters) InitCounters() {
	c.MinLevel = InitialStartLevel
	c.MinTraceCnt = math.MaxInt64
}

func (c *Counters) CountersAdd(add *Counters) {
	var max = func(a, b int) int {
		if a < b {
			return b
		}
		return a
	}
	var min = func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	c.Elements += add.Elements
	c.Inserts += add.Inserts
	c.Probes += add.Probes
	c.Iterations += add.Iterations
	c.Deletes += add.Deletes
	c.Lookups += add.Lookups
	c.Aborts += add.Aborts
	c.Fails += add.Fails
	c.Bumps += add.Bumps
	c.TableGrows += add.TableGrows
	//tot.BucketSize = add.BucketSize
	//tot.BucketsSize = add.BucketsSize
	c.MaxPathLen = max(c.MaxPathLen, add.MaxPathLen)
	c.MaxProbes = max(c.MaxProbes, add.MaxProbes)
	c.MaxIterations = max(c.MaxIterations, add.MaxIterations)
	c.MinLevel = min(c.MinLevel, add.MinLevel)
	c.MinTraceCnt = min(c.MinTraceCnt, add.TraceCnt)
	if add.Limited {
		c.Limited = true
	}
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
	//rf := c.rnd()
	rf := c.rnd.Float64()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

// Dynamicall exapnd the data structure by adding a hash table. Called from Insert and friends.
func (c *Cuckoo) addTable(growFactor float64) {
	//fmt.Printf("table: %d\n", c.Ntables)
	c.Ntables++
	buckets := c.Nbuckets
	slots := c.Nslots
	c.Size += buckets * slots
	c.MaxElements = int(float64(c.Size) * c.MaxLoadFactor)
	newTable := make([]Slots, buckets, buckets)
	for b, _ := range newTable {
		if len(newTable[b]) == 0 {
			newTable[b] = makeSlots(newTable[b], slots)
			for s, _ := range newTable[b] {
				newTable[b][s].val = c.emptyValue // ???
			}
		}
	}
	c.tbs = append(c.tbs, newTable)
	c.seeds = append(c.seeds, uint64(len(c.seeds)+1))
	//fmt.Printf("c.seeds=%v\n", c.seeds)
	c.hfs = append(c.hfs, c.getHash(c.HashName, uint64(c.seeds[len(c.seeds)-1])))
	//fmt.Printf("c.hfs=%v\n", c.seeds)
	c.hs = append(c.hs, 0)
	c.TableCounters = append(c.TableCounters, TableCounters{Size: c.Nbuckets * c.Nslots})
	// perhaps reset the stats ???
}

// Create a new cuckoo hash table of size  = tables * buckets * slots.
// If buckets is negative, the next prime number greater than abs(buckets) is automatically generated,
// You can pass an eseed to seed the random number generator used to select a bucket for eviction.
// Don't allow more than size * loadFactor elements to be stored.
// Therefore, a loadFactor of 1.0 means the hash table can be completely full.
// Use a lower loadFactor to reduce the amount of CPU time used for Inserts when the table gets full.
// Use hashName to specify which hash function to use.
// Currently the only valid hashName strings are "j364" and "aes".
// Only use "aes" on Intel 64 bit machines with the AES instructions.
// If specified, use emptyKey as the key that signifies that an element is unused.
// However, often the default, the Go zero initialization suffices as the emptyKey.
func New(tables, buckets, slots int, eseed int64, loadFactor float64, hashName string, emptyKey ...Key) *Cuckoo {
	var s Slots
	var b Bucket

	//fmt.Printf("New: tables=%d, buckets=%d, slots=%d, loadFactor=%f, hashName=%q\n", tables, buckets, slots, loadFactor, hashName)
	if len(s) > 0 && len(s) != slots {
		fmt.Printf("New: slot mismatch compiled slots=%d, requested slots=%d\n", len(s), slots)
		return nil
	}

	if buckets < 0 {
		pbuckets := primes.NextPrime(-buckets)
		//fmt.Printf("buckets=%d, pbuckets=%d\n", buckets, pbuckets)
		buckets = pbuckets
	}

	if tables < 1 || buckets < 1 || slots < 1 || loadFactor < 0.0 || loadFactor > 1.0 {
		fmt.Printf("New: tables=%d, buckets=%d, slots=%d, loadFactor=%f, hashName=%q\n", tables, buckets, slots, loadFactor, hashName)
		return nil
	}

	//fmt.Printf("New: tables=%d, buckets=%d, slots=%d, loadFactor=%f, hashName=%q\n", tables, buckets, slots, loadFactor, hashName)
	c := &Cuckoo{}

	h, err := c.setHash(hashName)
	if err != nil {
		return nil
	}
	c.hashno = h
	c.HashName = hashName

	//fmt.Printf("unsafe.Sizeof(akey)=%d\n", unsafe.Sizeof(akey))
	/*
		c.b = make([]byte, unsafe.Sizeof(akey), unsafe.Sizeof(akey))
		c.b = c.b[:]
	*/

	c.Nbuckets, c.Nslots = buckets, slots
	c.buf = newBuf(2048)
	c.encoder = binary.NewEncoder(c.buf)
	c.grow = true
	c.n = uint64(buckets)
	c.r = uint64(4294967296) / c.n // reciprocal of buckets
	c.StartLevel, c.LowestLevel = InitialStartLevel, InitialLowestLevel
	c.MaxLoadFactor = loadFactor
	if len(emptyKey) > 0 {
		c.emptyKey = emptyKey[0]
	}
	c.ekiz = c.emptyKey == zeroKey
	//c.rnd = rand.Float64

	c.eseed = int64(eseed)
	src := rand.NewSource(int64(c.eseed))
	r := rand.New(src)
	c.rnd = r

	c.BucketSize = int(unsafe.Sizeof(b))
	c.SlotsSize = int(unsafe.Sizeof(s))
	//fmt.Printf("c.seeds=%#v\n", c.seeds)

	//fmt.Printf("c.seeds=%#v\n", c.seeds)
	//fmt.Printf("c.hfs=%#v\n", c.hfs)
	//fmt.Printf("c.hf=%#v\n", c.hf)
	//fmt.Printf("New: c.Config=%#v\n", c.Config)

	c.tbs = [][]Slots{}
	c.TableCounters = []TableCounters{}
	c.seeds = []uint64{}
	c.hfs = []hash.Hash64{}
	c.hs = []uint64{}
	for i := 0; i < tables; i++ {
		c.addTable(1.0)
	}
	//fmt.Printf("c=%#v\n", c)
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

// Given key calculate the hash for the specified table
func (c *Cuckoo) calcHashForTable(t int, key Key) uint64 {
	return c.calcHash(c.hfs[t], c.seeds[t], key)
}

// end inlined functions

/*
func  (c *Cuckoo) calcHashForTable(t int, key Key) {
	c.hs[t] = c.calcHash(c.hf[t], c.seeds[t], key)
}
*/

/*
func (c *Cuckoo) lowHash(hash int64) {
	switch c.sectors {
	case 1:
		return 0
	case 2:
		return hash & 1
	case 4:
		return hash & 3
	case 8:
		return hash & 7
	case 16:
		return hash & 15
	}
}
*/

// Calculate hashes for key for all hash tables. No longer used.
func (c *Cuckoo) calcHashes(key Key) {
	// calculate hashes for each table
	for k, v := range c.hfs {
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
		//ba := ha % uint32(c.Nbuckets)

		// this was an experiment to see if pre-calculating the reciprocal would be faster than MOD
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
		b := h % uint64(c.Nbuckets)

		/*
			bb := uint32(b)
			if ba != bb {
				fmt.Printf("ba=%d, bb=%d\n", ba, bb)
			}
		*/

		for s, _ := range c.tbs[t][b] {
			//fmt.Printf("Lookup: key=%d, table=%d, bucket=%d, slot=%d, found key=%d\n", key, t, b, s, c.tbs[t][b][s].key)
			if c.tbs[t][b][s].key == key {
				//fmt.Printf("Lookup: table=%d, bucket=%d, slot=%d, key=%d, value=%d\n", t, b, s, key, c.tbs[t][b][s].val)
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
		b := c.calcHashForTable(t, key) % uint64(c.Nbuckets)
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

var calls int

// Internal version of insert routine.
// Given key, value, and a starting level insert the KV pair. Return ok and level needed to insert.
// If level 0 is returned it means the insert failed
func (c *Cuckoo) insert(key Key, val Value, ilevel int) (ok bool, level int) {
	var k Key
	var v Value
	var bumps int
	var depth int

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
		var sk Key
		var sv Value
		var pk Key
		//c.calcHashes(kx)
		//fmt.Printf("Insert: level=%d, key=%d, ", level, kx)
		//phv()
		depth++
		k = kx // was :=
		v = vx // was :=
		// we used to move left to right, with the chance of an insert increasing as
		// we move because the tables filled up left to right.
		// Now we rotate the starting point. Why has no one done this before.
		t := c.rot
		for _, _ = range c.tbs {
			// rotate which table we start inserts with
			//ha := c.calcHashForTable(t, k)
			//ba := ha % uint32(c.Nbuckets)
			/*
				b := h - ((c.r * h) >> 32) * c.n
				if b > c.n {
					b -= c.n
				}
			*/
			//h := uint64(v)
			//ui32tob(c.buf.b, k)
			//ui32tob(c.buf.b, k)
			//h := uint64(murmur3.Sum32(c.buf.b, uint32(c.seeds[t])))
			//h := uint64(jenkins3.Sum32(c.buf.b, uint32(c.seeds[t])))
			//h := aeshash.Hash64(uint64(k), c.seeds[t])
			h := uint64(c.calcHashForTable(t, k))
			//fmt.Printf("h1=%#x, h=%#x\n", h1, h)
			//h := uint64(k)
			b := h % uint64(c.Nbuckets)

			//fmt.Printf("Insert: next table, h=%#x, level=%d, table=%d, bucket=%d, key=%d, value=%d\n", h, level, t, b, k, v)
			// check all the slots in the current table and see if we can insert
			//s := lowHash(h, )
			//for {
			for s, _ := range c.tbs[t][b] {
				c.Probes++
				pk = c.tbs[t][b][s].key // avoid previous allocation
				c.TraceCnt++
				if c.Trace {
					fmt.Printf("{%q: %d, %q: %d, %q: %q, %q: %d, %q: %d, %q: %d, %q: %v, %q: %v},\n",
						"i", c.TraceCnt, "l", level, "op", "P", "t", t, "b", b, "s", s, "k", k, "v", v)
				}
				if pk == c.emptyKey || pk == k { // added replacement semantics
					c.tbs[t][b][s].key, c.tbs[t][b][s].val = k, v
					c.TraceCnt++
					if c.Trace {
						fmt.Printf("{%q: %d, %q: %d, %q: %q, %q: %d, %q: %d, %q: %d, %q: %v, %q: %v},\n",
							"i", c.TraceCnt, "l", level, "op", "I", "t", t, "b", b, "s", s, "k", k, "v", v)
					}
					if pk == c.emptyKey || pk == k {
						//fmt.Printf("Insert: h=%#x, level=%d, table=%d, bucket=%d, slot=%d, pk=%d, key=%d, value=%d\n", h, level, t, b, s, pk, k, v)
					}
					c.Elements++
					c.TableCounters[t].Elements++
					return true
				}
			}
			// unproven and untested optimization below XXX
			// if first insert attempt and no slots in this table and more than 2 tables, try the next table
			if depth == 0 && len(c.tbs) > 2 {
				continue
			}
			// No slots available in this table available, evict a random KV pair and store the current KV where it was.
			// move to the next table and different bucket and hope it works out better.
			bumps++
			c.Bumps++
			c.TableCounters[t].Bumps++
			victim := c.rbetween(0, c.Nslots-1)
			//fmt.Printf("insert: level=%d, bump value=%d for value=%d, table=%d, bucket=%d, slot=%d\n", level, c.tbs[t][b][victim].val, val, t, b, victim)
			sk, sv = c.tbs[t][b][victim].key, c.tbs[t][b][victim].val // avoid previous stack allocation
			c.TraceCnt++
			if c.Trace {
				fmt.Printf("{%q: %d, %q: %d, %q: %q, %q: %d, %q: %d, %q: %d, %q: %v, %q: %v},\n",
					"i", c.TraceCnt, "l", level, "op", "E", "t", t, "b", b, "s", victim, "k", sk, "v", v)
			}
			c.tbs[t][b][victim].key = k
			c.tbs[t][b][victim].val = v
			c.TraceCnt++
			if c.Trace {
				fmt.Printf("{%q: %d, %q: %d, %q: %q, %q: %d, %q: %d, %q: %d, %q: %v, %q: %v},\n",
					"i", c.TraceCnt, "l", level, "op", "I", "t", t, "b", b, "s", victim, "k", k, "v", v)
			}
			k = sk
			v = sv
			//c.calcHashes(k) ??? XXX ???
			//fmt.Printf("insert: level=%d, new key=%d, val=%d\n", level, k, v)
			t++
			if t > len(c.tbs)-1 {
				t = 0
			}
		}
		// Could not find any space for key in any table. Since the key has now changed,
		// we try again with what will probably be different buckets hoping for a place.
		c.Iterations++
		level--

		// If we reach level 0 we have failed to insert after InitialStartLevel interations, each examining
		// t hash tables with s slots each. We don't stop because they current key
		// is probaly not be the key we started with, so we keep going, hoping to finally get the original
		// key back, to avoid data loss.
		// It can also happen that in the process of doing this the key ends up being inserted because
		// the loop and logic is identical except we stop trying if the key is inserted OR we get the original
		// key back.
		// We skip 0 because it's used as a return value that Insert failed because of load factor constraint.
		// We call this an abort. An abort does not imply the KV failed to insert.
		if level == 0 {
			//fmt.Printf("insert: begin abort key=%d, val=%d, calls=%d, depth=%d, c.Iterations=%d\n", key, val, calls, depth, c.Iterations)
			c.Aborts++ // stop trying to insert and recover displaced data
			level = -1
		}
		// At this point we have failed to recover the original KV after abs(InitialLowestLevel) more interations.
		// This means that the insert failed AND a random KV was also deleted from the Cuckoo table.
		// Give up, we call this a "fail"
		if level <= c.LowestLevel {
			c.Fails++
			fmt.Printf("cukcoo: Insert FAILED, val=%v, key=%v\n", k, v)
			return false
		}
		if level <= 0 {
			// NB: fine point, on insert failure, the key NOT inserted may NOT be the original key.
			// Keep interating until the original key is not found to prevent random data loss
			// So level less than 0 means had to work to get a displaced key back into the hash table
			_, found := c.Lookup(key)
			//fmt.Printf("key %d found=%v\n", key, found)
			// if we can't find the key that was passed in then it is safe to stop because there will
			// be no data loss. If we can find they key that was passed in, then some other key
			// has been displaced.
			// This is an interesting case that I had never seen before. Insert fails and a random
			// piece of data that was previusly inserted has been lost. Luckily the fix is pretty easy.
			if !found {
				fmt.Printf("insert: aborted at key=%d, value=%d, calls=%d, depth=%d, level=%d, aborts=%d\n", key, val, calls, depth, level, c.Aborts)
				return false
			}
		}
		// ??? consider bumping c.rot here as opposed to below
		return ins(k, v) // try to insert again, tail recursively
	}

	// insert starts here
	//fmt.Printf("Insert: level=%d, key=%d, value=%d\n", level, key, val)
	calls++
	k = key
	v = val
	sva, svi := c.Probes, c.Iterations
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
			c.TableGrows++
			//c.Ntables++
			c.addTable(0)
			goto again
		}
	}
	if c.Probes-sva > c.MaxProbes {
		c.MaxProbes = c.Probes - sva
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
	c.rot++
	c.rot %= c.Ntables
	//fmt.Printf("c.rot=%d, c.Ntables=%d\n", c.rot, c.Ntables)
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
