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
	Msg       string             // log message
	Meta      []LogMessageMetaKV // log metadata

	trace  string // stack trace (optional)
	caller string // caller (optional)
}

// NewLogMessage
//
// Creates a new LogMessage
func NewLogMessage(level Level, msg string) *LogMessage {
	return &LogMessage{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Msg:       msg,
		Meta:      make([]LogMessageMetaKV, 0, 1),
	}
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

	return strings.TrimSuffix(fmt.Sprintf("%s [%s] %s: %s%s%s",
		lm.Timestamp.Format(time.RFC3339Nano),
		levelNames[lm.Level],
		loggerName,
		strings.TrimSuffix(lm.Msg, "\n"),
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
