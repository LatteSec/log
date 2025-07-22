package log

import (
	"fmt"
	"os"
	"time"
)

func run[T any](name string, rerun bool, fn func() T) (out T) {
	for {
		var panicked bool

		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "panic in %s: %v\n", name, r)
					panicked = true
				}
			}()

			out = fn()
		}()

		if !panicked || !rerun {
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func noPanicRun[T any](name string, fn func() T) (out T) {
	return run(name, false, fn)
}

func noPanicReRunVoid(name string, fn func()) {
	run(name, true, func() any {
		fn()
		return nil
	})
}
