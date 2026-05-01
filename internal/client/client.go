package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/researchoors/darkbloom-traffic/internal/generator"
)

// Client is an OpenAI-compatible HTTP client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Client with the given base URL, API key, and request timeout.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// chatResponse represents a non-streaming OpenAI chat completion response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// streamChunk represents a single SSE chunk in a streaming response.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ChatCompletion sends a chat completion request and returns the number of
// completion tokens and the request latency. It handles both streaming and
// non-streaming responses based on req.Stream.
func (c *Client) ChatCompletion(ctx context.Context, req generator.Request) (tokens int, latency time.Duration, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	latency = time.Since(start)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, latency, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	if req.Stream {
		tokens, err = c.parseStream(resp.Body)
	} else {
		tokens, err = c.parseNonStream(resp.Body)
	}

	return tokens, latency, err
}

// parseNonStream reads the full response body and extracts usage.completion_tokens.
func (c *Client) parseNonStream(r io.Reader) (int, error) {
	var cr chatResponse
	if err := json.NewDecoder(r).Decode(&cr); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return cr.Usage.CompletionTokens, nil
}

// parseStream reads SSE lines from the response body, parses each chunk,
// and returns the completion_tokens from the final chunk that includes usage.
func (c *Client) parseStream(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	var tokens int

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and non-data lines.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// The stream ends with "data: [DONE]".
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return 0, fmt.Errorf("decode stream chunk: %w", err)
		}

		if chunk.Usage != nil {
			tokens = chunk.Usage.CompletionTokens
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read stream: %w", err)
	}

	return tokens, nil
}
