package logger

import "github.com/sirupsen/logrus"

type logrusLogger struct {
	entry *logrus.Entry
}

// New returns a Logger backed by the global logrus standard logger.
// Configure the standard logger in main (formatter, level, hooks) before the first log call.
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
