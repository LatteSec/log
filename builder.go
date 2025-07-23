package log

type (
	LoggerBuilder struct {
		LoggerMeta
		path string
	}

	LoggerMeta struct {
		level Level  // defaults to WARN
		name  string // the name of the logger

		stdoutEnabled bool
		stderrEnabled bool

		handlers []LogHandler
		cleanup  []func() // to be ran on fatal
	}
)

func NewLogger() *LoggerBuilder {
	return &LoggerBuilder{
		LoggerMeta: LoggerMeta{
			level:         WARN,
			stdoutEnabled: true,
			stderrEnabled: true,
		},
	}
}

func (lb *LoggerBuilder) Build() (*Logger, error) {
	if lb.level <= TRACE || lb.level > QUIET {
		return nil, ErrInvalidLogLevel
	}

	if lb.path != "" {
		fh, err := NewFileHandler(lb.path)
		if err != nil {
			return nil, err
		}

		lb.handlers = append(lb.handlers, fh)
		lb.path = ""
	}

	if lb.name == "" {
		lb.name = "???"
	}

	return &Logger{
		LoggerMeta: lb.LoggerMeta,
	}, nil
}

func (lb *LoggerBuilder) WithHandlers(hs ...LogHandler) *LoggerBuilder {
	lb.handlers = append(lb.handlers, hs...)
	return lb
}

func (lb *LoggerBuilder) WithCleanup(fns ...func()) *LoggerBuilder {
	lb.cleanup = append(lb.cleanup, fns...)
	return lb
}

func (lb *LoggerBuilder) Name(name string) *LoggerBuilder     { lb.name = name; return lb }
func (lb *LoggerBuilder) WithFile(path string) *LoggerBuilder { lb.path = path; return lb }

func (lb *LoggerBuilder) WithStdout(on bool) *LoggerBuilder { lb.stdoutEnabled = on; return lb }
func (lb *LoggerBuilder) WithStderr(on bool) *LoggerBuilder { lb.stderrEnabled = on; return lb }

func (lb *LoggerBuilder) Trace() *LoggerBuilder { lb.level = TRACE; return lb }
func (lb *LoggerBuilder) Debug() *LoggerBuilder { lb.level = DEBUG; return lb }
func (lb *LoggerBuilder) Info() *LoggerBuilder  { lb.level = INFO; return lb }
func (lb *LoggerBuilder) Warn() *LoggerBuilder  { lb.level = WARN; return lb }
func (lb *LoggerBuilder) Error() *LoggerBuilder { lb.level = ERROR; return lb }
func (lb *LoggerBuilder) WithLevel(level Level) *LoggerBuilder {
	lb.level = level
	return lb
}
