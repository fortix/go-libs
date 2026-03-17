// Package cache provides a generic TTL-based cache with automatic cleanup.
package cache

import (
	"context"
	"sync"
	"time"
)

// Options configures the cache behavior.
type Options struct {
	// MaxTTL caps all TTL values to this maximum.
	// 0 means no cap.
	MaxTTL time.Duration

	// DefaultTTL is used when Set is called with ttl <= 0.
	// If 0, a default of 60 seconds is used.
	DefaultTTL time.Duration

	// CleanupInterval specifies how often to clean expired entries.
	// If 0, cleanup is disabled (lazy expiration only).
	CleanupInterval time.Duration
}

// entry holds a cached value with its expiration time.
type entry[T any] struct {
	value     T
	expiresAt time.Time
}

// Cache is a generic TTL-based cache with thread-safe operations.
// It supports both lazy expiration (on access) and active cleanup
// (background goroutine).
type Cache[T any] struct {
	entries    map[string]*entry[T]
	mu         sync.RWMutex
	maxTTL     time.Duration
	defaultTTL time.Duration

	cleanupTicker  *time.Ticker
	cleanupCancel  context.CancelFunc
	cleanupRunning bool
}

// New creates a new Cache with the given options.
func New[T any](opts Options) *Cache[T] {
	if opts.DefaultTTL <= 0 {
		opts.DefaultTTL = 60 * time.Second
	}

	c := &Cache[T]{
		entries:    make(map[string]*entry[T]),
		maxTTL:     opts.MaxTTL,
		defaultTTL: opts.DefaultTTL,
	}

	if opts.CleanupInterval > 0 {
		c.startCleanup(opts.CleanupInterval)
	}

	return c
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired,
// zero value and false otherwise.
func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, exists := c.entries[key]
	if !exists {
		var zero T
		return zero, false
	}

	if time.Now().After(ent.expiresAt) {
		delete(c.entries, key)
		var zero T
		return zero, false
	}

	return ent.value, true
}

// Set stores a value in the cache with the given TTL.
// If ttl <= 0, the cache's default TTL is used.
// The TTL is capped to MaxTTL if configured.
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	if c.maxTTL > 0 && ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &entry[T]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a key from the cache.
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*entry[T])
}

// Len returns the number of entries in the cache (including expired ones
// that haven't been cleaned up yet).
func (c *Cache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stop stops the cleanup goroutine if running.
// Call this to release resources when done with the cache.
func (c *Cache[T]) Stop() {
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
		if c.cleanupCancel != nil {
			c.cleanupCancel()
		}
		c.cleanupTicker = nil
		c.cleanupCancel = nil
		c.cleanupRunning = false
	}
}

// startCleanup starts a background goroutine to periodically clean expired entries.
func (c *Cache[T]) startCleanup(interval time.Duration) {
	if c.cleanupRunning {
		return
	}

	c.cleanupTicker = time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())
	c.cleanupCancel = cancel
	c.cleanupRunning = true

	// Capture ticker channel to avoid race with Stop()
	tickerChan := c.cleanupTicker.C

	go func() {
		for {
			select {
			case <-tickerChan:
				c.cleanExpired()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// cleanExpired removes all expired entries from the cache.
func (c *Cache[T]) cleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, ent := range c.entries {
		if now.After(ent.expiresAt) {
			delete(c.entries, key)
		}
	}
}
