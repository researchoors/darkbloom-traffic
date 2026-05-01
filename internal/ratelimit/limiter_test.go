package ratelimit

import (
	"context"
	"testing"
	"time"
)

// TestLimiterEnforcesRate verifies that 3 calls at 2 RPS take at least ~1 second.
// At 2 RPS the interval is 500ms. The first call proceeds immediately,
// the second waits ~500ms, the third waits another ~500ms → total ≥ ~1000ms.
// We assert > 900ms to allow minor clock skew.
func TestLimiterEnforcesRate(t *testing.T) {
	limiter := NewLimiter(2) // 500ms interval

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("unexpected error on call %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected elapsed > 900ms, got %v", elapsed)
	}
}

// TestLimiterRespectsContext verifies that Wait returns immediately with an
// error when the context is already cancelled.
func TestLimiterRespectsContext(t *testing.T) {
	limiter := NewLimiter(2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := limiter.Wait(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
