# Backend Service

Go HTTP service that powers summarization, UV advice, Smart FAQ, and Upload & Ask (pgvector-backed RAG). It exposes endpoints under `/api/v1`.

## Getting Started

```bash
# Install deps (Go 1.23+)
go mod download

# Export required env vars
export LLM_API_KEY=sk-your-openai-key
export JWT_SECRET=replace-this-with-a-secure-secret
# Optional tweaks:
# export LLM_BASE_URL=https://api.openai.com/v1
# export LLM_MODEL=gpt-4o-mini
# export LOG_LEVEL=debug

# Start the server
make run
```

Configuration values come from environment variables, `configs/config.yaml`, or sane defaults (`internal/infra/config`). At minimum you must set `LLM_API_KEY` so the service can reach OpenAI/ChatGPT. UV advice relies on `UV_API_BASE_URL` (defaults to data.gov.sg) and can be fine-tuned via `UV_PROMPT`. Upload & Ask requires Postgres + pgvector and optional Valkey/Redis for the background queue.

## API Usage

### Authentication

Register once with an email/password/nickname (nickname must be <=10 letters). The backend hashes passwords and returns both an access token (1h TTL) plus a refresh token (24h TTL by default). Refresh tokens are exchanged silently by the frontend whenever the access token expires.

```bash
curl --location 'http://localhost:8080/api/v1/auth/register' \
  --header 'Content-Type: application/json' \
  --data '{"email":"user@example.com","password":"password123","nickname":"CodeStar"}'

TOKEN=$(curl --silent --location 'http://localhost:8080/api/v1/auth/login' \
  --header 'Content-Type: application/json' \
  --data '{"email":"user@example.com","password":"password123"}')

ACCESS_TOKEN=$(echo "$TOKEN" | jq -r '.token')
REFRESH_TOKEN=$(echo "$TOKEN" | jq -r '.refreshToken')
```

You can verify a token (and fetch the greeting shown on the dashboard) with:

```bash
curl --location 'http://localhost:8080/api/v1/auth/me' \
  --header "Authorization: Bearer $ACCESS_TOKEN"
```

Response:

```json
{
  "message": "Welcome to the private dashboard",
  "user": {
    "email": "user@example.com",
    "nickname": "CodeStar"
  }
}
```

If the access token expires you can manually refresh it (the frontend handles this automatically):

```bash
curl --location 'http://localhost:8080/api/v1/auth/refresh' \
  --header 'Content-Type: application/json' \
  --data "{\"refreshToken\":\"$REFRESH_TOKEN\"}"
```

### POST `/api/v1/summaries`

Sync summarization; returns a JSON payload with `summary` and `keywords`.

```bash
curl --location 'http://localhost:8080/api/v1/summaries' \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer $ACCESS_TOKEN" \
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
  --header "Authorization: Bearer $ACCESS_TOKEN" \
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

- **Auth**: `JWT_SECRET` secures login tokens and `LLM_API_KEY` is mandatory for LLM access. The `/login` frontend route captures email/password/nickname, stores both tokens plus the nickname in `localStorage`, and silently exchanges refresh tokens when the access token expires.
- **Prompt overrides**: Provide `prompt` in the request body to customize ChatGPT instructions; otherwise the default prompt in config is used.
- **Logging**: Set `LOG_LEVEL=debug` to see raw LLM responses (logged before parsing). Logs are JSON to stdout.
- **Timeouts**: HTTP read/write timeouts are configurable under the `http` section in config; ensure they exceed typical LLM latency and embedding latency.
- **Protection**: Rate limits (`http.rateLimit`) and retry behavior (`http.retry`) are configurable so you can tune resiliency per environment.
- **UV Advisor config**: Override `UV_API_BASE_URL` to point at a different data source or `UV_PROMPT` to change how the AI structures advice.
- **Testing**: Run `GOCACHE=$(pwd)/.gocache go test ./...` to avoid sandbox cache issues.
- **Re-run upload document processing** (Redis/Valkey queue only): push a job back onto the queue with the document and user IDs:
  `redis-cli -u "$UPLOADASK_REDIS_ADDR" LPUSH 'uploadask:jobs' '{"name":"process_document","payload":{"document_id":"<doc-uuid>","user_id":<user-id>}}'`
  If the document is marked `failed`, you can reset it first: `UPDATE upload_documents SET status='pending', failure_reason=NULL WHERE id='<doc-uuid>';`

## Upload & Ask API (pgvector)

Endpoints live under `/api/v1/upload-ask/*` and require auth:

- `POST /documents` (multipart) — upload a file; stored in R2/memory, metadata persisted in Postgres.
- `GET /documents` — list documents for the user.
- `GET /documents/:id` — fetch document metadata.
- `POST /qa/query` — embed the question, run pgvector similarity over processed chunks, and call the LLM to answer with citations.
- `GET /qa/sessions` — list previous QA sessions.
- `GET /qa/sessions/:id/logs` — view prior Q/A exchanges.

### Dependencies

- **Postgres + pgvector**: required for document/chunk storage. The pool registers the `vector` type automatically; pgvector must be installed (`CREATE EXTENSION vector`).
- **Valkey/Redis** (optional): queues background document processing (`uploadask:jobs` list) and can be disabled to process synchronously.
- **Object storage**: R2 adapter; falls back to in-memory storage for local dev.

### Configuration

Set via `configs/config.yaml` or env:

- `UPLOADASK_POSTGRES_DSN` — Postgres DSN (required for persistence).
- `UPLOADASK_REDIS_ADDR` — enables Valkey/Redis queue when set.
- `UPLOADASK_STORAGE_*` — R2 endpoint/access/secret/bucket (otherwise in-memory).
- `UPLOADASK_VECTOR_DIM` — embedding vector dimension (defaults to 1536 for `text-embedding-3-small`).
- `HTTP_WRITE_TIMEOUT` — ensure this exceeds worst-case embed + chat latency; otherwise clients see socket hangups even if the handler finishes.

### Behavior

1. Upload: store metadata + blob, enqueue processing.
2. Process: chunk text, embed via OpenAI-compatible embeddings, persist chunks with pgvector, mark document processed.
3. Query: embed question, search pgvector, return top chunks + LLM answer with inline citations.

## UV Advisor API

### POST `/api/v1/uv-advice`

Fetches UV readings from data.gov.sg (or a custom `UV_API_BASE_URL`) and asks ChatGPT to return clothing/protection suggestions.

```bash
curl --location 'http://localhost:8080/api/v1/uv-advice' \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer $ACCESS_TOKEN" \
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

## Smart FAQ API

### POST `/api/v1/faq/search`

Answer a question using one of four lookup strategies (exact, semantic hash, similarity or hybrid). The service checks Redis-backed (or in-memory) caches first, falls back to the LLM if needed, and records the question for the trending list.

Looking for implementation details? See the [FAQ spec](docs/faq/faq-spec.md) for the ranking heuristics, cache flows, and data contracts shared with the frontend.

Production deployments connect to Aiven-managed Postgres (for long-term FAQ storage) and Valkey/Redis (for the FAQ cache). The defaults in `configs/config.yaml` map directly to that setup—override them only if you run your own databases.

```bash
curl --location 'http://localhost:8080/api/v1/faq/search' \
  --header 'Content-Type: application/json' \
  --header "Authorization: Bearer $ACCESS_TOKEN" \
  --data '{
    "question": "How far is the moon?",
    "mode": "hybrid"
  }'
```

Response:

```json
{
  "question": "How far is the moon?",
  "matchedQuestion": "How far is the moon?",
  "answer": "About 384,400 km separate Earth and the Moon.",
  "source": "cache",
  "mode": "exact",
  "recommendations": [
    { "query": "How far is the moon?", "count": 4 }
  ]
}
```

### GET `/api/v1/faq/trending`

Returns the top 10 most common FAQ searches to power the recommendation list in the UI.

### FAQ Cache Backend

The Smart FAQ service uses a Valkey/Redis-compatible cache before calling the LLM. Enable it by setting `faq.redis.enabled=true` (see `configs/config.yaml` or the `FAQ_REDIS_*` env vars) and point `faq.redis.addr` at your Valkey connection string. The address may be a raw `host:port` pair or a URL (e.g. `rediss://user:pass@hostname:port/db`).

Environment overrides:

- `FAQ_REDIS_ENABLED=true`
- `FAQ_REDIS_ADDR=rediss://default:***@valkey.example.com:12954`

If the cache is unreachable the service automatically falls back to the in-memory store.

## Project Layout

- `cmd/app`: Wire setup, providers, HTTP server entrypoint.
- `internal/domain`: Core business logic for summarizer, UV advisor, FAQ, auth, and upload-ask.
- `internal/infra`: Integrations (ChatGPT client, pgvector repos, Valkey queues, R2 storage, config loading).
  - `uploadask/*`: chunker, embedder, queue, storage, and pgvector repositories.
  - `faqrepo/*`: FAQ pg/pgvector repository.
- `internal/interface/http`: Gin handlers, router, middleware, error handling.
- `configs/config.yaml`: Default runtime configuration (overridable via env).
- `docs/`: Specs and schemas (FAQ, upload-ask, login).

## License

MIT-style; see project owner for details.
