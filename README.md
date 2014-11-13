**Under Active Development**
============================

**"A total work in progress at this point in time" –– leb**
==========================================

Cuckoo Hash Tables
==================

This package implements an implementation of cuckoo hash tables.

Implementation
--------------
This version of cuckoo tables implements a 3 dimensional has tables. In concrete terms we have "t" hash tables, each has tables has "b" buckets, and each bucket has "s" slots. Total entries is simply t * b * s. In practical terms t can range from 2 to 4 and maybe as high as 8 and slots can range from 1 to 8 and maybe as high as 16 or 32. The number of buckets should be a prime number. Within reason slots are more efficient execution time wise then hash tables, so prefer slots to tables. For expositional purpose consider hash tables laid out in left to right order.

The insert algorithm is as follows. A hash value for each table is calculated and the bucket indexed by the key for the leftmost table is tried first and if a free slot is found it is used. If none is available a random key is evicted and the new value is stored where the old key was. The evicted key and it's value are  then attempted to be stored in the next hash table to the right and the same procedure is followed until hopefully a home is found for all key/value pairs. If not, the entire procedure is tried again for a her specified number of iterations. When the number of iterations has expired (== 0) the algorithm goes into recovery mode, where instead of trying to insert a value it tries to get the value to be inserted back as the value to be inserted. This isn't alway possible in which case data loss happens.

Cuckoo tables are know for the efficiency. Go to the example and ty 