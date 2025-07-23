package log

import (
	"bytes"
	"compress/gzip"
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

func NewFileHandler(path string) (*FileHandler, error) {
	val, _ := fileHandlers.LoadOrStore(path, &refCountedFileHandler{
		handler: newFileHandler(path),
	})

	rh := val.(*refCountedFileHandler)
	rh.handler.release = func() {
		if atomic.AddInt32(&rh.count, -1) == 0 {
			fileHandlers.Delete(path)
		}
	}

	if atomic.AddInt32(&rh.count, 1) == 1 {
		if err := rh.handler.Start(); err != nil && !errors.Is(err, ErrAlreadyStarted) {
			fileHandlers.Delete(path)
			return nil, err
		}
	}
	return rh.handler, nil
}

type FileHandler struct {
	WriterHandler

	muFile sync.RWMutex // covers filePtr and logCh

	logDir      string
	logFilename string
	filePtr     atomic.Pointer[os.File]
	maxFileSize int64 // exceeding this size will trigger log rotation. defaults to 10MB. set to 0 to disable

	release func()
}

func newFileHandler(path string) *FileHandler {
	return &FileHandler{
		logDir:      filepath.Dir(path),
		logFilename: filepath.Base(path),
	}
}

func (f *FileHandler) Start() error {
	_, base := f.GetLogfileLocation()
	if base == "." {
		return ErrMissingLogFilename
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.logCh != nil {
		return ErrAlreadyStarted
	}

	f.closeCh = make(chan struct{})
	f.cleanupId = registerCleanup(f.Close)

	f.wg.Add(2)
	go noPanicReRunVoid(base+"-log-handler", f.logWriter)
	go noPanicReRunVoid(base+"-log-rotater", f.logRotater)
	return nil
}

func (f *FileHandler) Close() error {
	f.muFile.Lock()
	defer f.muFile.Unlock()

	ptr := f.filePtr.Load()
	if ptr != nil {
		_ = ptr.Close()
		f.filePtr.Store(nil)
	}

	if f.release != nil {
		f.release()
		f.release = nil
	}

	return f.WriterHandler.Close()
}

func (f *FileHandler) logWriter() {
	f.muFile.Lock()
	if f.logCh == nil {
		f.logCh = make(chan *LogMessage, 1<<10)
	}
	f.muFile.Unlock()

	logfile, err := f.ensureLogFile()
	if err != nil {
		Error().Msgf("failed to open log file: %v", err)
		f.wg.Done()
		return
	}
	f.filePtr.Store(logfile)

	for {
		select {
		case msg := <-f.logCh:
			l := f.FormatLog(msg)
			f.muFile.Lock()
			_, err := f.filePtr.Load().WriteString(l)
			f.muFile.Unlock()
			releaseLogMessage(msg)

			if err != nil {
				Error().Msgf("failed to write to log file: %v", err).Send()
				f.wg.Done()
				return
			}
		case <-f.closeCh:
			f.wg.Done()
			return
		}
	}
}

func (f *FileHandler) logRotater() {
	f.mu.RLock()
	if f.maxFileSize == 0 {
		f.mu.RUnlock()
		f.wg.Done()
		return
	}
	f.mu.RUnlock()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.mu.RLock()
			logPath := filepath.Join(f.logDir, f.logFilename)
			f.mu.RUnlock()

			info, err := os.Stat(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					if _, err := f.ensureLogFile(); err != nil {
						Error().Msgf("failed to recreate missing log file, killing rotation: %v", err).Send()
						f.wg.Done()
						return
					}
					continue
				}

				Error().Msgf("failed to stat log file, killing rotation: %v", err).Send()
				f.wg.Done()
				return
			}

			if info.Size() <= f.maxFileSize {
				continue
			}

			f.muFile.Lock()

			rotatedName := fmt.Sprintf("%s-%s.gz", f.logFilename, time.Now().UTC().Format("2006-01-02_15-04-05"))
			rotatedPath := filepath.Join(f.logDir, rotatedName)

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

		case <-f.closeCh:
			f.wg.Done()
			return
		}
	}
}

func (f *FileHandler) GetLogfileLocation() (dir, base string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.logDir, f.logFilename
}

func (f *FileHandler) SetLogfileLocation(dir, base string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	dir = filepath.Clean(dir)
	base = filepath.Clean(strings.TrimSuffix(filepath.Base(base), ".log"))

	if dir != "." && base == "." {
		return ErrMissingLogFilename
	}
	if base != "." {
		base += ".log"
	}

	f.logDir = dir
	f.logFilename = base

	return nil
}

func (f *FileHandler) ensureLogDir() error {
	if f.logDir == "." {
		return nil
	}

	return os.MkdirAll(filepath.Clean(f.logDir), 0o700)
}

func (f *FileHandler) ensureLogFile() (*os.File, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

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
