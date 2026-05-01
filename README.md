# Dark Bloom Traffic Generator

Synthetic traffic generator for the Dark Bloom network.

## What It Does

Generates a persistent stream of LLM requests at 4-5 RPS across all network models. This tool continuously sends synthetic traffic to stress test, benchmark, or warm up the Dark Bloom inference network.

## Quick Start

Set the required environment variables:

```bash
export DARKBLOOM_BASE_URL=https://api.darkbloom.ai
export DARKBLOOM_API_KEY=your-api-key-here
```

Then run:

```bash
go run ./cmd/traffic
```

## Configuration

All configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DARKBLOOM_BASE_URL` | Base URL for the Dark Bloom API | *(required)* |
| `DARKBLOOM_API_KEY` | API key for authentication | *(required)* |
| `DARKBLOOM_TARGET_TPS` | Target tokens per second | `500` |
| `DARKBLOOM_MAX_RPS` | Maximum requests per second | `4.5` |

## Architecture

The traffic generator follows a pipeline architecture:

```
┌────────────┐    ┌─────────────┐    ┌───────────┐    ┌────────┐    ┌───────┐
│ Discoverer │ -> │ Rate Limiter │ -> │ Generator │ -> │ Client │ -> │ Stats │
└────────────┘    └─────────────┘    └───────────┘    └────────┘    └───────┘
```

- **Discoverer**: Discovers available models from the network
- **Rate Limiter**: Controls request throughput to stay within target RPS
- **Generator**: Creates synthetic prompts and request payloads
- **Client**: Sends HTTP requests to the Dark Bloom API
- **Stats**: Collects and reports traffic statistics

## License

MIT License
