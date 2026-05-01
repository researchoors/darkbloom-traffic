package stats

import (
	"context"
	"log"
	"sync"
	"time"
)

// entry represents a single recorded request with its timestamp and token count.
type entry struct {
	ts     time.Time
	tokens int
}

// Tracker collects and reports rolling statistics on token throughput and request rate.
type Tracker struct {
	mu            sync.Mutex
	entries       []entry
	interval      time.Duration
	totalTokens   int64
	totalRequests int64
}

// NewTracker creates a new Tracker that logs stats at the given interval.
func NewTracker(interval time.Duration) *Tracker {
	return &Tracker{
		interval: interval,
	}
}

// Record appends a new entry with the current time and increments cumulative totals.
// The latency parameter is reserved for future use (e.g., latency histograms).
func (t *Tracker) Record(tokens int, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.entries = append(t.entries, entry{
		ts:     time.Now(),
		tokens: tokens,
	})
	t.totalTokens += int64(tokens)
	t.totalRequests++
}

// TPS returns the rolling average tokens-per-second over the last 60 seconds.
func (t *Tracker) TPS() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-60 * time.Second)
	var sum int64
	for _, e := range t.entries {
		if !e.ts.Before(cutoff) {
			sum += int64(e.tokens)
		}
	}
	return float64(sum) / 60.0
}

// RPS returns the rolling average requests-per-second over the last 60 seconds.
func (t *Tracker) RPS() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-60 * time.Second)
	var count int64
	for _, e := range t.entries {
		if !e.ts.Before(cutoff) {
			count++
		}
	}
	return float64(count) / 60.0
}

// pruneOldEntries removes entries older than 60 seconds from the slice.
func (t *Tracker) pruneOldEntries() {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-60 * time.Second)
	i := 0
	for _, e := range t.entries {
		if !e.ts.Before(cutoff) {
			t.entries[i] = e
			i++
		}
	}
	t.entries = t.entries[:i]
}

// Start launches a background goroutine that periodically logs stats and prunes
// old entries. It stops when the provided context is cancelled.
func (t *Tracker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tps := t.TPS()
				rps := t.RPS()
				log.Printf("[stats] TPS=%.2f RPS=%.2f totalTokens=%d totalRequests=%d",
					tps, rps, t.totalTokens, t.totalRequests)
				t.pruneOldEntries()
			}
		}
	}()
}
