package log

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type cleanupFunc func() error

var (
	once sync.Once

	muCleanup    sync.Mutex
	cleanupIdGen uint64
	cleanupFns   = make(map[uint64]cleanupFunc)
)

// Register registers a cleanup function
// that is called on exit
func registerCleanup(fn cleanupFunc) uint64 {
	id := atomic.AddUint64(&cleanupIdGen, 1)
	muCleanup.Lock()
	cleanupFns[id] = fn
	muCleanup.Unlock()
	return id
}

func unregisterCleanup(id uint64) {
	muCleanup.Lock()
	delete(cleanupFns, id)
	muCleanup.Unlock()
}

func runCleanup() {
	muCleanup.Lock()
	fns := make([]cleanupFunc, 0, len(cleanupFns))
	for _, fn := range cleanupFns {
		fns = append(fns, fn)
	}
	cleanupFns = make(map[uint64]cleanupFunc)
	atomic.StoreUint64(&cleanupIdGen, 0)
	muCleanup.Unlock()
	for i, fn := range fns {
		name := fmt.Sprintf("cleanup %d", i)
		if err := noPanicRun(name, fn); err != nil {
			fmt.Fprintf(os.Stderr, "%s failed: %v\n", name, err)
		}
	}
}

func handleSigint() {
	once.Do(func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		runCleanup()
	})
}
