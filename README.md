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

## Contributing

### Project Structure

```text
go-libs/
├── cache/              # Generic TTL cache
│   ├── cache.go
│   ├── cache_test.go
│   └── README.md
├── logger/             # Logger interface
│   ├── logger.go
│   ├── logger_test.go
│   └── README.md
├── netx/               # Networking extensions
│   └── dns/            # DNS resolver
│       ├── resolver.go
│       ├── resolver_test.go
│       └── README.md
├── go.mod
├── Taskfile.yml
└── README.md
```

### Adding a New Package

1. Create a new directory at the appropriate level:
   - Root level for generic utilities (e.g., `slices/`, `maps/`)
   - Under `netx/` for networking-related packages
2. Implement the package with:
   - `<name>.go` - Main implementation
   - `<name>_test.go` - Unit tests (≥70% coverage required)
   - `README.md` - Package documentation

### Requirements

- **Test coverage**: Minimum 70% for all new packages
- **Documentation**: Each package must have a README with:
  - Features list
  - Installation instructions
  - Usage examples
  - API reference
- **Code style**: Run `go fmt` and `go vet` before submitting
- **Tests must pass**: Run `task test` or `go test ./...`

### Running Coverage Check

```bash
# Check coverage for all packages
task test:cover

# Check specific package
go test ./cache -cover
```

## License

MIT
