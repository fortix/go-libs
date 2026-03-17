# cache

Generic TTL-based cache with automatic cleanup.

## Features

- Generic type parameter support (Go 1.18+)
- TTL-based expiration
- Lazy expiration on access + optional background cleanup
- Thread-safe with `sync.RWMutex`
- Configurable max TTL and default TTL

## Installation

```bash
go get github.com/fortix/go-libs/cache
```

## Usage

```go
import "github.com/fortix/go-libs/cache"
```

### Basic Example

```go
// Create a string cache
c := cache.New[string](cache.Options{
    DefaultTTL:     5 * time.Minute,
    MaxTTL:         10 * time.Minute,
    CleanupInterval: 1 * time.Minute,
})
defer c.Stop()

// Set a value with default TTL
c.Set("key", "value", 0)

// Set a value with custom TTL
c.Set("key2", "value2", 30*time.Second)

// Get a value
val, ok := c.Get("key")
if ok {
    fmt.Println(val) // "value"
}

// Delete a key
c.Delete("key")

// Clear all entries
c.Clear()
```

### Different Types

```go
// Cache for integers
intCache := cache.New[int](cache.Options{})

// Cache for structs
type User struct {
    Name string
    Age  int
}
userCache := cache.New[User](cache.Options{})
userCache.Set("user1", User{Name: "Alice", Age: 30}, time.Minute)

// Cache for slices
sliceCache := cache.New[[]string](cache.Options{})
```

## Options

| Option | Type | Description |
|--------|------|-------------|
| `DefaultTTL` | `time.Duration` | TTL when Set is called with ttl <= 0. Default: 60s |
| `MaxTTL` | `time.Duration` | Maximum TTL cap. 0 = unlimited |
| `CleanupInterval` | `time.Duration` | Background cleanup interval. 0 = disabled |

## API

### `New[T any](opts Options) *Cache[T]`

Creates a new cache with the given options.

### `Get(key string) (T, bool)`

Returns the value and true if found and not expired, zero value and false otherwise.

### `Set(key string, value T, ttl time.Duration)`

Stores a value. If ttl <= 0, uses DefaultTTL. TTL is capped to MaxTTL.

### `Delete(key string)`

Removes a key from the cache.

### `Clear()`

Removes all entries.

### `Len() int`

Returns the number of entries (including expired ones not yet cleaned).

### `Stop()`

Stops the background cleanup goroutine.
