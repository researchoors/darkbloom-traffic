package generator

import (
	"math/rand"
	"sync"

	"github.com/researchoors/darkbloom-traffic/internal/models"
)

// Message represents a single chat message in the request payload.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request represents an OpenAI-compatible chat completion request.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

// Generator produces randomized chat completion requests that cycle through
// the available model list in round-robin order.
type Generator struct {
	models              []models.Model
	targetTPS           float64
	maxRPS              float64
	currentContextTokens int
	mu                  sync.Mutex
	rng                 *rand.Rand
	modelIndex          int
}

// NewGenerator creates a Generator that round-robins through the supplied
// models. targetTPS and maxRPS configure the adaptive context window.
// currentContextTokens defaults to 256.
func NewGenerator(models []models.Model, targetTPS, maxRPS float64) *Generator {
	return &Generator{
		models:              models,
		targetTPS:           targetTPS,
		maxRPS:              maxRPS,
		currentContextTokens: 256,
		rng:                 rand.New(rand.NewSource(0)), // deterministic seed for reproducibility in tests
	}
}

// Next returns a new Request with round-robin model selection and randomised
// parameters. The prompt is generated to approximate the current context token
// count.
func (g *Generator) Next() Request {
	g.mu.Lock()
	idx := g.modelIndex
	g.modelIndex = (g.modelIndex + 1) % len(g.models)
	tokens := g.currentContextTokens
	rng := g.rng
	g.mu.Unlock()

	temperature := 0.3 + rng.Float64()*(1.5-0.3)
	topP := 0.8 + rng.Float64()*(1.0-0.8)
	maxTokens := 50 + rng.Intn(500-50+1)

	prompt := "Repeat after me: " + randomWords(rng, tokens)

	return Request{
		Model:       g.models[idx].ID,
		Messages:    []Message{{Role: "user", Content: prompt}},
		Temperature: temperature,
		TopP:        topP,
		MaxTokens:   maxTokens,
		Stream:      true,
	}
}

// AdjustContext adapts currentContextTokens based on observed throughput
// relative to the target. If observedTPS is below 90% of target, the context
// window grows by 10%; if above 110%, it shrinks by 10%. The value is clamped
// to [50, 4000].
func (g *Generator) AdjustContext(observedTPS float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	low := g.targetTPS * 0.9
	high := g.targetTPS * 1.1

	switch {
	case observedTPS < low:
		g.currentContextTokens = int(float64(g.currentContextTokens) * 1.1)
	case observedTPS > high:
		g.currentContextTokens = int(float64(g.currentContextTokens) * 0.9)
	}

	if g.currentContextTokens < 50 {
		g.currentContextTokens = 50
	}
	if g.currentContextTokens > 4000 {
		g.currentContextTokens = 4000
	}
}

// randomWords produces a space-separated string of random words whose total
// length approximates the requested number of tokens (assuming ~4 chars per
// token).
func randomWords(rng *rand.Rand, tokens int) string {
	if tokens <= 0 {
		return ""
	}
	// Approximate each token as ~4 characters + a space separator.
	targetLen := tokens * 4
	words := make([]byte, 0, targetLen)
	for len(words) < targetLen {
		if len(words) > 0 {
			words = append(words, ' ')
		}
		// Generate a random word of length 3–7 lowercase letters.
		wlen := 3 + rng.Intn(5)
		for j := 0; j < wlen; j++ {
			words = append(words, byte('a'+rng.Intn(26)))
		}
	}
	return string(words)
}

// CurrentContextTokens returns the current context token count (thread-safe).
func (g *Generator) CurrentContextTokens() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentContextTokens
}
