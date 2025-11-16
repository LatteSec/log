package log

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type LogMessageMetaKV struct {
	K, V string
}

type LogMessage struct {
	Timestamp time.Time          // timestamp
	Level     Level              // log level
	Message   string             // log message
	Meta      []LogMessageMetaKV // log metadata

	trace  string // stack trace (optional)
	caller string // caller (optional)

	loggerName string // used only in log handlers, meaningless otherwise

	send func(*LogMessage)
}

// NewLogMessage
//
// Creates a new LogMessage
func NewLogMessage() *LogMessage {
	return &LogMessage{
		Timestamp: time.Now().UTC(),
		Meta:      make([]LogMessageMetaKV, 0, 1),
	}
}

func (lm *LogMessage) WithSend(send func(*LogMessage)) *LogMessage { lm.send = send; return lm }
func (lm *LogMessage) Send() {
	if err := lm.SendE(); err != nil {
		panic(err)
	}
}

func (lm *LogMessage) SendE() error {
	if lm.send == nil {
		return fmt.Errorf("LogMessage.SendE: no send function set")
	}
	lm.send(lm)
	return nil
}

func (lm *LogMessage) WithMeta(key string, value any) *LogMessage {
	lm.Meta = append(lm.Meta, LogMessageMetaKV{K: key, V: fmt.Sprintf("%v", value)})
	return lm
}

func (lm *LogMessage) WithMetaf(key, format string, v ...any) *LogMessage {
	lm.Meta = append(lm.Meta, LogMessageMetaKV{K: key, V: fmt.Sprintf(format, v...)})
	return lm
}

func (lm *LogMessage) WithTraceStack() *LogMessage {
	lm.trace = traceStack()
	return lm
}

func (lm *LogMessage) WithCaller() *LogMessage {
	lm.caller = traceCaller()
	return lm
}

func (lm *LogMessage) WithLevel(level Level) *LogMessage { lm.Level = level; return lm }
func (lm *LogMessage) LevelString() string               { return levelNames[lm.Level] }

func (lm *LogMessage) Msg(msg ...any) *LogMessage { lm.Message = fmt.Sprint(msg...); return lm }
func (lm *LogMessage) Msgf(format string, v ...any) *LogMessage {
	lm.Message = fmt.Sprintf(format, v...)
	return lm
}

func (lm *LogMessage) Debug() *LogMessage { return lm.WithLevel(DEBUG) }
func (lm *LogMessage) Info() *LogMessage  { return lm.WithLevel(INFO) }
func (lm *LogMessage) Warn() *LogMessage  { return lm.WithLevel(WARN) }
func (lm *LogMessage) Error() *LogMessage { return lm.WithLevel(ERROR) }
func (lm *LogMessage) Fatal() *LogMessage { return lm.WithLevel(ERROR) }

func (lm *LogMessage) String(loggerName string) string {
	var metaStr string
	if len(lm.Meta) > 0 {
		meta := make([]string, 0, len(lm.Meta))
		for _, m := range lm.Meta {
			meta = append(meta, fmt.Sprintf("%s=%s", m.K, m.V))
		}
		metaStr = fmt.Sprintf(" {%s}", strings.Join(meta, ", "))
	}

	var debugStr string
	if lm.trace != "" || lm.caller != "" {
		debugStr = fmt.Sprintf("\n==== DEBUG ====\nCaller: %s\nTrace: %s", lm.caller, lm.trace) + "===== END =====\n\n"
	}

	if loggerName == "" {
		loggerName = lm.loggerName
	}

	return strings.TrimSuffix(fmt.Sprintf("%s [%s] %s: %s%s%s",
		lm.Timestamp.Format(time.RFC3339Nano),
		lm.LevelString(),
		loggerName,
		strings.TrimSuffix(lm.Message, "\n"),
		metaStr,
		debugStr,
	), "\n") + "\n"
}

func traceCaller() string {
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return "???"
	}
	short := filepath.Base(file)
	fn := runtime.FuncForPC(pc).Name()
	return fmt.Sprintf("trace: %s:%d (%s)", short, line, fn)
}

func traceStack() string {
	buf := make([]byte, 4<<10)
	n := runtime.Stack(buf, false)
	return "stack:\n" + string(buf[:n])
}
