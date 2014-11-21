// Copyright © 2014 Lawrence E. Bakst. All rights reserved.
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

var grow bool = true
var normal bool = false

var zeroKey	Key
var zeroVal Value

type Bucket struct {
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
	MaxAttempts		int
	MaxIterations	int
	Limited			bool
}

// Per table stats
type TableStat struct {
	Size		int
	Elements	int
	Bumps		int
}

// These two constants work well for many cases but not all
const InitialStartLevel = 2000
const InitialLowestLevel = -8000
type Config struct {
	MaxLoadFactor	float64
	StartLevel		int
	LowestLevel		int
	Tables			int
	Buckets			int
	Slots			int
	Size			int
	MaxElements		int
	HashName		string
}

type Cuckoo struct {
	tbs				[][]Buckets		// alignment lives here, your data stored here.
	Config							// config data
	CuckooStat						// stats
	TableStats		[]TableStat		// per table stats
	seeds			[]uint32		// seeds used per table
	hf				[]hash.Hash32	// one for each table + the last one reserved for fingerprints
	hs				[]uint32		// hash sums for each table and fingerprint
	b				[]byte
	//stash			[]bucket		// store keys that fail insert
	r				func() float64	// random numbers for eviction
	emptyKey		Key				// empty key
	emptyValue		Value			// if empty key store value lives here and not in a hash table
	emptyKeyValid	bool			// something store here
	ekiz			bool			// empty key is zero
}

func (c *Cuckoo) GetStat(s string) int {
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
		panic("GetStat")
	}
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

func getHash(hashName string, seed int) hash.Hash32 {
	switch hashName {
	case "m332":
		return murmur3.New(uint32(seed))
	default:
		s := fmt.Sprintf("cuckoo: unknown hash function %q\n", hashName)
		panic(s)
	}
}

func (c *Cuckoo) addHash() {
	c.seeds = append(c.seeds, uint32(len(c.seeds) + 1))
	c.hf = append(c.hf, getHash(c.HashName, int(c.seeds[len(c.seeds) - 1])))
	c.hs = append(c.hs, 0)
	c.TableStats = append(c.TableStats, TableStat{Size: c.Buckets * c.Slots})
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


func New(tables, buckets, slots int, loadFactor float64, hashName string, emptyKey ...Key) *Cuckoo {
	var b Buckets

	if len(b) != slots {
		return nil
	}

	c := &Cuckoo{}
	if buckets < 0 {
		pbuckets := primes.NextPrime(-buckets)
		//fmt.Printf("buckets=%d, pbuckets=%d\n", buckets, pbuckets)
		buckets = pbuckets
	}
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
	c.TableStats = make([]TableStat, tables)

	c.b = make([]byte, 4) // ???

	c.seeds = make([]uint32, tables, tables)
	c.seeds = c.seeds[0:0]
	c.hf = make([]hash.Hash32, tables, tables)
	c.hf = c.hf[0:0]
	c.hs = make([]uint32, len(c.hf))
	c.hs = c.hs[0:0]
	c.TableStats = c.TableStats[0:0]
	for i := 0; i < tables; i++ {
		c.addHash()
	}
	//fmt.Printf("c.seeds=%#v\n", c.seeds)
	//fmt.Printf("c.hf=%#v\n", c.hf)
	//fmt.Printf("c.Config=%#v\n", c.Config)

	// init the table
	c.tbs = make([][]Buckets, tables, tables)
	for t, _ := range c.tbs {
		c.tbs[t] = make([]Buckets, buckets, buckets)
		c.TableStats[t].Size = buckets * slots
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

func (c *Cuckoo) LoadFactor() float64 {
	return float64(c.Elements) / float64(c.Size)
}

func (c *Cuckoo) SetStartLevel(sl int) {
	c.StartLevel = sl
}

func (c *Cuckoo) SetLowestLevel(ll int) {
	c.LowestLevel = ll
}

func  (c *Cuckoo) calcHashes(key Key) {
	if normal {
		buf := new(bytes.Buffer)
		//b := make([]byte, 4)
		err := binary.Write(buf, binary.LittleEndian, int32(key))
		if err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		if l, err2 := buf.Read(c.b); l != 4 {
			fmt.Printf("l=%d, err=%q\n", l, err2)
			panic("Insert: Read")
		}
	} else {
		c.b[0], c.b[1], c.b[2], c.b[3] = byte(key&0xFF), byte((key>>8)&0xFF), byte((key>>16)&0xFF), byte((key>>24)&0xFF)
	}
	// calculate hashes for each table
	for k, v := range c.hf {
		if false {
			v.Reset()
			v.Write(c.b)
			h1 := v.Sum32()
			c.hs[k] = h1 % uint32(c.Buckets)
			//fmt.Printf("c.hs[%d]=0x%x, Sum32(b)=0x%x\n", k, h1, murmur3.Sum32(b, c.seeds[k]))
		} else {
			c.hs[k] = murmur3.Sum32(c.b, c.seeds[k]) % uint32(c.Buckets)
		}
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}
}

/*
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

	c.calcHashes(key)
	// calculate hashes for each table
	for k, v := range c.hf {
		v.Reset()
		v.Write(b)
		c.hs[k] = v.Sum32() % uint32(c.Buckets)
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}


*/

func (c *Cuckoo) Lookup(key Key) (Value, bool) {
	c.Lookups++

	if key == c.emptyKey {
		if c.emptyKeyValid {
			return c.emptyValue, true
		} else {
			return zeroVal, false
		}
	}

	c.calcHashes(key)
	for t, _ := range c.tbs {
		b := c.hs[t] % uint32(c.Buckets)
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

/*
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

	// calculate hashes for each table
	for k, v := range c.hf {
		v.Reset()
		v.Write(b)
		c.hs[k] = v.Sum32() % uint32(c.Buckets)
		//fmt.Printf("hf[%d]=0x%x\n", k, c.hs[k])
	}
*/

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

	c.calcHashes(key)
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
	//fmt.Printf("Delete: can't find %v\n", key)
	return false, zeroVal
}

func (c *Cuckoo) insert(key Key, val Value, level int) (ok bool, rlevel int) {
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
					c.TableStats[t].Elements++
					return true
				}
			}
			// no slots available in this table available, pick a random key to move to the next table
			c.Bumps++
			c.TableStats[t].Bumps++
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

		// skip 0 because it's used as a signal that Insert failed because of load constaint
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
				fmt.Printf("insert: aborted\n")
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
			c.Elements++
			c.emptyKeyValid = true
			c.emptyValue = v
		}
		return true, level
	}
	aok := ins(k, v)
	if aok {
		c.Inserts++
	} else {
		if grow {
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
	return aok, level
}

func (c *Cuckoo) Insert(key Key, val Value) (ok bool, rlevel int) {
	return c.insert(key, val, c.StartLevel)
}

/*
func init() {
	var k Key
	var v Value = 1// "foobar"

	fmt.Printf("Key=%T\n", k)
	fmt.Printf("Value=%T\n", v)
}
*/
