package logger

// Logger is the application logging interface.
// Replace the underlying library by providing a different adapter.
type Logger interface {
	WithField(key string, value any) Logger
	WithError(err error) Logger
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
}
