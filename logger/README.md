# logger

Minimal logger interface for structured logging.

## Features

- 6 log levels: Trace, Debug, Info, Warn, Error, Fatal
- Key-value pairs for structured logging
- Compatible with popular logging libraries (slog, zap, logrus, etc.)
- `NoopLogger` for optional logging

## Installation

```bash
go get github.com/fortix/go-libs/logger
```

## Usage

```go
import "github.com/fortix/go-libs/logger"
```

### Interface

```go
type Logger interface {
    Trace(msg string, keysAndValues ...any)
    Debug(msg string, keysAndValues ...any)
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
    Fatal(msg string, keysAndValues ...any)
}
```

### NoopLogger

Use when logging is optional or for testing:

```go
log := logger.Noop()
log.Debug("this is discarded")
log.Info("this too", "key", "value")
```

### Implementing the Interface

Any logging library with compatible method signatures satisfies this interface:

```go
// Example with github.com/paularlott/logger
import "github.com/paularlott/logger"

type MyService struct {
    log logger.Logger
}

func NewService(log logger.Logger) *MyService {
    if log == nil {
        log = logger.Noop()
    }
    return &MyService{log: log}
}
```

### Using with go.uber.org/zap

```go
type zapAdapter struct {
    *zap.SugaredLogger
}

func (z *zapAdapter) Trace(msg string, keysAndValues ...any) {
    z.Debugw(msg, keysAndValues...)
}
func (z *zapAdapter) Debug(msg string, keysAndValues ...any) {
    z.Debugw(msg, keysAndValues...)
}
func (z *zapAdapter) Info(msg string, keysAndValues ...any) {
    z.Infow(msg, keysAndValues...)
}
func (z *zapAdapter) Warn(msg string, keysAndValues ...any) {
    z.Warnw(msg, keysAndValues...)
}
func (z *zapAdapter) Error(msg string, keysAndValues ...any) {
    z.Errorw(msg, keysAndValues...)
}
func (z *zapAdapter) Fatal(msg string, keysAndValues ...any) {
    z.Fatalw(msg, keysAndValues...)
}
```

### Using with log/slog

```go
type slogAdapter struct {
    *slog.Logger
}

func (s *slogAdapter) Trace(msg string, keysAndValues ...any) {
    s.Debug(msg, keysAndValues...)
}
func (s *slogAdapter) Debug(msg string, keysAndValues ...any) {
    s.Debug(msg, keysAndValues...)
}
func (s *slogAdapter) Info(msg string, keysAndValues ...any) {
    s.Info(msg, keysAndValues...)
}
func (s *slogAdapter) Warn(msg string, keysAndValues ...any) {
    s.Warn(msg, keysAndValues...)
}
func (s *slogAdapter) Error(msg string, keysAndValues ...any) {
    s.Error(msg, keysAndValues...)
}
func (s *slogAdapter) Fatal(msg string, keysAndValues ...any) {
    s.Error(msg, keysAndValues...)
    os.Exit(1)
}
```
