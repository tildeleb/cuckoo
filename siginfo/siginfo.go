package siginfo

import (
	_ "log"
	"os"
	"os/signal"
	"syscall"
)

// SIGINFO isn't part of the stdlib, but it's 29 on most systems
const SIGINFO = syscall.Signal(29)

func SetHandler(f func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, SIGINFO)

	go func() {
		for _ = range ch {
			f()
		}
	}()
}
