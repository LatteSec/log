package log

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var fileHandlers = sync.Map{} // map[string]*refCountedFileHandler

type refCountedFileHandler struct {
	handler *FileHandler
	count   int32
}

// NewFileHandler creates a new FileHandler and starts it.
func NewFileHandler(path string) (*FileHandler, error) {
	return newRefCounted(newFileHandler(path))
}

func newRefCounted(fh *FileHandler) (*FileHandler, error) {
	newPth := filepath.Join(fh.logDir, fh.logFilename)
	val, _ := fileHandlers.LoadOrStore(newPth, &refCountedFileHandler{
		handler: fh,
	})

	var once sync.Once

	rh := val.(*refCountedFileHandler)
	rh.handler.release = func() bool {
		if atomic.AddInt32(&rh.count, -1) == 0 {
			once.Do(func() {
				rh.handler.muFile.Lock()
				defer rh.handler.muFile.Unlock()

				rh.handler.running = false
			})
			return true
		}

		return false
	}

	var once2 sync.Once
	rh.handler.onRelease = func() {
		once2.Do(func() {
			rh.handler.muFile.Lock()
			defer rh.handler.muFile.Unlock()

			ptr := rh.handler.filePtr
			if ptr != nil {
				_ = ptr.Sync()
				_ = ptr.Close()
				rh.handler.filePtr = nil
			}

			fileHandlers.Delete(newPth)
		})
	}

	if atomic.AddInt32(&rh.count, 1) == 1 {
		if err := rh.handler.Start(); err != nil && !errors.Is(err, ErrAlreadyStarted) {
			fileHandlers.Delete(newPth)
			return nil, err
		}
	}

	return rh.handler, nil
}

type FileHandler struct {
	BaseHandler

	muFile sync.Mutex // covers filePtr and logCh

	logDir      string
	logFilename string
	filePtr     *os.File
	maxFileSize int64 // exceeding this size will trigger log rotation. defaults to 10MB. set to 0 to disable

	release   func() bool // returns true if the handler is no longer in use
	onRelease func()
}

func newFileHandler(path string) *FileHandler {
	f := &FileHandler{
		logDir:      filepath.Dir(path),
		logFilename: filepath.Base(path),
	}

	f.BaseHandler = BaseHandler{
		CancelPreFunc: func(ctx context.Context, lh LogHandler) error {
			if f.release != nil {
				if !f.release() {
					return ErrSkipClose
				}
				f.release = nil
			}
			return nil
		},
		CloseFunc: func(ctx context.Context, lh LogHandler) error {
			if f.onRelease != nil {
				f.onRelease()
			}
			return nil
		},
		StartFunc: func(ctx context.Context, lh LogHandler) error {
			_, base := f.getLogfileLocation()
			if base == "." {
				return ErrMissingLogFilename
			}

			logfile, err := f.ensureLogFile()
			if err != nil {
				return fmt.Errorf("failed to open log file: %w", err)
			}
			f.filePtr = logfile

			return nil
		},
		HandleFunc: func(ctx context.Context, msg *LogMessage) error {
			f.muFile.Lock()
			defer f.muFile.Unlock()

			if f.filePtr == nil {
				panic("FileHandler: filePtr is nil")
			}

			_, err := f.filePtr.WriteString(msg.String(""))
			if err != nil {
				return err
			}
			return nil
		},
		Subprocesses: []func(context.Context) error{f.logRotater},
	}

	return f
}

func (f *FileHandler) logRotater(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			maxFilesize := f.GetMaxFileSize()
			if maxFilesize == 0 {
				return nil
			}

			logDir, logFilename := f.GetLogfileLocation()
			logPath := filepath.Join(logDir, logFilename)
			rotatedName := fmt.Sprintf("%s-%s.gz", logFilename, time.Now().UTC().Format("2006-01-02_15-04-05"))
			rotatedPath := filepath.Join(logDir, rotatedName)

			info, err := os.Stat(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					f.muFile.Lock()
					_, err := f.ensureLogFile()
					f.muFile.Unlock()

					if err != nil {
						return fmt.Errorf("failed to recreate missing log file, killing rotation: %w", err)
					}

					continue
				}

				f.wg.Done()
				return fmt.Errorf("failed to stat log file, killing rotation: %w", err)
			}

			if info.Size() <= maxFilesize {
				continue
			}

			f.muFile.Lock()
			original, err := os.Open(filepath.Clean(logPath))
			if err != nil {
				f.muFile.Unlock()
				Error().Msgf("failed to open log for rotation: %v", err).Send()
				continue
			}

			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			_, err = io.Copy(gz, original)
			_ = original.Close()
			_ = gz.Close()
			if err != nil {
				f.muFile.Unlock()
				Error().Msgf("failed to compress rotated log: %v", err).Send()
				continue
			}

			if err := os.WriteFile(rotatedPath, buf.Bytes(), 0o600); err != nil {
				f.muFile.Unlock()
				Error().Msgf("failed to write rotated log file: %v", err).Send()
				continue
			}

			if err := os.Truncate(logPath, 0); err != nil {
				Error().Msgf("failed to truncate original log after rotation: %v", err).Send()
			}

			f.muFile.Unlock()
		}
	}
}

func (f *FileHandler) getLogfileLocation() (dir, base string) {
	return f.logDir, f.logFilename
}

func (f *FileHandler) GetLogfileLocation() (dir, base string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.getLogfileLocation()
}

func (f *FileHandler) SetMaxFileSize(size int64) {
	f.mu.Lock()
	f.maxFileSize = size
	f.mu.Unlock()
}

func (f *FileHandler) GetMaxFileSize() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.maxFileSize
}

func (f *FileHandler) SetLogfileLocation(dir, base string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := filepath.Join(dir, base)
	if path == "." {
		return ErrMissingLogFilename
	}
	path = strings.TrimSuffix(path, ".log") + ".log"

	f.logDir, f.logFilename = filepath.Split(path)
	return nil
}

func (f *FileHandler) ensureLogDir() error {
	if f.logDir == "." {
		return nil
	}

	return os.MkdirAll(filepath.Clean(f.logDir), 0o700)
}

func (f *FileHandler) ensureLogFile() (*os.File, error) {
	if f.logFilename == "." {
		return nil, ErrNoLogFileConfigured
	}
	if err := f.ensureLogDir(); err != nil {
		return nil, err
	}

	logfileLocation := filepath.Join(f.logDir, f.logFilename)
	if logfileLocation == "." {
		return nil, ErrNoLogFileConfigured
	}

	stat, err := os.Stat(logfileLocation)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) {
		return f.openLogFile()
	}

	if stat.IsDir() {
		return nil, ErrFoundDirWhenExpectingFile
	}

	return f.openLogFile()
}

func (f *FileHandler) openLogFile() (*os.File, error) {
	return os.OpenFile(
		filepath.Join(f.logDir, f.logFilename),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o600,
	)
}
