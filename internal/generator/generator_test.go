package generator

import (
	"testing"

	"github.com/researchoors/darkbloom-traffic/internal/models"
)

func TestNextRoundRobin(t *testing.T) {
	m := []models.Model{
		{ID: "model-a", OwnedBy: "org1"},
		{ID: "model-b", OwnedBy: "org2"},
		{ID: "model-c", OwnedBy: "org3"},
	}
	g := NewGenerator(m, 100, 50)

	// Call Next enough times to go through all models multiple times.
	iterations := len(m) * 3
	gotOrder := make([]string, 0, iterations)
	for i := 0; i < iterations; i++ {
		req := g.Next()
		gotOrder = append(gotOrder, req.Model)
	}

	// Verify round-robin cycling: the sequence should repeat every len(m).
	for cycle := 0; cycle < 3; cycle++ {
		for i, mdl := range m {
			idx := cycle*len(m) + i
			if gotOrder[idx] != mdl.ID {
				t.Errorf("round-robin mismatch at position %d (cycle %d, slot %d): got %q, want %q",
					idx, cycle, i, gotOrder[idx], mdl.ID)
			}
		}
	}
}

func TestNextRandomizedParams(t *testing.T) {
	m := []models.Model{
		{ID: "model-a", OwnedBy: "org1"},
	}
	g := NewGenerator(m, 100, 50)

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		req := g.Next()

		if req.Temperature < 0.3 || req.Temperature > 1.5 {
			t.Errorf("temperature %f out of range [0.3, 1.5]", req.Temperature)
		}
		if req.TopP < 0.8 || req.TopP > 1.0 {
			t.Errorf("top_p %f out of range [0.8, 1.0]", req.TopP)
		}
		if req.MaxTokens < 50 || req.MaxTokens > 500 {
			t.Errorf("max_tokens %d out of range [50, 500]", req.MaxTokens)
		}
		if req.Stream != true {
			t.Errorf("stream should always be true, got %v", req.Stream)
		}
		if len(req.Messages) == 0 {
			t.Fatal("expected at least one message")
		}
		if req.Messages[0].Role != "user" {
			t.Errorf("expected role 'user', got %q", req.Messages[0].Role)
		}
	}
}

func TestAdjustContextIncrease(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)

	initial := g.currentContextTokens // 256

	// observedTPS is well below targetTPS*0.9 = 90
	g.AdjustContext(80)

	expected := int(float64(initial) * 1.1)
	if g.currentContextTokens != expected {
		t.Errorf("expected currentContextTokens=%d after increase, got %d", expected, g.currentContextTokens)
	}
}

func TestAdjustContextDecrease(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)

	initial := g.currentContextTokens

	// observedTPS is well above targetTPS*1.1 = 110
	g.AdjustContext(120)

	expected := int(float64(initial) * 0.9)
	if g.currentContextTokens != expected {
		t.Errorf("expected currentContextTokens=%d after decrease, got %d", expected, g.currentContextTokens)
	}
}

func TestAdjustContextNoChange(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)

	initial := g.currentContextTokens

	// observedTPS is right at target — within the 0.9-1.1 band
	g.AdjustContext(100)

	if g.currentContextTokens != initial {
		t.Errorf("expected no change, got %d (was %d)", g.currentContextTokens, initial)
	}
}

func TestAdjustContextClampLow(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)
	g.currentContextTokens = 50

	// Even with high TPS triggering a decrease, should not go below 50
	g.AdjustContext(200)

	if g.currentContextTokens < 50 {
		t.Errorf("currentContextTokens clamped below 50: got %d", g.currentContextTokens)
	}
}

func TestAdjustContextClampHigh(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)
	g.currentContextTokens = 4000

	// Even with low TPS triggering an increase, should not go above 4000
	g.AdjustContext(1)

	if g.currentContextTokens > 4000 {
		t.Errorf("currentContextTokens clamped above 4000: got %d", g.currentContextTokens)
	}
}

func TestNewGeneratorDefaults(t *testing.T) {
	m := []models.Model{{ID: "m", OwnedBy: "o"}}
	g := NewGenerator(m, 100, 50)

	if g.currentContextTokens != 256 {
		t.Errorf("expected default currentContextTokens=256, got %d", g.currentContextTokens)
	}
	if g.targetTPS != 100 {
		t.Errorf("expected targetTPS=100, got %f", g.targetTPS)
	}
	if g.maxRPS != 50 {
		t.Errorf("expected maxRPS=50, got %f", g.maxRPS)
	}
	if g.rng == nil {
		t.Error("expected rng to be initialized")
	}
}
