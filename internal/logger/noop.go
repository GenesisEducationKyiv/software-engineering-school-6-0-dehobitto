package logger

type noopLogger struct{}

// NewNoop returns a logger that discards all entries.
func NewNoop() Logger {
	return noopLogger{}
}

func (n noopLogger) WithField(_ string, _ any) Logger { return n }
func (n noopLogger) WithError(_ error) Logger         { return n }
func (n noopLogger) Info(_ string)                    {}
func (n noopLogger) Warn(_ string)                    {}
func (n noopLogger) Error(_ string)                   {}
func (n noopLogger) Fatal(_ string)                   {}
