package main

import "fmt"
import _ "math"
import "hash"
import "bytes"
import "time"
import "flag"
import "math/rand"
import "encoding/binary"

import "leb/cuckoo/murmur3"
import "leb/cuckoo/primes"

type Key int
type Value int

type bucket struct {
	key		Key
	val		Value
}

// Per table stats
type stat struct {
	size		int
	elements	int
	bumps		int
}

type Cuckoo struct {
	ntables			int
	nbuckets		int
	nslots			int
	elements		int
	inserts			int
	bumps			int
	emptyKey		Key
	stats			[]stat
	hf				[]hash.Hash32	// one for each table + the last one reserved for fingerprints
	hs				[]uint32		// hash sums for each table and fingerprint
	tbs				[][][]bucket	// [ntables][nbuckets][nslots]TablesBucketsSlots
}

var r = rand.Float64

func getHashes(hashName string, n int) []hash.Hash32 {
	var h []hash.Hash32
	for i := 0; i < n; i++ {
		seed := i
		if seed == n {
			seed = 0
		}
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

func rbetween(a int, b int) int {
	rf := r()
	diff := float64(b - a + 1)
	r2 := rf * diff
	r3 := r2 + float64(a)
	//	fmt.Printf("rbetween: a=%d, b=%d, rf=%f, diff=%f, r2=%f, r3=%f\n", a, b, rf, diff, r2, r3)
	ret := int(r3)
	return ret
}

func New(ntables, nbuckets, nslots int) *Cuckoo {
	c := &Cuckoo{ntables: ntables, nbuckets: nbuckets, nslots: nslots}
	c.elements = ntables * nbuckets * nslots
	c.stats = make([]stat, ntables)
	c.tbs = make([][][]bucket, ntables, ntables)
	c.hf = getHashes("m332", ntables)
	c.hs = make([]uint32, len(c.hf))
	for t, _ := range c.tbs {
		c.tbs[t] = make([][]bucket, nbuckets, nbuckets)
		c.stats[t].size = nbuckets * nslots
		for b, _ := range c.tbs[t] {
			c.tbs[t][b] = make([]bucket, nslots, nslots)
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

	// calculate hashes for each table
	for k, v := range c.hf {
		v.Reset()
		v.Write(b)
		c.hs[k] = v.Sum32() % uint32(c.nbuckets)
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
	return 0, false
}

func (c *Cuckoo) Verify(base, n int) bool {
	//fmt.Printf("Verify: base=%d, n=%d \n", base, n)
	cnt := 0
	for i := base; i < base + n; i++ {
		cnt++
		v, ok := c.Lookup(Key(i))
		if !ok {
			fmt.Printf("Verify: lookup FAILED i=%d, cnt=%d\n", i, cnt)
			return false
		}
		//fmt.Printf("Verify: check i=%d, cnt=%d != v=%d\n", i, cnt, uint32(v))
		if uint32(cnt) != uint32(v) {
			fmt.Printf("Verify: FAIL i=%d, cnt=%d != v=%d\n", i, cnt, uint32(v))
			return false
		}
	}
	//fmt.Printf("Verify: OK\n")
	return true
}

func (c *Cuckoo) insert(key Key, val Value, level int) (ok bool) {
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
		buf := new(bytes.Buffer)
		b := make([]byte, 4)
		err := binary.Write(buf, binary.LittleEndian, int32(key))
		if err != nil {
			//fmt.Printf("Write: err=%q\n", err)
			panic("Insert: binary.Write")
		}
		if l, err2 := buf.Read(b); l != 4 {
			fmt.Printf("l=%d, err=%q\n", l, err2)
			panic("Insert: Read")
		}
		// calculate hashes for each table
		for k, v := range c.hf {
			v.Reset()
			v.Write(b)
			c.hs[k] = v.Sum32() % uint32(c.nbuckets)
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
				if c.tbs[t][b][s].key == 0 {
					c.tbs[t][b][s].key, c.tbs[t][b][s].val = k, v
					//fmt.Printf("Insert: level=%d, key=%d, value=%d, table=%d, bucket=%d, slot=%d\n", level, k, v, t, b, s)
					c.inserts++
					c.stats[t].elements++
					return true
				}
			}
			// no slots available in any table available, pick a random key to move to the next table
			c.bumps++
			c.stats[t].bumps++
			victim := rbetween(0, c.nslots-1)
			//fmt.Printf("insert: level=%d, bump value=%d for value=%d, table=%d, bucket=%d, slot=%d\n", level, c.tbs[t][b][victim].val, val, t, b, victim)
			bucket := c.tbs[t][b][victim]
			c.tbs[t][b][victim].key = k
			c.tbs[t][b][victim].val = v
			k = bucket.key
			v = bucket.val
			calcHashes(k)
			//fmt.Printf("insert: level=%d, new key=%d, val=%d\n", level, k, v)
		}
		level--
		if level <= 0 {
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
	//fmt.Printf("Insert: level=%d, key=%d, value=%d\n", level, key, val)
	return ins(key, val)
}

func (c *Cuckoo) Insert(key Key, val Value) (ok bool) {
	return c.insert(key, val, *maxLevel)
}

func trials(tables, buckets, slots, trials int, verbose bool) (float64, int) {
	tot := float64(0)
	fails := 0
	for t := 0; t < trials; t++ {
		ck := New(tables, buckets, slots)
		base := 0
		if *ranb {
			base = rbetween(1, 1<<29)
		}
		cnt := 1
		maxi := base + tables * buckets * slots
		percent := float64(1.0)
		for i := base+1; i <= base + tables * buckets * slots; i++ {
			if !ck.Insert(Key(i), Value(uint32(cnt))) {
				percent = float64(ck.inserts)/float64(ck.elements)
				if verbose {
					fmt.Printf("failed: %d/%d, bumps=%d, %d/%d=%0.4f\n", i, maxi, ck.bumps, ck.inserts, ck.elements, percent)
				}
				maxi = i - 1
				fails++
				break
			}
			cnt++
		}
		tot += percent
		if verbose {
			for k, v := range ck.stats {
				fmt.Printf("table[%d]: %d/%d=%0.4f\n", k, v.elements, v.size, float64(v.elements)/float64(v.size))
			}
		}
		ck.Verify(base+1, maxi - base)
	}
	avg := tot / float64(trials)
	//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, avg=%0.2f\n", tables, buckets, slots, trials, avg)
	return avg, fails
}

var auto = flag.Bool("a", false, "automatic")
var ranf = flag.Bool("rr", true, "random run")
var ranb = flag.Bool("rb", false, "random base")
var ntables = flag.Int("t", 8, "tables")
var nbuckets = flag.Int("b", 10, "buckets")
var nslots = flag.Int("s", 8, "slots")
var ntrials = flag.Int("nt", 100, "number of trials")
var maxLevel = flag.Int("l", 2, "max level")

func main () {
	flag.Parse()
    seed := int64(0)
    // fixed pattern or different values each time
    if *ranf {
        seed = int64(0)
    } else {
        seed = time.Now().UTC().UnixNano()
    }
    rand.Seed(seed)

    //tables := []int{2, 3, 4, 5, 6, 7, 8}
    //slots := []int{1, 2, 3, 4, 5, 6, 7, 8}

    st := 0
    ss := 0
    fails := 0
    verbose := false
    if *ntrials == 1 {
    	verbose = true
    }
    if *auto {
    	max := float64(0)
		for _, b := range primes.Primes {
    		for t := 1; t <= *ntables; t++ {
    			for s := 1; s <= *nslots; s++ {
					//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d\n", t, b, s, *ntrials)
    				avg, f := trials(t, b, s, *ntrials, verbose)
    				fails += f
 					//fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d\n", t, b, s, *ntrials)
    				if avg > max {
    					max = avg
    					st = t
    					ss = s
    				}
    			}
    		}
	    	fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, fails=%d, avg=%0.4f\n", st, b, ss, *ntrials, fails, max)
    		max = 0.0
    	}
    } else {
		avg, f := trials(*ntables, *nbuckets, *nslots, *ntrials, verbose)
		fmt.Printf("tables=%d, buckets=%d, slots=%d, trials=%d, fails=%d, avg=%0.4f\n", *ntables, *nbuckets, *nslots, *ntrials, f, avg)
	}
}
