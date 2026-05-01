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

// Model represents an LLM model available via the API.
type Model struct {
	ID      string
	OwnedBy string
}

// Discoverer fetches and caches the list of available models from /v1/models.
type Discoverer struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	mu         sync.RWMutex
	models     []Model
}

// NewDiscoverer creates a Discoverer that queries baseURL with the given apiKey.
// The internal HTTP client has a 30-second timeout.
func NewDiscoverer(baseURL, apiKey string) *Discoverer {
	return &Discoverer{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// modelsResponse maps the JSON shape returned by /v1/models.
type modelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// Refresh fetches the model list from /v1/models and stores it locally.
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
	defer func() { _ = resp.Body.Close() }()

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

// Models returns a copy of the current model list (thread-safe).
func (d *Discoverer) Models() []Model {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Model, len(d.models))
	copy(out, d.models)
	return out
}

// Count returns the number of currently cached models (thread-safe).
func (d *Discoverer) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.models)
}
