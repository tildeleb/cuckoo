**Under Active Development**
============================

**"A total work in progress at this point in time" –– leb**
==========================================

Cuckoo Hash Tables
==================

This package is an implementation of a Cuckoo Hash Table. [^1] A Cuckoo Hashtable is similar to Go's builtin map but uses multiple hash tables with a random walk slot eviction strategy when hashing conflicts occur. A hash table is comprised of buckets and each bucket can have multiple slots. Each slot contains a key/value pair.

*The package is is 100% written in Go with no external dependencies.*

Load factors as high as .999 are achievable with the caveats that the amount of work per insertion increases as the hash table fills up (load factor increases) and the amount of work per delete increases with the number of hash tables and slots. The amount of work on Insert can be ameliorated by increasing the number of hash tables, the number of slots per bucket, or both. Cuckoo hash tables are subject to pathological cases (cycles) that can prevent an insert from completing. If a cycle does occur, it is automatically detected, another hash table is added, and the insert is guaranteed to complete. The amount of work done before a cycle is assumed can also be configured by the user via an API call.

In this implementation there are three ways to reduce the probability of running into a pathological case:

1. Set the number of slots per bucket (again, 4, 8, or 16 are good numbers)
2. Set the number of hash tables to a number greater than 2 helps (4 is a good number)
3. Reduce the load factor  

Slots are a very effective way of achieving high load factors efficiently. Adding hash tables is not nearly as efficient as more hashes per insert need to be calculated.

The implementation keeps a number of counters that can be used to derive statistics about how well the implementation is performing. I could not have easily developed the package with the counters.

An example  program is included which easily allows one to qucikly try out new combinations of parameters and explore the results. Unit tests verify that the implementation works as advertised. Benchmarks are also included.

Benchmarks
----------
The following benchmark data is from a run on my MacBook Pro 2.5 GHz Core i7. The Cuckoo Hashtable configuration is 2 hash tables with 8 slots per bucket along with the array optimization. Another optimization is turned on that marshals numeric quantities (currently 32 bit only) more efficiently than using the binary package.

	leb@hula:~/gotest/src/leb/cuckoo % go test -bench=. -v
	=== RUN TestMemoryEfficiency-11
	--- PASS: TestMemoryEfficiency-11 (1.43 seconds)
		cuckoo_test.go:71: Cuckoo Hash LoadFactor:       0.99
		cuckoo_test.go:72: Cuckoo Hash memory allocated: 15 MiB
		cuckoo_test.go:73: Go map memory allocated:      75 MiB
	PASS
	BenchmarkCuckoo2T2SInsert-11	 5000000	       296 ns/op	       0 B/op	       0 allocs/op
	BenchmarkCuckoo2T2SSearch-11	 5000000	       290 ns/op	       0 B/op	       0 allocs/op
	BenchmarkCuckoo2T2SDelete-11	10000000	       332 ns/op	       0 B/op	       0 allocs/op
	BenchmarkGoMapInsert-11	10000000	       219 ns/op	       9 B/op	       0 allocs/op
	BenchmarkGoMapSearch-11	20000000	       140 ns/op	       0 B/op	       0 allocs/op
	BenchmarkGoMapDelete-11	50000000	        28.4 ns/op	       0 B/op	       0 allocs/op
	ok  	leb/cuckoo	30.475s
	leb@hula:~/gotest/src/leb/cuckoo % 

Benchmarks Discussion
---------------------
For the case "var map[uint32]uint32 Cuckoo Hash uses 5X less memory than Go's builtin map and does so while achieving a load factor of 99% with a bit less efficiency. Again the Cuckoo Hash for this example uses 2 hash tables and each bucket has 8 slots. From a performance standpoint the Cuckoo hash achieves 296 ns/op on Inserts vs 219 ns/op for the build-in map. Much of this overhead has comes from calculating 2 hashes per insert.

Selectable Hash Functions
-------------------------
The hash function used by this package can be selected. Currently the only hash function supported is Murmur 3 with a 32 bit output or "m332" however support for many other hash functions is planned for a future commit.

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
	  -b=10: buckets
	  -base=1: base of fill series, -1 for random
	  -flf=1: fill load factor
	  -h="m332": name of hash function
	  -lf=0.96: maximum load factor
	  -ll=-8000: lowest level
	  -nt=5: number of trials
	  -pl=false: print level of each insert
	  -ps=false: print summary for each trail
	  -rb=true: random base
	  -rr=true: random run
	  -s=8: slots
	  -sl=2000: starting level
	  -t=8: tables
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

The key number to look at here is the api which has moved from 1.20 on the classic hash table to a 15.20 here. Note that the hash algorithm has to try 15 locations on average to insert a key.


Implementation
--------------
This version of a cuckoo hash table implements a three dimensional hash table. In concrete terms we have "t" hash tables, each has tables has "b" buckets, and each bucket has "s" slots. Total entries is simply t * b * s. In practical terms t can range from 2 to 4 and maybe as high as 8 and slots can range from 1 to 8 and maybe as high as 16 or 32. Access to slots is fast because the pre-fetcher gets them into the L1 cache. The number of buckets should be a prime number. Within reason slots are more efficient execution time wise then hash tables, so prefer slots to tables. For expositional purpose consider hash tables laid out in left to right order.

The insert algorithm is as follows. For the given key, a hash value is calculated for each hash table. The bucket in the leftmost table is indexed by its key and if a free slot is found it is used. If none of the slots are free a random slot is evicted and the new key/value pair is stored where there and the evicted slots becomes the new ke/value pair to be inserted in the next rightmost hash table.

The evicted key and it's value are  then attempted to be stored in the next hash table to the right and the same procedure is followed until hopefully a home is found for all key/value pairs.

The entire procedure is repeated until the end of the left to right hash tables is reached. again for a her specified number of iterations. When the number of iterations has expired (== 0) the algorithm goes into recovery mode, where instead of trying to insert a value it tries to get the value to be inserted back as the value to be inserted. This isn't alway possible in which case data loss happens.

Cuckoo tables are know for the efficiency. Go to the example and ty 

Implementation FAQ
------------------
**Q**: Why do you use mod instead of power of two tables with a bit mask for bucket indexing?  
**A**: I am interested in working with large datasets. The optimization of using power of two table sizes is known to me but it doesn't scale to large datasets. e.g. if I have a 16 GiB entry dataset and it grows y one more I will need a 32 GiB allocation and waste 16 GiB.

**Q**: Don't you know mod is slow?  
**A**: Sure but see above.

**Q**: You mention large datasets but this version uses a 32 bit hash?  
**A**: Right. This version uses a 32 bit murmur 3 hash which supports a hash table with 4G buckets. Assuming 4 slots per bucket that's 16G entries. Currently enough for what I need. a 64 bit version will be forthcoming as will versions that use different hash functions.

**Q**: Why is Delete so slow?  
**A**: Essentially because Delete has to look is t * s places to find the key whereas Go's build in map only has to look in a single place. In the example benchmarks t == 2 and s == 8 so s * t == 16. Therefor on average Delete has to do 8 lookups to find the key. The speed of Delete can be increased by decreasing the number of slots and tables.

References
----------
[^1]: [21] R. Pagh and F. Rodler. Cuckoo hashing. Journal of Algorithms,51(2):122–144, May 2004.

