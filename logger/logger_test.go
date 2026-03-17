package logger

import (
	"testing"
)

func TestNoopLogger(t *testing.T) {
	// Just verify NoopLogger implements Logger interface
	var _ Logger = NoopLogger{}
	var _ Logger = Noop()

	// These should not panic
	log := Noop()
	log.Trace("test", "key", "value")
	log.Debug("test", "key", "value")
	log.Info("test", "key", "value")
	log.Warn("test", "key", "value")
	log.Error("test", "key", "value")
	log.Fatal("test", "key", "value")
}

func TestNoopSingleton(t *testing.T) {
	// Noop() should return the same instance
	l1 := Noop()
	l2 := Noop()
	if l1 != l2 {
		t.Fatal("expected same NoopLogger instance")
	}
}
