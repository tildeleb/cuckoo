package jenkins3_test

func main() {
	q := "This is the time for all good men to come to the aid of their country..."
	//qq := []byte{"xThis is the time for all good men to come to the aid of their country..."}
	//qqq := []byte{"xxThis is the time for all good men to come to the aid of their country..."}
	//qqqq[] := []byte{"xxxThis is the time for all good men to come to the aid of their country..."}

	u := stu(q)
	h1 := hashword(u, (len(q)-1)/4, 13)
	h2 := hashword(u, (len(q)-5)/4, 13)
	h3 := hashword(u, (len(q)-9)/4, 13)
	fmt.Printf("%08x, %0x8, %08x\n", h1, h2, h3)

	b, c := uint32(0), uint32(0)
	c, b = hashlittle2("", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// deadbeef deadbeef

	b, c = 0xdeadbeef, 0
	c, b = hashlittle2("", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// bd5b7dde deadbeef

  	b, c = 0xdeadbeef, 0xdeadbeef
	c, b = hashlittle2("", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// 9c093ccd bd5b7dde

	b, c = 0, 0
	c, b = hashlittle2("Four score and seven years ago", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// 17770551 ce7226e6

	b, c = 1, 0
	c, b = hashlittle2("Four score and seven years ago", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// e3607cae bd371de4

	b, c = 0, 1
	c, b = hashlittle2("Four score and seven years ago", c, b)
	fmt.Printf("%08x, %08x\n", c, b)	// cd628161 6cbea4b3
}
