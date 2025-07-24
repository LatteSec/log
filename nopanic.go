package log

import (
	"fmt"
	"os"
)

func run[T any](name string, fn func() T) (out T) {
	defer func() {
		if r := recover(); r != nil {
			if o, ok := r.(T); ok {
				out = o
				return
			}
			fmt.Fprintf(os.Stderr, "panic in %s: %v\n", name, r)
		}
	}()

	out = fn()
	return
}

func noPanicRun[T any](name string, fn func() T) (out T) {
	return run(name, fn)
}

func noPanicRunVoid(name string, fn func()) {
	run(name, func() any {
		fn()
		return nil
	})
}
