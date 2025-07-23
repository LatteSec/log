package log

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Loggers use LogHandlers under the hood
// to handle log messages
type LogHandler interface {
	Handle(loggerName string, msg *LogMessage) // handles the log message
	FormatLog(msg *LogMessage) string          // formats the log message

	Start() error    // starts the handler
	Close() error    // closes the handler
	IsRunning() bool // returns true if the handler is running
}

type BaseHandler struct {
	LogHandler
}

func (b *BaseHandler) FormatLog(msg *LogMessage) string {
	return msg.String("")
}

type WriterHandler struct {
	BaseHandler

	wg      sync.WaitGroup
	mu      sync.RWMutex
	logCh   chan *LogMessage
	closeCh chan struct{}

	writer    io.Writer
	cleanupId uint64
}

func NewWriterHandler(writer io.Writer) *WriterHandler {
	return &WriterHandler{writer: writer}
}

func (w *WriterHandler) Handle(loggerName string, msg *LogMessage) {
	if !w.IsRunning() {
		return
	}

	select {
	case w.logCh <- acquireLogMessage(loggerName, msg):
	default: // drop
	}
}

func (w *WriterHandler) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.logCh != nil {
		return ErrAlreadyStarted
	}
	if w.writer == nil {
		w.writer = io.Discard
	}

	w.logCh = make(chan *LogMessage, 1<<10)
	w.closeCh = make(chan struct{})
	w.cleanupId = registerCleanup(w.Close)

	w.wg.Add(1)
	go noPanicReRunVoid("log-handler", w.logWriter)

	return nil
}

func (w *WriterHandler) Close() error {
	if !w.IsRunning() {
		return ErrNotStarted
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	var err error
	if w.writer != os.Stdout && w.writer != os.Stderr {
		if closer, ok := w.writer.(io.Closer); ok {
			err = closer.Close()
		}
	}

	unregisterCleanup(w.cleanupId)
	w.cleanupId = 0

	close(w.closeCh)
	w.logCh = nil
	w.wg.Wait()

	return err
}

func (i *WriterHandler) IsRunning() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.logCh != nil
}

func (w *WriterHandler) logWriter() {
	for {
		select {
		case msg := <-w.logCh:
			fmt.Fprint(w.writer, w.FormatLog(msg))
			releaseLogMessage(msg)
		case <-w.closeCh:
			w.wg.Done()
			return
		}
	}
}
