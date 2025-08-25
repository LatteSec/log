package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var ErrSkipClose = errors.New("skip running closing")

// Loggers use LogHandlers under the hood
// to handle log messages
type LogHandler interface {
	Handle(loggerName string, msg *LogMessage) // handles the log message

	Start() error    // starts the handler
	Close() error    // closes the handler
	IsRunning() bool // returns true if the handler is running
}

// Start() -> StartFunc() -> Subprocess -> CancelPreFunc() -> CancelPostFunc() -> OnSigint() -> CloseFunc()
type BaseHandler struct {
	LogHandler

	mu     sync.RWMutex
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	logCh  chan *LogMessage

	running   bool
	cleanupId uint64

	HandleFunc func(context.Context, *LogMessage) error

	StartFunc      func(context.Context, LogHandler) error
	CancelPreFunc  func(context.Context, LogHandler) error // runs before ctx.cancel is executed, return ErrSkipClose to skip
	CancelPostFunc func(context.Context, LogHandler) error
	CloseFunc      func(context.Context, LogHandler) error
	Subprocesses   []func(context.Context) error // processes must terminate when ctx is done
}

func (b *BaseHandler) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

func (b *BaseHandler) Start() error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return ErrAlreadyStarted
	}

	if b.HandleFunc == nil {
		b.mu.Unlock()
		return ErrInvalidLogHandler
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.logCh = make(chan *LogMessage, 1<<10)
	b.running = true
	b.wg.Add(len(b.Subprocesses) + 1)

	if b.StartFunc != nil {
		if err := b.StartFunc(b.ctx, b); err != nil {
			b.mu.Unlock()
			return err
		}
	}

	b.cleanupId = registerCleanup(func() error {
		b.mu.Lock()
		defer b.mu.Unlock()

		var err error
		if b.CancelPreFunc != nil {
			err = b.CancelPreFunc(b.ctx, b)
		}
		return errors.Join(err, b.close())
	})
	b.mu.Unlock()

	queue := make(chan struct{})
	ready := make(chan struct{})
	go noPanicRunVoid("log-handler", func() {
		defer b.wg.Done()
		<-queue

		b.logHandler(ready)
	})

	for i, f := range b.Subprocesses {
		i, f := i, f
		go noPanicRunVoid(fmt.Sprintf("log-handler:proc#%d", i), func() {
			defer b.wg.Done()
			<-queue

			if err := f(b.ctx); err != nil {
				fmt.Fprintf(os.Stderr, "error in logger subprocess: %v\n", err)
			}
		})
	}

	close(queue)
	<-ready
	return nil
}

// callers responsiblity to hold a lock
func (b *BaseHandler) close() error {
	if !b.running {
		return ErrNotStarted
	}

	var errs []error
	if b.CancelPreFunc != nil {
		err := b.CancelPreFunc(b.ctx, b)
		if err != nil && errors.Is(err, ErrSkipClose) {
			return nil
		}
		errs = append(errs, err)
	}

	b.running = false
	if b.cleanupId != 0 {
		unregisterCleanup(b.cleanupId)
		b.cleanupId = 0
	}

	b.cancel()

	if b.CancelPostFunc != nil {
		errs = append(errs, b.CancelPostFunc(b.ctx, b))
	}

	b.wg.Wait()

	if b.CloseFunc != nil {
		errs = append(errs, b.CloseFunc(b.ctx, b))
	}

	close(b.logCh)
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (b *BaseHandler) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.close()
}

func (b *BaseHandler) Handle(loggerName string, msg *LogMessage) {
	if msg == nil {
		return
	}

	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()
	if !running {
		return
	}

	select {
	case <-b.ctx.Done():
		return
	case b.logCh <- acquireLogMessage(loggerName, msg):
	default: // drop
	}
}

func (b *BaseHandler) logHandler(ready chan struct{}) {
	close(ready)
	for {
		select {
		case <-b.ctx.Done(): // drain
			for {
				select {
				case m := <-b.logCh:
					_ = noPanicRun("flush-log-msg", func() error {
						return b.HandleFunc(context.Background(), m)
					})
					releaseLogMessage(m)
				default:
					return
				}
			}

		case m := <-b.logCh:
			err := noPanicRun("write-log-msg", func() error {
				return b.HandleFunc(b.ctx, m)
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error in logger: %v\n", err)
			}
			releaseLogMessage(m)
		}
	}
}

type WriterHandler struct {
	BaseHandler
	writer io.Writer
}

func NewWriterHandler(writer io.Writer) *WriterHandler {
	wr := &WriterHandler{writer: writer}

	wr.BaseHandler = BaseHandler{
		HandleFunc: func(ctx context.Context, msg *LogMessage) (err error) {
			_, err = fmt.Fprint(wr.writer, msg.String(""))
			return
		},
		CloseFunc: func(ctx context.Context, h LogHandler) error {
			if wr.writer != os.Stdout && wr.writer != os.Stderr {
				if closer, ok := wr.writer.(io.Closer); ok {
					return closer.Close()
				}
			}
			return nil
		},
	}

	return wr
}

func (wh *WriterHandler) Writer() io.Writer {
	wh.mu.RLock()
	defer wh.mu.RUnlock()
	return wh.writer
}

func (wh *WriterHandler) SetWriter(writer io.Writer) {
	wh.mu.Lock()
	defer wh.mu.Unlock()
	wh.writer = writer
}
