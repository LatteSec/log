package log

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
)

// Log Level
type Level int

// Log Levels
//
// Arranged from most to least verbose
const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
	QUIET
)

var (
	levelNames = [6]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "QUIET"}

	DefaultLogger        atomic.Pointer[Logger]
	DefaultStdoutHandler atomic.Pointer[WriterHandler]
	DefaultStderrHandler atomic.Pointer[WriterHandler]

	ErrNotStarted                = errors.New("not started")
	ErrAlreadyStarted            = errors.New("already started")
	ErrInvalidLogLevel           = errors.New("invalid log level")
	ErrInvalidMaxFileSize        = errors.New("invalid max file size")
	ErrMissingLogFilename        = errors.New("missing log filename")
	ErrNoLogFileConfigured       = errors.New("no log file configured")
	ErrFoundDirWhenExpectingFile = errors.New("found directory when expecting file")
)

func init() {
	go handleSigint()

	DefaultStdoutHandler.Store(&WriterHandler{writer: os.Stdout})
	DefaultStderrHandler.Store(&WriterHandler{writer: os.Stderr})
}

func Register(l *Logger) {
	DefaultLogger.Store(l)
}

func log(msg *LogMessage) {
	logger := DefaultLogger.Load()
	if logger == nil {
		newLogger, _ := NewLogger().Name("default").Build()
		if err := newLogger.Start(); err != nil {
			panic(fmt.Errorf("could not start default logger: %v", err))
		}

		DefaultLogger.Store(newLogger)
		logger = newLogger
		logger.Info().Msg("default logger started").Send()
	}

	logger.SendLog(msg)
}

func newGLobalLogMessage() *LogMessage {
	return NewLogMessage().WithSend(log)
}

func Log(level Level) *LogMessage { return newGLobalLogMessage().WithLevel(level) }
func Debug() *LogMessage          { return newGLobalLogMessage().Debug() }
func Info() *LogMessage           { return newGLobalLogMessage().Info() }
func Warn() *LogMessage           { return newGLobalLogMessage().Warn() }
func Error() *LogMessage          { return newGLobalLogMessage().Error() }
func Fatal() *LogMessage          { return newGLobalLogMessage().Fatal() }
