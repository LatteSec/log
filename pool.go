package log

import "sync"

// recycled log messages to reduce allocations
var logMsgPool = sync.Pool{
	New: func() any {
		return &LogMessage{
			Meta: make([]LogMessageMetaKV, 0, 1),
		}
	},
}

func acquireLogMessage(loggerName string, msg *LogMessage) *LogMessage {
	lm := logMsgPool.Get().(*LogMessage)

	lm.Timestamp = msg.Timestamp
	lm.Level = msg.Level
	lm.Message = msg.Message
	lm.trace = msg.trace
	lm.caller = msg.caller
	lm.loggerName = loggerName

	lm.Meta = lm.Meta[:0]
	lm.Meta = append(lm.Meta, msg.Meta...)

	return lm
}

func releaseLogMessage(lm *LogMessage) {
	lm.Meta = lm.Meta[:0]
	lm.trace = ""
	lm.caller = ""
	lm.loggerName = ""

	// shrink if overinflated
	if cap(lm.Meta) > 16 {
		lm.Meta = make([]LogMessageMetaKV, 0, 1)
	}

	logMsgPool.Put(lm)
}
