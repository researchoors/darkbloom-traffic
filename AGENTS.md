# Dark Bloom Traffic Generator - AI Agent Guide

## Project Overview

This is a Go CLI tool for generating synthetic LLM traffic against the Dark Bloom network. It produces a persistent stream of requests at configurable rates (default 4-5 RPS) across all available models.

## Directory Structure

```
darkbloom-traffic/
├── cmd/
│   └── traffic/        # Main application entry point
├── internal/
│   ├── config/         # Configuration loading and validation
│   ├── models/         # Model discovery and management
│   ├── ratelimit/      # Rate limiting implementation
│   ├── generator/      # Request payload generation
│   ├── client/         # HTTP client for API communication
│   └── stats/          # Statistics collection and reporting
├── README.md           # User documentation
└── AGENTS.md           # This file
```

## Key APIs Per Package

### `internal/config`
- `Load() (*Config, error)` - Loads configuration from environment variables
- `Config` struct holds all runtime configuration

### `internal/models`
- `Discoverer` - Interface for model discovery
- `Discover(baseURL string) ([]Model, error)` - Fetches available models from the API
- `Model` struct represents a deployable model

### `internal/ratelimit`
- `Limiter` - Token bucket rate limiter
- `NewLimiter(rps float64) *Limiter` - Creates a new rate limiter
- `Wait(ctx context.Context) error` - Blocks until a request is allowed

### `internal/generator`
- `Generator` - Generates synthetic request payloads
- `NewGenerator(models []Model) *Generator` - Creates a generator for given models
- `Generate() (*Request, error)` - Produces a new request payload

### `internal/client`
- `Client` - HTTP client wrapper for Dark Bloom API
- `NewClient(baseURL, apiKey string) *Client` - Creates an authenticated client
- `Send(ctx context.Context, req *Request) (*Response, error)` - Sends a request

### `internal/stats`
- `Collector` - Aggregates traffic statistics
- `NewCollector() *Collector` - Creates a new stats collector
- `Record(latency time.Duration, success bool)` - Records a request result
- `Report() string` - Returns formatted statistics

## Development Commands

### Run Tests
```bash
go test -race ./...
```

### Run Linter
```bash
golangci-lint run
```

### Build
```bash
go build ./...
```

### Build Binary
```bash
go build -o darkbloom-traffic ./cmd/traffic
```

## Running the Application

```bash
# Set required environment variables
export DARKBLOOM_BASE_URL=https://api.darkbloom.ai
export DARKBLOOM_API_KEY=your-api-key

# Run directly
go run ./cmd/traffic

# Or build and run
go build -o darkbloom-traffic ./cmd/traffic
./darkbloom-traffic
```
