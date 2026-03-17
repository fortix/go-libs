package cache

import (
	"testing"
	"time"
)

func TestCacheGetSet(t *testing.T) {
	c := New[string](Options{})

	// Test set and get
	c.Set("key1", "value1", time.Minute)
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Fatalf("expected value1, got %s", val)
	}
}

func TestCacheNotFound(t *testing.T) {
	c := New[string](Options{})

	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent key")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := New[string](Options{})

	// Set with very short TTL
	c.Set("key1", "value1", 50*time.Millisecond)

	// Should be present immediately
	_, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1 immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be expired")
	}
}

func TestCacheDefaultTTL(t *testing.T) {
	c := New[string](Options{
		DefaultTTL: 100 * time.Millisecond,
	})

	// Set without specifying TTL (uses default)
	c.Set("key1", "value1", 0)

	// Should be present immediately
	_, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}

	// Wait for default TTL to expire
	time.Sleep(150 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to expire after default TTL")
	}
}

func TestCacheMaxTTL(t *testing.T) {
	c := New[string](Options{
		MaxTTL: 50 * time.Millisecond,
	})

	// Try to set with longer TTL than max
	c.Set("key1", "value1", time.Minute)

	// Should expire at MaxTTL, not the requested TTL
	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected key1 to expire at MaxTTL")
	}
}

func TestCacheDelete(t *testing.T) {
	c := New[string](Options{})

	c.Set("key1", "value1", time.Minute)
	_, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}

	c.Delete("key1")

	_, ok = c.Get("key1")
	if ok {
		t.Fatal("expected key1 to be deleted")
	}
}

func TestCacheClear(t *testing.T) {
	c := New[string](Options{})

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)

	if c.Len() != 2 {
		t.Fatalf("expected len 2, got %d", c.Len())
	}

	c.Clear()

	if c.Len() != 0 {
		t.Fatalf("expected len 0 after clear, got %d", c.Len())
	}
}

func TestCacheGenericTypes(t *testing.T) {
	// Test with different types
	t.Run("int", func(t *testing.T) {
		c := New[int](Options{})
		c.Set("key", 42, time.Minute)
		val, ok := c.Get("key")
		if !ok || val != 42 {
			t.Fatalf("expected 42, got %d, ok=%v", val, ok)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type Item struct {
			Name  string
			Value int
		}
		c := New[Item](Options{})
		c.Set("key", Item{Name: "test", Value: 123}, time.Minute)
		val, ok := c.Get("key")
		if !ok || val.Name != "test" || val.Value != 123 {
			t.Fatalf("unexpected result: %+v, ok=%v", val, ok)
		}
	})

	t.Run("slice", func(t *testing.T) {
		c := New[[]string](Options{})
		c.Set("key", []string{"a", "b", "c"}, time.Minute)
		val, ok := c.Get("key")
		if !ok || len(val) != 3 {
			t.Fatalf("unexpected result: %v, ok=%v", val, ok)
		}
	})
}

func TestCacheCleanup(t *testing.T) {
	// Test with cleanup enabled
	c := New[string](Options{
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: 100 * time.Millisecond,
	})
	defer c.Stop()

	c.Set("key1", "value1", 0)

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Entry should be cleaned up
	if c.Len() > 0 {
		t.Fatalf("expected cache to be cleaned, len=%d", c.Len())
	}
}

func TestCacheStop(t *testing.T) {
	c := New[string](Options{
		CleanupInterval: 10 * time.Millisecond,
	})

	// Stop should not panic
	c.Stop()

	// Multiple stops should be safe
	c.Stop()
}

func TestCacheConcurrent(t *testing.T) {
	c := New[int](Options{})

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			c.Set(string(rune(n)), n, time.Minute)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 100; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		go func(n int) {
			c.Get(string(rune(n)))
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 100; i++ {
		<-done
	}
}
