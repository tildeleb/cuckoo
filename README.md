*Under Active Development*

<img src="http://clipart.coolclips.com/100/wjm/tf05110/CoolClips_hous1343.jpg"></img>

Cuckoo Hash Tables
==================

This package is an implementation of a Cuckoo Hash Table (CHT). [^1] A Cuckoo Hash Table is similar to Go's builtin hash map but uses multiple hash tables with a cascading random walk slot eviction strategy when hashing conflicts occur. Additional hash tables can optionally be added on the fly. A Cuckoo Hash Table is a 3D data structure. Multiple hash tables are comprised of buckets. Each bucket contains slots. Each slot contains a key/value pair. The hash tables all use the same hash function but with different seeds.

Go's builtin map is well designed and implemented. The author uses it all the time. This CHT is a boutique and bespoke data structure better suited for special cases where the datasets are large, memory efficiency is key, or both.

*Why use a CHT instead of Go's builtin map?*

1. Memory Efficiency. In a benchmark below `map[uint64]uint64` the CHT used 6.5X (15 MiB vs 117 MiB) less memory than Go's built-in map at competitive speeds for insert and lookup. This is because you can tune CHT's key and value to your specific needs, while Go's map current uses a single hash table with 8 slots and an overflow pointer. For small lookup tables this means CHT stores more data in L1/L2/L3 cache. For larger tables it means greater overall memory efficiency.

2. Memory Efficiency. In addition to being the above, a CHT can handle load factors as high as .999 with some tradeoff in insert efficiency. If you have a mostly read only data structure a CHT is perfect. Even if you don't, the knobs and dials in this implementation can be set to give you the insert efficiency you desire.

3. Large Scale Memory Efficiency. As of 1.4 I believe Go's maps uses power of two sizes for its hash tables. This allows a mask to be used to calculate the bucket index instead of a MOD instruction. Unfortunately, large data sets often require a much larger allocation than required. For example a 2 GB + 1 byte data structure will require 4 GB. (*needs to be confirmed*)

4. No GC pauses. As of Go 1.4 Go's maps can be subject to significant GC pauses as the overflow pointers are scanned. The CHT has no overflow pointers and no pointers at all, unless you add them to the value you store. This may change in Go 1.5.

5. Knobs and Dials. This CHT has a number of knobs you can tweak to get the results you want

Load factors as high as .999 are achievable with the caveats that the amount of work per insertion increases as the hash table fills up (load factor increases) and the amount of work per delete increases with the number of hash tables and slots. The amount of work on Insert can be ameliorated by decreasing the load factor, increasing the number of hash tables, the number of slots per bucket, or both.

Additionally, Cuckoo Hash Tables are subject to pathological cases (cycles) that can prevent an insert from completing. If a cycle does occur, it can be automatically detected, another hash table is added on the fly, and the insert is guaranteed to complete. The amount of work done before a cycle is assumed can also be configured by the user via an API call.

In this implementation there are three ways to reduce the probability of running into a pathological case:

1. Increase the number of slots per bucket (again, 4, 8, or 16 are good numbers)
2. Increase the number of hash tables to a number greater than 2 helps (4 is a good number)
3. Reduce the load factor  

Slots are a very effective way of achieving high load factors efficiently. 8, 16, and 32 slots per bucket allow for very high load factors. Adding hash tables is not nearly as efficient as more hashes per insert need to be calculated. Therefore more slots are preferred over more hash tables, but some balance is required between the two. Hash tables can be added on the fly, slots can not.

The implementation keeps a number of counters that can be used to derive statistics about how well the implementation is performing. I could not have easily developed this package without the counters.

An example program is included which easily allows one to quickly try out new combinations of parameters and explore the results. Unit tests verify that the implementation works as advertised. Benchmarks are also included.

Status
------
*The code should currently be considered pre-alpha quality.*

Goals For This Version
----------------------
* Allow for many configuration options to explore the design space
* 64 bit hash functions
* Clear code that facilitates understanding the algorithm
* Top notch memory and CPU efficiency within the bounds of pure Go
* Comprehensive counters to allow for dynamic debugging and statistical analysis
* Good example program with lots of options to play with various parameters
* User selectable hash functions
* Support for non power of two table size and therefore the use of mod to calculate a bucket index.
* Production quality code with testing
* 100% written in Go with ~~no~~ few external dependencies (for the main package)

Future Development
------------------
* Concurrent lock free access
* Stable iteration even with concurrent access 
* More hash functions like CityHash, SIPHash, and others
* More test cases

Bugs and Issues
---------------
* Simple iteration support in this version, works well when table is full-ish, slow when table close to empty.

Example of a Pathological Case
------------------------------
In the following example a 4 table x 11 buckets x 8 slot cuckoo hash is constructed and 1 million trials are run doing inserts/verify/delete.. In all but a single case the CHT was able to achieve a perfect load factor of 1.0, which means that the table was completely filled. In the single case that failed, only a single insert, the final insert, could not be completed. This defines life with a cuckoo table.

If the single failure makes you unhappy I suggest you change the number of slots from 8 to 16 and investigate how many trials it takes to find a failure.

	leb@hula:~/gotest/src/leb/cuckoo/example % time ./example -t 4 -b 11 -s 8 -nt=1000000 -flf=1.0 -lf=10 -dg -rb=true
	trials: size=8 kbytes
	trials: tables=4, buckets=11, slots=8, size=352, max=352, trials=1000000, fails=1, avg=1.0000
	trials: MaxRemaining=1, LowestLevel=-168, Aborts=168, bpi=2.15, api=21.68, ipi=0.1618
	./example -t 4 -b 11 -s 8 -nt=1000000 -flf=1.0 -lf=10 -dg -rb=true  524.49s user 3.76s system 101% cpu 8:42.49 total
	leb@hula:~/gotest/src/leb/cuckoo/example % 

How to Build
------------

1. Edit the file "kv_default.go" and define the types for "Key" and "Value"
2. % go build  

Note the default build uses a slice per bucket to implement slots. This allows for experimentation with the number of slots without recompiling but is inefficient from a memory usage perspective. When the number of slots needed for your application is finalized do the following:

1. Edit "kvt_array.go" and define Slots to be the number of slots needed for your application.
2. % go build -tags="array"

This will build a version that uses arrays. Calls to cuckoo.New where the number of slots passed in does not match the number specified in "kvt_array.go" will fail.

Included Sub-Packages
---------------------
* jenkins hash package
* murmur3 hash package
* dtest test framework
* primes provides prime numbers for 

Benchmarks
----------
The following benchmark data is from a run on my MacBook Pro 2.5 GHz Core i7. The Cuckoo Hashtable configuration is 2 hash tables with 8 slots per bucket with the array optimization. Another optimization is turned on that marshals numeric quantities (currently 32 and 64 bit only) more efficiently than using the binary package.

	leb@hula:~/gotest/src/github.com/tildeleb/cuckoo % go test -bench=. -v
	=== RUN TestBasic-11
	--- PASS: TestBasic-11 (0.41s)
	=== RUN TestMemoryEfficiency-11
	--- PASS: TestMemoryEfficiency-11 (0.56s)
		cuckoo_test.go:147: Cuckoo Hash LoadFactor:       0.99
		cuckoo_test.go:148: Cuckoo Hash memory allocated: 15 MiB
		cuckoo_test.go:149: Go map memory allocated:      117 MiB
	=== RUN: Example
	--- PASS: Example (0.00s)
	PASS
	BenchmarkCuckoo2T2SInsert-11	 5000000	       242 ns/op	       0 B/op	       0 allocs/op
	BenchmarkCuckoo2T2SSearch-11	10000000	       172 ns/op	       0 B/op	       0 allocs/op
	BenchmarkCuckoo2T2SDelete-11	10000000	       308 ns/op	       0 B/op	       0 allocs/op
	BenchmarkGoMapInsert-11	 5000000	       264 ns/op	      33 B/op	       0 allocs/op
	BenchmarkGoMapSearch-11	10000000	       144 ns/op	       0 B/op	       0 allocs/op
	BenchmarkGoMapDelete-11	50000000	        22.8 ns/op	       0 B/op	       0 allocs/op
	ok  	github.com/tildeleb/cuckoo	33.872s
	leb@hula:~/gotest/src/github.com/tildeleb/cuckoo % 

Benchmarks Discussion
---------------------
For the case "var map[uint64]uint64 Cuckoo Hash uses 6.5X less memory than Go's builtin map and does so while achieving a load factor of 99% with similar efficiency. Again, the Cuckoo Hash for this example uses 2 hash tables and each bucket has 8 slots. From a performance standpoint the Cuckoo hash achieves 242 ns/op on Inserts vs 264 ns/op for the build-in map.


Selectable Hash Functions
-------------------------
The hash function used by this package can be selected. Currently only two hash functions are supported.

1. "aes" This hash function is the same hash function used by Go's map. AESNI instructions are used to generates a very fast high quality hash function. Special versions for 32 and 64 bit data are supported. This hash function is 5x faster than "j264".

2. "j264" This is a version of Jenkin's 2nd generation hash functions. There is some optimization for speed but no special versions of 32 and 64 bit data. No assembler optimization. No fast path. No inlining.

Defining Your Own Key/Value Types
---------------
The package supports almost any kind of key and value type by simply creating a new "kvt" file. The file "kvt_default.go" can be edited to change the definitions for Key and Value.


Support for Arrays or Slices via Build Tags
------------------------------------------
The package has an optimization to implement slots as either slices or arrays. Slices allow the number of slots to be selected at runtime but the slice overhead per bucket is high. Therefore, once the number of slots is known, it's best to switch to a static array size. 

[fix]

Slices are not very efficient but you can try out new sizes without having to edit a file.
Arrays are more efficient wither cpu or memory wise because they are not a reference type so there is the overhead of an 8 byte pointer on a 64 bit system and the cache miss(es) that go along with that pointer dereference.


As an example here is a file "kvt_uint32_uint32_slice.go" that defines a "uint32" key, a "uint32" value, and a uses slices.
	
	// +build kuint32,vuint32,slice
	
	package cuckoo
	
	type Key uint32
	type Value uint32
	
	type Buckets	[]Bucket		// slots
	func makeSlots(b Buckets, slots int) Buckets {
		return make(Buckets, slots, slots)
	}

To build this version of the Cuckoo Hash you would issue the following command

	go build -tags="kuint32 vuint32 slice"

and here is a similar file "kvt_uint32_uint32_array.go" that uses an array type instead of a slice:

	// +build kuint32,vuint32,array
	
	package cuckoo
	
	type Key uint32
	type Value uint32
	
	const Slots = 4
	
	type Buckets	[Slots]Bucket		// slots
	func makeSlots(b Buckets, slots int) Buckets {
		return b
	}
	
To build this version of the Cuckoo Hash you would issue the following command

	go build -tags="kuint32 vuint32 array"

Example Program
---------------
There is an example program which is useful or exploring the tuning of cuckoo hash tables and verifying the implementation.

	Usage of ./example:
	  -a=false: automatic
	  -b=31: buckets
	  -base=1: base of fill series, -1 for random
	  -cp="": write cpu profile to file
	  -dg=false: dont't add hash tables automatically
	  -flf=1: fill load factor
	  -fo=false: fill only
	  -h="aes": name of hash function (aes or j264)
	  -lf=0.96: maximum load factor
	  -ll=-8000: lowest level
	  -mp="": write memory profile to this file
	  -nt=5: number of trials
	  -pl=false: print level of each insert
	  -pr=false: print progress
	  -ps=false: print stats at the end of all trails
	  -pt=false: print summary for each trail
	  -rb=true: random base
	  -rr=true: random run
	  -s=8: slots
	  -sl=2000: starting level
	  -t=4: tables
	  -v=false: verbose

Let's take a simple example of a classic (two table) cuckoo table. This example creates a cuckoo hash table with 2 hash tables, 11 buckets, and 1 slot per bucket. The occupancy of the hash table won't exceed a load factor of greater than 40%.

	% ./example -t 2 -b 11 -s 1 -nt=5 -lf=0.4 -ps 
	trials: size=352 bytes
	trials: trial=0, Remaining=14, Aborts=0, LowestLevel=2000, MaxAttemps=2, MaxIterations=0, bpi=0.25, api=1.25, ipi=0.0000
	trials: trial=1, Remaining=14, Aborts=0, LowestLevel=2000, MaxAttemps=2, MaxIterations=0, bpi=0.12, api=1.12, ipi=0.0000
	trials: trial=2, Remaining=14, Aborts=0, LowestLevel=2000, MaxAttemps=1, MaxIterations=0, bpi=0.00, api=1.00, ipi=0.0000
	trials: trial=3, Remaining=14, Aborts=0, LowestLevel=2000, MaxAttemps=2, MaxIterations=0, bpi=0.25, api=1.25, ipi=0.0000
	trials: trial=4, Remaining=14, Aborts=0, LowestLevel=2000, MaxAttemps=2, MaxIterations=0, bpi=0.38, api=1.38, ipi=0.0000
	trials: tables=2, buckets=11, slots=1, size=22, max=22, trials=5, fails=5, avg=0.3636
	trials: Aborts=0, bpi=0.20, api=1.20, ipi=0.0000
	trials: MaxRemaining=14
	trials: LowestLevel=2000
	trials: c=&cuckoo.CuckooStat{BucketSize:16, Elements:40, Inserts:40, Attempts:48, Iterations:0, Deletes:40, Lookups:40, Fails:0, Bumps:8, Aborts:0, MaxAttempts:0, MaxIterations:0, Limited:false}
	 % 

So this creates a cuckoo table with 2 hash tables, 11 buckets, and 1 slot per bucket. It runs 5 trials with a load factor of 40%. 2 x 11 x 1 = 22 x .4 = 8.8 = 8. 22 slots - 8 = 14 slots remaining. The average load achieved for all 3 trials is 0.36.

The stats "bpi", "api", and "ili" stand for "bumper per insert", "attempts per insert", and "iterations per insert". 

Now let's look at cuckoo table can support a load factor of 99.9%, albeit with some time consuming insertions as the table fills up.

	leb% ./example -t 4 -b 14009 -s 4 -nt=5 -lf=0.999 -ps     
	trials: size=3 Mbytes
	trials: trial=0, Remaining=225, Aborts=0, LowestLevel=1390, MaxAttemps=9775, MaxIterations=610, bpi=3.20, api=15.30, ipi=0.4255
	trials: trial=1, Remaining=225, Aborts=0, LowestLevel=1301, MaxAttemps=11200, MaxIterations=699, bpi=3.18, api=15.22, ipi=0.4205
	trials: trial=2, Remaining=225, Aborts=0, LowestLevel=1472, MaxAttemps=8464, MaxIterations=528, bpi=3.13, api=15.02, ipi=0.4081
	trials: trial=3, Remaining=225, Aborts=0, LowestLevel=1631, MaxAttemps=5920, MaxIterations=369, bpi=3.17, api=15.16, ipi=0.4169
	trials: trial=4, Remaining=225, Aborts=0, LowestLevel=1375, MaxAttemps=10016, MaxIterations=625, bpi=3.19, api=15.27, ipi=0.4237
	trials: tables=4, buckets=14009, slots=4, size=224144, max=224144, trials=5, fails=5, avg=0.9990
	trials: Aborts=0, bpi=3.17, api=15.20, ipi=0.4189
	trials: MaxRemaining=225
	trials: LowestLevel=1301
	trials: c=&cuckoo.CuckooStat{BucketSize:16, Elements:1119595, Inserts:1119595, Attempts:17013688, Iterations:469042, Deletes:1119595, Lookups:1119595, Fails:0, Bumps:3554019, Aborts:0, MaxAttempts:0, MaxIterations:0, Limited:false}
	leb% 

The key number to look at here is the api which has moved from 1.20 on the classic hash table to a 15.20 here. So the CHT has to try 15 locations on average to insert a key.


Implementation [Must Proofread]
--------------
This version of a cuckoo hash table implements a three dimensional hash table. In concrete terms we have "t" hash tables, each has tables has "b" buckets, and each bucket has "s" slots. Total entries is simply t * b * s. In practical terms t can range from 2 to 4 and maybe as high as 8 and slots can range from 1 to 8 and maybe as high as 16 or 32. Access to slots is fast because the pre-fetcher gets them into the L1 cache. The number of buckets should be a prime number. Within reason slots are more efficient execution time wise then hash tables, so prefer slots to tables. For expositional purpose consider hash tables laid out in left to right order.

The insert algorithm is as follows. For the given key, a hash value is calculated for each hash table. The bucket in the leftmost table is indexed by its key and if a free slot is found it is used. If none of the slots are free a random slot is evicted and the new key/value pair is stored where there and the evicted slots becomes the new ke/value pair to be inserted in the next rightmost hash table.

The evicted key and it's value are  then attempted to be stored in the next hash table to the right and the same procedure is followed until hopefully a home is found for all key/value pairs.

The entire procedure is repeated until the end of the left to right hash tables is reached. again for a her specified number of iterations. When the number of iterations has expired (== 0) the algorithm goes into recovery mode, where instead of trying to insert a value it tries to get the value to be inserted back as the value to be inserted. This isn't alway possible in which case data loss happens.

Cuckoo tables are known for their efficiency. Go to the example folder and run: 

	leb@hula:~/gotest/src/leb/cuckoo/example % time ./example -t 4 -b 11 -s 8 -nt=1000000 -flf=1.0 -lf=10 -dg -rb=true
	trials: size=8 kbytes
	trials: tables=4, buckets=11, slots=8, size=352, max=352, trials=1000000, fails=1, avg=1.0000
	trials: MaxRemaining=1, LowestLevel=-168, Aborts=168, bpi=2.15, api=21.68, ipi=0.1618
	./example -t 4 -b 11 -s 8 -nt=1000000 -flf=1.0 -lf=10 -dg -rb=true  524.49s user 3.76s system 101% cpu 8:42.49 total
	leb@hula:~/gotest/src/leb/cuckoo/example % 

A cuckoo has table with 352 locations in it was constructed and 352 random numbers were inserted into this hash table. 1 millions trials were run and with the exception of a single trial all trials achieved a perfect load factor of 1.0, e.g. all the numbers could be inserted. In the single failure case only the last number could not be inserted. So you feel lucky today? If not, I suggest you up the number of tables or slots until you feel lucky.

	leb@hula: % time ./example -t 4 -b 31 -s 16 -flf=1.0 -lf=1.0 -dg -rb=true -nt=1000000
	trials: cucko hash table size=248 Kibytes
	trials: tables=4, buckets=31, slots=16, size=1984, max=1984, trials=1000000, fails=0, avg=1.0000
	trials: MaxRemaining=0, LowestLevel=1438, Aborts=0, bpi=2.09, api=41.98, ipi=0.1481
	./example -t 4 -b 31 -s 16 -flf=1.0 -lf=1.0 -dg -rb=true -nt=1000000  5420.39s user 28.36s system 100% cpu 1:30:42.80 total
	leb@hula: % # 	load: 3.28  cmd: example 75597 running 5339.27u 27.95s

Since the example above had a failure rate of 0.0001% let's see if we can improve that. The easiest way is to increase the associativity per bucket and go from 8 slots/bucket to 16 slocks/bucket.

Note the size of the hash tables is 248 Kibytes which just fits in the L2 cache of the processor in my laptop. When I test there are 3 sizes that makes sense to test with. 32KB or less means everything fits in the L1 data cache and this runs very quickly. 256KB is the size of my L2 cache. Memory latency here is 10X L1. My L3 cache is 8 MB. Latencies are about 4, 12, and 28 cycles respectively.

A cuckoo has table with 1984 locations in it was constructed and 1984 random numbers were inserted into this hash table, verified, and deleted. 1 millions trials were run and all trials achieved a perfect load factor of 1.0, e.g. all the numbers could be inserted. 

I am lucky today.


Implementation FAQ
------------------
**Q** Why is delete so slow?
**A** Two reasons. First, Go's map uses a trick where some of the hash bits are used to index into the slots, saving a scan of the slots. The CHT can't use that trick. Second, Go's map just sets a bit to delete a slot but the CHT currently copies the "empty key" into the key of the slot being deleted.

**Q**: Why do you use mod instead of power of two tables with a bit mask for bucket indexing?  
**A**: This was a difficult decision. Calculating MOD is much slower than performing a power of two masking AND operation. Also to (always) be considered are the memory caching effects. ‘MOD’ is slower than AND by an amount larger than an L1-miss-L2-hit time. So assuming that miss-hit pattern (unclear, depends on table size and other factors) it might be better to re-probe once than calculate the MOD.

I am interested in working with large datasets. In the end the main reason I choose MOD over power of two table sizes with AND masking is because the latter doesn't scale efficiently to large datasets. e.g. if I have a 16 GB  dataset and it grows by one more entry, it will need a 32 GiB allocation and waste 16 GiB.

**Q**: Don't you know MOD is slow?  
**A**: Sure, but see above.

**Q**: What hash functions are used?
**A**: You currently have a choice of an X86-64 accelerated 64 bit hash calculated using AESNI on X86-64 platforms. or Siphash also accelerated on X86-64 but has fallback to pure Go for platforms other than X86-64. 

**Q**: Why is Delete so slow?  
**A**: Essentially because Delete has to look is t * s places to find the key whereas Go's build in map only has to look in a single place. In the example benchmarks t == 2 and s == 8 so s * t == 16. Therefor on average Delete has to do 8 lookups to find the key. The speed of Delete can be increased by decreasing the number of slots and tables.

**Q**: Why isn't there a stash?  
**A**: I read the Microsoft paper and wasn't impresses. Adding a stash is like adding a bag to the design. In order to guarantee insert doesn't fail we just add another hash table if needed. Adding a stash buffer would add a parallel data structure and code to manipulate it in Insert, Delete, and Lookup. 


References
----------
[^1]: [21] R. Pagh and F. Rodler. Cuckoo hashing. Journal of Algorithms,51(2):122–144, May 2004.

