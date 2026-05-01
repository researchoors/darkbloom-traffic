package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/researchoors/darkbloom-traffic/internal/generator"
)

func TestChatCompletionNonStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify Authorization header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", auth)
		}

		// Verify Content-Type.
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		// Decode request to verify Stream=false.
		var req generator.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Stream {
			t.Error("expected Stream=false")
		}

		// Return a non-streaming response with usage.completion_tokens=42.
		resp := `{
			"choices": [{"message": {"content": "Hello"}}],
			"usage": {"completion_tokens": 42}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 10*time.Second)
	req := generator.Request{
		Model:    "gpt-4",
		Messages: []generator.Message{{Role: "user", Content: "hi"}},
		Stream:   false,
	}

	tokens, _, err := client.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens != 42 {
		t.Errorf("expected tokens=42, got %d", tokens)
	}
}

func TestChatCompletionStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode request to verify Stream=true.
		var req generator.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !req.Stream {
			t.Error("expected Stream=true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming chunks with content deltas.
		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world"}}]}`,
			`data: {"choices":[{"delta":{"content":"!"}}],"usage":{"completion_tokens":7}}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 10*time.Second)
	req := generator.Request{
		Model:    "gpt-4",
		Messages: []generator.Message{{Role: "user", Content: "hi"}},
		Stream:   true,
	}

	tokens, _, err := client.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens != 7 {
		t.Errorf("expected tokens=7, got %d", tokens)
	}
}

func TestChatCompletionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", 10*time.Second)
	req := generator.Request{
		Model:    "gpt-4",
		Messages: []generator.Message{{Role: "user", Content: "hi"}},
		Stream:   false,
	}

	_, _, err := client.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
