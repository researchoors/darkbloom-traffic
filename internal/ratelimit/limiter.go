package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter controls the rate at which calls proceed.
// It enforces a minimum interval between successive Wait calls.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

// NewLimiter creates a Limiter that enforces the given rate (requests per second).
func NewLimiter(rps float64) *Limiter {
	return &Limiter{
		interval: time.Duration(float64(time.Second) / rps),
	}
}

// Wait blocks until the next call is permitted according to the rate limit.
// It respects context cancellation and returns ctx.Err() if the context is
// cancelled before the wait completes.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()

	// Check for an already-cancelled context before doing any work.
	select {
	case <-ctx.Done():
		l.mu.Unlock()
		return ctx.Err()
	default:
	}

	elapsed := time.Since(l.last)
	remaining := l.interval - elapsed

	if remaining > 0 {
		l.mu.Unlock()

		select {
		case <-time.After(remaining):
			l.mu.Lock()
			l.last = time.Now()
			l.mu.Unlock()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	l.last = time.Now()
	l.mu.Unlock()
	return nil
}
