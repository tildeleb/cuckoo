package main

import (
	"log"

	"leb.io/cuckoo/internal/siginfo"
)

func main() {
	var f = func() {
		log.Printf("You pressed ^T")
	}
	siginfo.SetHandler(f)
	select {}
}

/*
func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, SIGINFO)

	go func() {
		for _ = range ch {
			// f()
			log.Printf("You pressed ^T")
		}
	}()

	select {}
}*/
