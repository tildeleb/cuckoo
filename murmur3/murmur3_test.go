//Copyright 2013, Sébastien Paolacci. All rights reserved.
//© Copyright 2014 Lawrence E. Bakst All Rights Reserved
package murmur3

import (
	"hash"
	"testing"
	_ "fmt"
)

var tests = []struct{
	hash	uint32
	hex		bool
	s		string
}{
	{0x00000000, false, ""},
	{0x3c2569b2, false, "a"},
	{0x4f31114c, false, "bc"},
	{0xf5797de2, false, "def"},
	{0x13704969, false, "ghij"},
	{0x248bfa47, false, "hello"},
	{0x149bbb7f, false, "hello, world"},
	{0xe31e8a70, false, "19 Jan 2038 at 3:14:07 AM"},
	{0xd5c48bfc, false, "The quick brown fox jumps over the lazy dog."},
	{0xb61e6dcc, true,	"daf1596449909da0dde3987638909728"},				// same hash with all seeds!
}

func unhex(c byte) uint8 {
        switch {
        case '0' <= c && c <= '9':
                return c - '0'
        case 'a' <= c && c <= 'f':
                return c - 'a' + 10
        case 'A' <= c && c <= 'F':
                return c - 'A' + 10
        }
        panic("unhex: bad input")
}

func hexToBytes(s string) []byte {
        var data = make([]byte, 1000, 1000)
        data = data[0:len(s)/2]

        n := len(s)
        if (n&1) == 1 {
                panic("gethex: string must be even")
        }
        j = len()
        for i := range data {
                data[i] = unhex(s[2*i])<<4 | unhex(s[2*i+1])
        }
        //fmt.Printf("hexToBytes: len(data)=%d, len(s)=%d\n", len(data), len(s))
        return data[0:len(s)/2]
}

func TestRef(t *testing.T) {
	var h32 hash.Hash32 = New(0)
	for _, elem := range tests {
		h32.Reset()
		b := []byte(elem.s)
		if elem.hex {
			b = hexToBytes(elem.s)
		}
		h32.Write(b)
		//fmt.Printf("TestRef: %q, len=%d\n", elem.s, len(string(elem.s)))
		if v := h32.Sum32(); v != elem.hash {
			t.Errorf("h32.Sum32: %q 0x%x (want 0x%x)", elem.s, v, elem.hash)
		}

		if v := Sum32(b, 0); v != elem.hash {
			t.Errorf("Sum32: %q 0x%x (want 0x%x)", elem.s, v, elem.hash)
		}

	}
}

//---

func bench32(b *testing.B, length int) {
	buf := make([]byte, length)
	b.SetBytes(int64(length))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Sum32(buf, 0)
	}
}

func Benchmark32_1(b *testing.B) {
	bench32(b, 1)
}
func Benchmark32_2(b *testing.B) {
	bench32(b, 2)
}
func Benchmark32_4(b *testing.B) {
	bench32(b, 4)
}
func Benchmark32_8(b *testing.B) {
	bench32(b, 8)
}
func Benchmark32_16(b *testing.B) {
	bench32(b, 16)
}
func Benchmark32_32(b *testing.B) {
	bench32(b, 32)
}
func Benchmark32_64(b *testing.B) {
	bench32(b, 64)
}
func Benchmark32_128(b *testing.B) {
	bench32(b, 128)
}
func Benchmark32_256(b *testing.B) {
	bench32(b, 256)
}
func Benchmark32_512(b *testing.B) {
	bench32(b, 512)
}
func Benchmark32_1024(b *testing.B) {
	bench32(b, 1024)
}
func Benchmark32_2048(b *testing.B) {
	bench32(b, 2048)
}
func Benchmark32_4096(b *testing.B) {
	bench32(b, 4096)
}
func Benchmark32_8192(b *testing.B) {
	bench32(b, 8192)
}

//---

func benchPartial32(b *testing.B, length int) {
	buf := make([]byte, length)
	b.SetBytes(int64(length))

	start := (32 / 8) / 2
	chunks := 7
	k := length / chunks
	tail := (length - start) % k

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasher := New(0)
		hasher.Write(buf[0:start])

		for j := start; j+k <= length; j += k {
			hasher.Write(buf[j : j+k])
		}

		hasher.Write(buf[length-tail:])
		hasher.Sum32()
	}
}

func BenchmarkPartial32_8(b *testing.B) {
	benchPartial32(b, 8)
}
func BenchmarkPartial32_16(b *testing.B) {
	benchPartial32(b, 16)
}
func BenchmarkPartial32_32(b *testing.B) {
	benchPartial32(b, 32)
}
func BenchmarkPartial32_64(b *testing.B) {
	benchPartial32(b, 64)
}
func BenchmarkPartial32_128(b *testing.B) {
	benchPartial32(b, 128)
}


/*
//     hasher := New32()
//     hasher.Write(data)
//     return hasher.Sum32()
// 	{0x248bfa47, 0xcbd8a7b341bd9b02, 0x5b1e906a48ae1d19, "hello"},
func main() {
	var h32 hash.Hash32 = murmur3.New32()

	//value := "now is the time for all good men to come to the aid of their country"
	value := "hello"
	fmt.Printf("hash=0x%x\n", 0x248bfa47)
	for i := 0; i < 10; i++ {
		h := murmur332([]byte(value), uint32(i))
		fmt.Printf("%d: hash=0x%x\n", i, h)
	}

	h32.Write([]byte(value))
	h := h32.Sum32()
	fmt.Printf("hash=0x%x\n", h)
}
*/