# Backend Service

Go HTTP service that proxies requests to ChatGPT for text summaries and UV-based outfit/protection recommendations. It exposes endpoints under `/api/v1`.

## Getting Started

```bash
# Install deps (Go 1.23+)
go mod download

# Export required env vars
export LLM_API_KEY=sk-your-openai-key
# Optional tweaks:
# export LLM_BASE_URL=https://api.openai.com/v1
# export LLM_MODEL=gpt-4o-mini
# export LOG_LEVEL=debug

# Start the server
make run
```

Configuration values come from environment variables, `configs/config.yaml`, or sane defaults (`internal/infra/config`). At minimum you must set `LLM_API_KEY` so the service can reach OpenAI/ChatGPT. UV advice relies on `UV_API_BASE_URL` (defaults to data.gov.sg) and can be fine-tuned via `UV_PROMPT`.

## API Usage

### POST `/api/v1/summaries`

Sync summarization; returns a JSON payload with `summary` and `keywords`.

```bash
curl --location 'http://localhost:8080/api/v1/summaries' \
  --header 'Content-Type: application/json' \
  --data '{
    "text": "Long input goes here...",
    "prompt": "Optional prompt override"
  }'
```

Response:

```json
{
  "summary": "Shortened text ...",
  "keywords": ["alpha", "beta"]
}
```

### POST `/api/v1/summaries/stream`

Streams partial summaries as Server-Sent Events (SSE). Use `curl -N` (no buffering) or any SSE client to see each chunk as it arrives.

```bash
curl -N --location 'http://localhost:8080/api/v1/summaries/stream' \
  --header 'Content-Type: application/json' \
  --data '{
    "text": "Same input as sync endpoint"
  }'
```

Output shape:

```
data: {"partial_summary":"First chunk"}

data: {"partial_summary":"...", "completed":true, "keywords":["alpha","beta"]}
```

### Error Format

All errors use:

```json
{
  "error": {
    "code": "summarize_failed",
    "message": "text cannot be empty"
  }
}
```

## Tips & Operational Notes

- **Auth**: `LLM_API_KEY` is mandatory; requests fail if it’s empty.
- **Prompt overrides**: Provide `prompt` in the request body to customize ChatGPT instructions; otherwise the default prompt in config is used.
- **Logging**: Set `LOG_LEVEL=debug` to see raw LLM responses (logged before parsing). Logs are JSON to stdout.
- **Timeouts**: HTTP read/write timeouts are configurable under the `http` section in config; ensure they exceed typical LLM latency.
- **Protection**: Rate limits (`http.rateLimit`) and retry behavior (`http.retry`) are configurable so you can tune resiliency per environment.
- **UV Advisor config**: Override `UV_API_BASE_URL` to point at a different data source or `UV_PROMPT` to change how the AI structures advice.
- **Testing**: Run `GOCACHE=$(pwd)/.gocache go test ./...` to avoid sandbox cache issues.

## UV Advisor API

### POST `/api/v1/uv-advice`

Fetches UV readings from data.gov.sg (or a custom `UV_API_BASE_URL`) and asks ChatGPT to return clothing/protection suggestions.

```bash
curl --location 'http://localhost:8080/api/v1/uv-advice' \
  --header 'Content-Type: application/json' \
  --data '{
    "date": "2024-07-01"
  }'
```

Response:

```json
{
  "date": "2024-07-01",
  "category": "very_high",
  "maxUv": 8,
  "peakHour": "2024-07-01T13:00:00+08:00",
  "summary": "Concise recap of the day",
  "clothing": ["Lightweight long sleeves", "Breathable trousers"],
  "protection": ["SPF 50 sunscreen", "Wide-brim hat"],
  "tips": ["Stay hydrated"],
  "readings": [
    { "hour": "2024-07-01T07:00:00+08:00", "value": 0 }
  ],
  "source": "https://api-open.data.gov.sg/v2/real-time/api/uv",
  "dataTimestamp": "2024-07-01T19:00:00+08:00"
}
```

Under the hood the UV advisor uses an OpenAI function call (`get_sg_uv`) so the model explicitly retrieves the latest data.gov.sg payload before composing its JSON summary.

## Project Layout

- `cmd/app`: Wire setup and entrypoint.
- `internal/domain/summarizer`: Core business logic + ChatGPT integration.
- `internal/interface/http`: Gin handlers, router, middleware.
- `configs/config.yaml`: Default runtime configuration.

## License

MIT-style; see project owner for details.
