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

	defaultLogger        atomic.Pointer[Logger]
	DefaultStdoutHandler atomic.Pointer[WriterHandler]
	DefaultStderrHandler atomic.Pointer[WriterHandler]

	ErrNotStarted                = errors.New("not started")
	ErrAlreadyStarted            = errors.New("already started")
	ErrInvalidLogHandler         = errors.New("invalid log handler")
	ErrInvalidLogLevel           = errors.New("invalid log level")
	ErrInvalidMaxFileSize        = errors.New("invalid max file size")
	ErrMissingLogFilename        = errors.New("missing log filename")
	ErrNoLogFileConfigured       = errors.New("no log file configured")
	ErrFoundDirWhenExpectingFile = errors.New("found directory when expecting file")
)

func init() {
	go handleSigint()

	if err := RegisterStdoutHandler(NewWriterHandler(os.Stdout)); err != nil {
		panic(err)
	}
	if err := RegisterStderrHandler(NewWriterHandler(os.Stderr)); err != nil {
		panic(err)
	}
}

func DefaultLogger() *Logger {
	logger := defaultLogger.Load()
	if logger != nil {
		return logger
	}

	logger, err := NewLogger().Name("default").Build()
	if err != nil {
		panic(fmt.Errorf("could not build default logger: %v", err))
	}
	if err := logger.Start(); err != nil {
		panic(fmt.Errorf("could not start default logger: %v", err))
	}

	defaultLogger.Store(logger)
	logger.Info().Msg("default logger started").Send()

	return logger
}

func Sync() {
	runCleanup()
}

func Register(l *Logger) {
	defaultLogger.Store(l)
}

func RegisterStdoutHandler(handler *WriterHandler) error {
	if err := handler.Start(); err != nil && err != ErrAlreadyStarted {
		return err
	}
	DefaultStdoutHandler.Store(handler)
	return nil
}

func RegisterStderrHandler(handler *WriterHandler) error {
	if err := handler.Start(); err != nil && err != ErrAlreadyStarted {
		return err
	}
	DefaultStderrHandler.Store(handler)
	return nil
}

func Log(level Level) *LogMessage { return DefaultLogger().Log(level) }
func Debug() *LogMessage          { return DefaultLogger().Debug() }
func Info() *LogMessage           { return DefaultLogger().Info() }
func Warn() *LogMessage           { return DefaultLogger().Warn() }
func Error() *LogMessage          { return DefaultLogger().Error() }
func Fatal() *LogMessage          { return DefaultLogger().Fatal() }
