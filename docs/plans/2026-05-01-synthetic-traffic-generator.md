# Dark Bloom Synthetic Traffic Generator — Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Build a long-running Go process that generates persistent LLM traffic across the Dark Bloom network at 4–5 RPS with configurable TPS targeting.

**Architecture:** Single binary with internal packages for config, model discovery, rate limiting, request generation, and throughput feedback. A goroutine-based pipeline: discoverer → scheduler → workers → stats. A feedback loop adjusts per-request context size to hit the TPS target.

**Tech Stack:** Go 1.24, stdlib HTTP client, no external deps. golangci-lint for CI.

---

### Task 1: Scaffold project structure + go.mod + .gitignore

**Objective:** Create the directory layout and Go module so `go build ./...` works.

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `cmd/traffic/main.go` (empty main with `fmt.Println("stub")`)
- Create: `internal/config/config.go` (package declaration only)
- Create: `internal/models/discover.go` (package declaration only)
- Create: `internal/ratelimit/limiter.go` (package declaration only)
- Create: `internal/generator/generator.go` (package declaration only)
- Create: `internal/stats/stats.go` (package declaration only)

**Step 1: Create directories**
```bash
mkdir -p ~/darkbloom-traffic/{cmd/traffic,internal/{config,models,ratelimit,generator,stats},.github/workflows,docs/plans}
```

**Step 2: Initialize go.mod**
```bash
cd ~/darkbloom-traffic && go mod init github.com/researchoors/darkbloom-traffic
```

**Step 3: Create .gitignore**
```
# Binaries
/traffic
*.exe

# IDE
.idea/
.vscode/

# OS
.DS_Store

# Env
.env
```

**Step 4: Create stub files** — each internal package gets a `package <name>` declaration. `cmd/traffic/main.go` gets:
```go
package main

import "fmt"

func main() {
	fmt.Println("darkbloom-traffic: stub")
}
```

**Step 5: Verify**
```bash
cd ~/darkbloom-traffic && go build ./...
```
Expected: no errors.

**Step 6: Commit**
```bash
cd ~/darkbloom-traffic && git init && git add -A && git commit -m "chore: scaffold project structure"
```

---

### Task 2: Implement config package

**Objective:** Config struct with env var binding, defaults, and validation.

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Write failing test**
Create `internal/config/config_test.go`:
```go
package config

import "testing"

func TestDefaultConfig(t *testing.T) {
	c := Default()
	if c.MaxRPS != 4.5 {
		t.Errorf("Default MaxRPS = %f, want 4.5", c.MaxRPS)
	}
	if c.TargetTPS != 500 {
		t.Errorf("Default TargetTPS = %f, want 500", c.TargetTPS)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	c := Config{}
	if err := c.Validate(); err == nil {
		t.Error("expected validation error for empty config")
	}
}

func TestValidateAcceptsValid(t *testing.T) {
	c := Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: 100, MaxRPS: 4}
	if err := c.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

**Step 2: Run test to verify failure**
```bash
cd ~/darkbloom-traffic && go test ./internal/config/ -v
```
Expected: FAIL — functions not defined.

**Step 3: Write implementation**
```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BaseURL              string
	APIKey               string
	TargetTPS            float64
	MaxRPS               float64
	ModelRefreshInterval time.Duration
	RequestTimeout       time.Duration
	StatsInterval        time.Duration
}

func Default() Config {
	return Config{
		BaseURL:              "https://inference.darkbloom.ai",
		TargetTPS:            500,
		MaxRPS:               4.5,
		ModelRefreshInterval: 5 * time.Minute,
		RequestTimeout:       120 * time.Second,
		StatsInterval:        10 * time.Second,
	}
}

func FromEnv() Config {
	c := Default()
	if v := os.Getenv("DARKBLOOM_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("DARKBLOOM_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("DARKBLOOM_TARGET_TPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			c.TargetTPS = f
		}
	}
	if v := os.Getenv("DARKBLOOM_MAX_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			c.MaxRPS = f
		}
	}
	return c
}

func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("DARKBLOOM_BASE_URL is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("DARKBLOOM_API_KEY is required")
	}
	if c.TargetTPS <= 0 {
		return fmt.Errorf("DARKBLOOM_TARGET_TPS must be positive, got %f", c.TargetTPS)
	}
	if c.MaxRPS <= 0 || c.MaxRPS > 10 {
		return fmt.Errorf("DARKBLOOM_MAX_RPS must be in (0, 10], got %f", c.MaxRPS)
	}
	return nil
}
```

**Step 4: Run test to verify pass**
```bash
cd ~/darkbloom-traffic && go test ./internal/config/ -v
```
Expected: PASS.

**Step 5: Commit**
```bash
cd ~/darkbloom-traffic && git add -A && git commit -m "feat: implement config package with env binding"
```

---

### Task 3: Implement models package (model discovery)

**Objective:** Fetch and cache available models from `/v1/models` endpoint. Thread-safe with RWMutex.

**Files:**
- Modify: `internal/models/discover.go`
- Create: `internal/models/discover_test.go`

**Step 1: Write failing test**
```go
package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Error("missing auth header")
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"qwen-4b","owned_by":"darkbloom"},{"id":"llama-8b","owned_by":"darkbloom"}]}`))
	}))
	defer srv.Close()

	d := NewDiscoverer(srv.URL, "testkey")
	if err := d.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := d.Count(); n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
	models := d.Models()
	if len(models) != 2 {
		t.Errorf("Models len = %d, want 2", len(models))
	}
	if models[0].ID != "qwen-4b" {
		t.Errorf("Models[0].ID = %q, want %q", models[0].ID, "qwen-4b")
	}
}

func TestDiscoverModelsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	d := NewDiscoverer(srv.URL, "key")
	if err := d.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := d.Count(); n != 0 {
		t.Errorf("Count = %d, want 0", n)
	}
}
```

**Step 2: Run test to verify failure**

**Step 3: Write implementation**
```go
package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Model struct {
	ID      string
	OwnedBy string
}

type Discoverer struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	mu         sync.RWMutex
	models     []Model
}

func NewDiscoverer(baseURL, apiKey string) *Discoverer {
	return &Discoverer{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type modelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func (d *Discoverer) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/v1/models", nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("models endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var mr modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return fmt.Errorf("decoding models: %w", err)
	}

	models := make([]Model, 0, len(mr.Data))
	for _, m := range mr.Data {
		models = append(models, Model{ID: m.ID, OwnedBy: m.OwnedBy})
	}

	d.mu.Lock()
	d.models = models
	d.mu.Unlock()

	return nil
}

func (d *Discoverer) Models() []Model {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Model, len(d.models))
	copy(out, d.models)
	return out
}

func (d *Discoverer) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.models)
}
```

**Step 4: Run test, verify pass, commit**

---

### Task 4: Implement rate limiter

**Objective:** Token-bucket rate limiter that enforces the RPS ceiling. Returns how long to wait before next request.

**Files:**
- Modify: `internal/ratelimit/limiter.go`
- Create: `internal/ratelimit/limiter_test.go`

**Implementation:**
- `NewLimiter(rps float64) *Limiter`
- `Wait(ctx) error` — blocks until a token is available
- Uses `time.Ticker` internally or golang.org/x/time/rate if we add a dep. **No — keep it stdlib.** Use a simple approach: track last send time, compute sleep duration.

```go
package ratelimit

import (
	"context"
	"sync"
	"time"
)

type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func NewLimiter(rps float64) *Limiter {
	return &Limiter{
		interval: time.Duration(float64(time.Second) / rps),
	}
}

func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	elapsed := time.Since(l.last)
	wait := l.interval - elapsed
	l.last = time.Now()
	l.mu.Unlock()

	if wait > 0 {
		select {
		case <-time.After(wait):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
```

Test: verify that 5 calls at 4.5 RPS take ~1.1s total (not instant).

---

### Task 5: Implement request generator with context-size entropy

**Objective:** Generate varied chat completion requests with randomized context sizes, temperature, top_p, top_k.

**Files:**
- Modify: `internal/generator/generator.go`
- Create: `internal/generator/generator_test.go`

**Key logic:**
- `NewGenerator(models, targetTPS, maxRPS) *Generator`
- `Next() Request` — returns a request with:
  - Round-robin model selection
  - Random prompt length: sampled from a distribution that targets the desired TPS given observed latency
  - Random `temperature` ∈ [0.3, 1.5]
  - Random `top_p` ∈ [0.8, 1.0]
  - Random `max_tokens` ∈ [50, 500]
- Prompt generation: repeat a base sentence N times where N is sampled to hit context size target

**Context size feedback loop:**
- Track rolling average tokens/sec from recent requests
- If observed TPS < target → increase context size (more tokens per request = fewer requests needed)
- If observed TPS > target → decrease context size
- Clamp context size to [50, 4000] tokens

```go
type Request struct {
	Model       string  `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64 `json:"temperature"`
	TopP        float64  `json:"top_p"`
	MaxTokens   int     `json:"max_tokens"`
	Stream      bool     `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
```

---

### Task 6: Implement stats package

**Objective:** Thread-safe throughput counter + periodic logger.

**Files:**
- Modify: `internal/stats/stats.go`
- Create: `internal/stats/stats_test.go`

**API:**
- `NewTracker(statsInterval time.Duration) *Tracker`
- `Record(tokens int, latency time.Duration)` — log a completed request
- `Start(ctx)` — launch periodic stats logger goroutine
- `TPS() float64` — current rolling average tokens/sec
- `RPS() float64` — current rolling average requests/sec

Rolling window: keep last 60s of (timestamp, tokens) pairs. Prune on each Record call.

---

### Task 7: Implement OpenAI client

**Objective:** HTTP client that sends chat completion requests and counts response tokens.

**Files:**
- Create: `internal/client/client.go`
- Create: `internal/client/client_test.go`

**API:**
- `NewClient(baseURL, apiKey string, timeout time.Duration) *Client`
- `ChatCompletion(ctx, generator.Request) (tokens int, latency time.Duration, err error)`

Parses `usage.completion_tokens` from the response. For streaming, count SSE delta chunks.

---

### Task 8: Wire main.go — the long-running process

**Objective:** Wire all packages together into a persistent worker with graceful shutdown.

**Files:**
- Modify: `cmd/traffic/main.go`

**Flow:**
1. Load config from env
2. Validate config
3. Create discoverer, fetch models
4. Start periodic model refresh goroutine
5. Create rate limiter, generator, client, stats tracker
6. Main loop: `limiter.Wait()` → `generator.Next()` → `client.ChatCompletion()` → `stats.Record()`
7. SIGINT/SIGTERM → cancel context → drain → exit

---

### Task 9: Add golangci-lint config

**Objective:** `.golangci.yml` with sensible defaults so CI linting passes.

**Files:**
- Create: `.golangci.yml`

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

issues:
  exclude-use-default: false
```

---

### Task 10: Add GitHub Actions CI

**Objective:** CI workflow with all 4 required checks.

**Files:**
- Create: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: go vet
        run: go vet ./...
      - name: go test -race
        run: go test -race ./...
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
      - name: go build
        run: go build ./...
```

---

### Task 11: Add README + AGENTS.md

**Objective:** Documentation for the repo.

**Files:**
- Create: `README.md`
- Create: `AGENTS.md`

---

### Task 12: Create public repo + push initial commit

**Objective:** Create `researchoors/darkbloom-traffic` on GitHub, push, verify CI green.

**Steps:**
1. `gh repo create researchoors/darkbloom-traffic --public --description "Synthetic traffic generator for the Dark Bloom network"`
2. Add remote, push
3. Wait for CI, verify green
