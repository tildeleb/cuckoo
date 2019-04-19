// quick 10 minute transliteration of plan9/p9p primes.c
package primes

import "math"

var big = 9.007199254740992e15

var pt = []int{
	2, 3, 5, 7, 11, 13, 17, 19, 23, 29,
	31, 37, 41, 43, 47, 53, 59, 61, 67, 71,
	73, 79, 83, 89, 97, 101, 103, 107, 109, 113,
	127, 131, 137, 139, 149, 151, 157, 163, 167, 173,
	179, 181, 191, 193, 197, 199, 211, 223, 227, 229,
}

var wheel = []int{
	10, 2, 4, 2, 4, 6, 2, 6, 4, 2,
	4, 6, 6, 2, 6, 4, 2, 6, 4, 6,
	8, 4, 2, 4, 2, 4, 8, 6, 4, 6,
	2, 4, 6, 2, 6, 6, 4, 2, 4, 6,
	2, 6, 4, 2, 4, 2, 10, 2,
}
var table [10000]byte
var tsiz8 = len(table) * 8
var bittab = []byte{1, 2, 4, 8, 16, 32, 64, 128}

func mark(nn float64, k int) {
	t1, _ := math.Modf(nn / float64(k))
	j := int(float64(k)*t1 - nn)
	if j < 0 {
		j += k
	}
	for ; j < 8; j += k {
		table[j>>3] |= bittab[j&07]
	}
}

// collect primes between a and b and call f with each prime
func Primes(nn, limit float64, f func(v int)) {
	if limit == 0.0 {
		limit = big
	}

	if limit > big {
		panic("limit > big")
	}

	if nn < 0 || nn > big {
		panic("nn < 0 || nn > big")
	}

	if nn == 0 {
		nn = 1
	}

	if nn < 230 {
		for k, vi := range pt {
			v := float64(vi)
			if v < nn {
				continue
			}
			if v > limit {
				return
			}
			f(int(pt[k]))
			if limit >= big {
				return
			}
		}
		nn = 230
	}

	temp, _ := math.Modf(nn / 2)
	nn = 2.0*temp + 1

	// clear the sieve table.
	for {
		for k := range table {
			table[k] = 0
		}

		// run the sieve.
		max := int(math.Sqrt(nn + float64(tsiz8)))
		mark(nn, 3)
		mark(nn, 5)
		mark(nn, 7)
		for i, k := 0, 11; k <= max; k += wheel[i] {
			mark(nn, k)
			i++
			if i >= len(wheel) {
				i = 0
			}
		}

		// now get the primes from the table and print them.
		for i := 0; i < tsiz8; i += 2 {
			if (table[i>>3] & bittab[i&07]) != 0 {
				continue
			}
			temp = nn + float64(i)
			if temp > limit {
				return
			}
			f(int(temp))
			if limit >= big {
				return
			}
		}
		nn += float64(tsiz8)
	}
}

func NextPrime(n int) (p int) {
	Primes(float64(n), 0.0, func(v int) { p = v })
	return
}
