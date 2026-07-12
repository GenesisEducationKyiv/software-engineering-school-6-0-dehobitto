package logger

import "github.com/sirupsen/logrus"

type Logger interface {
	WithField(key string, value any) Logger
	WithError(err error) Logger
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
}

type logrusLogger struct {
	entry *logrus.Entry
}

func New() Logger {
	return &logrusLogger{entry: logrus.NewEntry(logrus.StandardLogger())}
}

func (l *logrusLogger) WithField(key string, value any) Logger {
	return &logrusLogger{entry: l.entry.WithField(key, value)}
}

func (l *logrusLogger) WithError(err error) Logger {
	return &logrusLogger{entry: l.entry.WithError(err)}
}

func (l *logrusLogger) Info(msg string)  { l.entry.Info(msg) }
func (l *logrusLogger) Warn(msg string)  { l.entry.Warn(msg) }
func (l *logrusLogger) Error(msg string) { l.entry.Error(msg) }
func (l *logrusLogger) Fatal(msg string) { l.entry.Fatal(msg) }

type noopLogger struct{}

func NewNoop() Logger {
	return noopLogger{}
}

func (n noopLogger) WithField(_ string, _ any) Logger { return n }
func (n noopLogger) WithError(_ error) Logger         { return n }
func (n noopLogger) Info(_ string)                    {}
func (n noopLogger) Warn(_ string)                    {}
func (n noopLogger) Error(_ string)                   {}
func (n noopLogger) Fatal(_ string)                   {}
