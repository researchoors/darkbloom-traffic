package stats

import (
	"sync"
	"testing"
	"time"
)

func TestRecordAndTPS(t *testing.T) {
	tr := NewTracker(10 * time.Second)

	// Record 600 tokens total in the recent window
	tr.Record(200, 50*time.Millisecond)
	tr.Record(100, 30*time.Millisecond)
	tr.Record(300, 80*time.Millisecond)

	if tr.totalRequests != 3 {
		t.Errorf("expected totalRequests=3, got %d", tr.totalRequests)
	}
	if tr.totalTokens != 600 {
		t.Errorf("expected totalTokens=600, got %d", tr.totalTokens)
	}

	// All entries are within the last 60s, so TPS = 600/60 = 10.0
	tps := tr.TPS()
	if tps != 10.0 {
		t.Errorf("expected TPS=10.0, got %f", tps)
	}

	// RPS = 3 requests in 60s = 0.05
	rps := tr.RPS()
	if rps != 0.05 {
		t.Errorf("expected RPS=0.05, got %f", rps)
	}
}

func TestPruneOldEntries(t *testing.T) {
	tr := NewTracker(10 * time.Second)

	// Manually insert an old entry (61 seconds ago)
	tr.mu.Lock()
	tr.entries = append(tr.entries, entry{ts: time.Now().Add(-61 * time.Second), tokens: 500})
	tr.totalTokens += 500
	tr.totalRequests++
	// Insert a recent entry (5 seconds ago)
	tr.entries = append(tr.entries, entry{ts: time.Now().Add(-5 * time.Second), tokens: 100})
	tr.totalTokens += 100
	tr.totalRequests++
	tr.mu.Unlock()

	// Before pruning: 2 entries
	tr.mu.Lock()
	beforeLen := len(tr.entries)
	tr.mu.Unlock()
	if beforeLen != 2 {
		t.Errorf("expected 2 entries before prune, got %d", beforeLen)
	}

	// TPS should only count the recent 100 tokens → 100/60 ≈ 1.6667
	tps := tr.TPS()
	expectedTPS := 100.0 / 60.0
	if tps != expectedTPS {
		t.Errorf("expected TPS=%f after ignoring old entry, got %f", expectedTPS, tps)
	}

	// Now call prune explicitly and verify old entry is removed
	tr.pruneOldEntries()
	tr.mu.Lock()
	afterLen := len(tr.entries)
	tr.mu.Unlock()
	if afterLen != 1 {
		t.Errorf("expected 1 entry after prune, got %d", afterLen)
	}

	// Verify remaining entry has tokens=100
	tr.mu.Lock()
	remainingTokens := tr.entries[0].tokens
	tr.mu.Unlock()
	if remainingTokens != 100 {
		t.Errorf("expected remaining entry tokens=100, got %d", remainingTokens)
	}
}

func TestConcurrentRecord(t *testing.T) {
	tr := NewTracker(10 * time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.Record(1, time.Millisecond)
		}()
	}
	wg.Wait()

	if tr.totalRequests != 100 {
		t.Errorf("expected totalRequests=100, got %d", tr.totalRequests)
	}
	if tr.totalTokens != 100 {
		t.Errorf("expected totalTokens=100, got %d", tr.totalTokens)
	}
}
