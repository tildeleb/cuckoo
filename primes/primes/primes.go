package main

import "flag"
import "fmt"
import "strconv"
import "github.com/tildeleb/cuckoo/primes"

func main() {
	flag.Parse()
	numbers := flag.Args()
	nn, _ := strconv.ParseFloat(numbers[0], 64)
	limit := 0.0
	if len(numbers) > 1 {
		limit, _ = strconv.ParseFloat(numbers[1], 64)
		if limit < nn {
			panic("limit < nb")
		}
	}
	primes.Primes(nn, limit, func(v int) {fmt.Printf("%d\n", v)})
}