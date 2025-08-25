package log

import (
	"errors"
	"os"
	"sync"
)

type ILogger interface {
	Start() error
	Close() error

	GetName() string
	SetName(name string)

	GetLevel() Level
	SetLevel(level Level) error

	IsRunning() bool
	Stdout(on bool)
	Stderr(on bool)

	SendLog(msg *LogMessage)

	Log() *LogMessage
	Debug() *LogMessage
	Info() *LogMessage
	Warn() *LogMessage
	Error() *LogMessage
	Fatal() *LogMessage
}

type Logger struct {
	LoggerMeta
	mu      sync.RWMutex
	running bool
}

func (l *Logger) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return ErrAlreadyStarted
	}

	l.running = true
	for _, h := range l.handlers {
		if err := h.Start(); err != nil && err != ErrAlreadyStarted {
			return err
		}
	}
	return nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	l.running = false
	errs := []error{}
	for _, h := range l.handlers {
		if err := h.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (l *Logger) GetName() string     { l.mu.RLock(); defer l.mu.RUnlock(); return l.name }
func (l *Logger) SetName(name string) { l.mu.Lock(); defer l.mu.Unlock(); l.name = name }

func (l *Logger) GetLevel() Level { l.mu.RLock(); defer l.mu.RUnlock(); return l.level }
func (l *Logger) SetLevel(level Level) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if level < TRACE || level > QUIET {
		return ErrInvalidLogLevel
	}
	l.level = level
	return nil
}

func (l *Logger) IsRunning() bool { l.mu.RLock(); defer l.mu.RUnlock(); return l.running }
func (l *Logger) Stdout(on bool)  { l.mu.Lock(); defer l.mu.Unlock(); l.stdoutEnabled = on }
func (l *Logger) Stderr(on bool)  { l.mu.Lock(); defer l.mu.Unlock(); l.stderrEnabled = on }

func (l *Logger) SendLog(msg *LogMessage) {
	l.mu.RLock()
	if msg.Level < l.level {
		l.mu.RUnlock()
		return
	}

	if (l.level == TRACE && msg.Level >= ERROR) || msg.Level == TRACE {
		if msg.trace == "" {
			msg.WithTraceStack()
		}
		if msg.caller == "" {
			msg.WithCaller()
		}
	}

	shouldWriteToStd := l.level != QUIET
	name := l.name
	l.mu.RUnlock()

	if shouldWriteToStd {
		if msg.Level >= WARN && l.stderrEnabled {
			DefaultStderrHandler.Load().Handle(name, msg)
		} else if l.stdoutEnabled {
			DefaultStdoutHandler.Load().Handle(name, msg)
		}
	}

	for _, h := range l.handlers {
		h.Handle(name, msg)
	}
}

func (l *Logger) Log(level Level) *LogMessage {
	lm := NewLogMessage().WithSend(l.SendLog)
	lm.Level = level
	return lm
}
func (l *Logger) Debug() *LogMessage { return NewLogMessage().Debug().WithSend(l.SendLog) }
func (l *Logger) Info() *LogMessage  { return NewLogMessage().Info().WithSend(l.SendLog) }
func (l *Logger) Warn() *LogMessage  { return NewLogMessage().Warn().WithSend(l.SendLog) }
func (l *Logger) Error() *LogMessage { return NewLogMessage().Error().WithSend(l.SendLog) }
func (l *Logger) Fatal() *LogMessage {
	return NewLogMessage().Fatal().WithSend(func(lm *LogMessage) {
		l.SendLog(lm)

		l.mu.RLock()
		if l.cleanup != nil {
			for _, cleanup := range l.cleanup {
				cleanup()
			}
		}
		l.mu.RUnlock()

		runCleanup()
		os.Exit(1)
	})
}
