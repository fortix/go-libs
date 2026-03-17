// Package logger provides a minimal logger interface that can be implemented
// by any logging library (slog, zap, logrus, etc.).
package logger

// Logger is a minimal interface for structured logging.
// It uses key-value pairs for structured logging, compatible with
// most Go logging libraries including github.com/paularlott/logger.
type Logger interface {
	// Trace logs a message at trace level with optional key-value pairs.
	Trace(msg string, keysAndValues ...any)
	// Debug logs a message at debug level with optional key-value pairs.
	Debug(msg string, keysAndValues ...any)
	// Info logs a message at info level with optional key-value pairs.
	Info(msg string, keysAndValues ...any)
	// Warn logs a message at warn level with optional key-value pairs.
	Warn(msg string, keysAndValues ...any)
	// Error logs a message at error level with optional key-value pairs.
	Error(msg string, keysAndValues ...any)
	// Fatal logs a message at fatal level with optional key-value pairs.
	Fatal(msg string, keysAndValues ...any)
}

// NoopLogger is a no-operation logger that discards all log messages.
// Use this when logging is not needed or for testing.
type NoopLogger struct{}

func (n NoopLogger) Trace(msg string, keysAndValues ...any) {}
func (n NoopLogger) Debug(msg string, keysAndValues ...any) {}
func (n NoopLogger) Info(msg string, keysAndValues ...any)  {}
func (n NoopLogger) Warn(msg string, keysAndValues ...any)  {}
func (n NoopLogger) Error(msg string, keysAndValues ...any) {}
func (n NoopLogger) Fatal(msg string, keysAndValues ...any) {}

// noopLogger is a singleton NoopLogger for convenience.
var noopLogger = NoopLogger{}

// Noop returns a NoopLogger instance.
func Noop() Logger {
	return noopLogger
}
