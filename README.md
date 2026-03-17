# go-libs

A collection of reusable Go libraries.

## Packages

| Package | Description |
|---------|-------------|
| [cache](./cache) | Generic TTL-based cache with automatic cleanup |
| [logger](./logger) | Minimal logger interface for structured logging |
| [netx/dns](./netx/dns) | DNS resolver with caching and parallel queries |

## Installation

```bash
go get github.com/fortix/go-libs
```

## Quick Start

```go
import (
    "github.com/fortix/go-libs/cache"
    "github.com/fortix/go-libs/logger"
    "github.com/fortix/go-libs/netx/dns"
)
```

## Development

This project uses [Task](https://taskfile.dev/) for common development tasks.

```bash
# Run all tests
task test

# Run tests with coverage
task test:cover

# Run integration tests (requires network)
task test:integration

# Format code
task fmt

# Run linter
task lint
```

Or use standard Go commands:

```bash
go test ./...
go test ./... -cover
go fmt ./...
go vet ./...
```

## License

MIT
