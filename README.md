# Backend Service

Go HTTP service that powers summarization, UV advice, Smart FAQ, and Upload & Ask. The default developer workflow is local-first: Auth, Smart FAQ, and Upload & Ask persistence use SQLite under `data/` without GCP, Cloud Run, remote Postgres, pgvector, Valkey, or R2.

Shared frontend/backend API contract notes live in [`docs/api-contract.md`](docs/api-contract.md). The sibling frontend repository is `/Users/armstrong/Project/ai-helloworld-fe`.

## Getting Started

```bash
# Verify the local recovery contract.
./init.sh

# Required for local auth token signing.
export JWT_SECRET=replace-this-with-a-secure-secret

# Optional: enable real LLM-backed answers.
# Without this, local AI routes use deterministic offline responses.
# export LLM_API_KEY=sk-your-openai-key
# Optional tweaks:
# export LLM_BASE_URL=https://api.openai.com/v1
# export LLM_MODEL=gpt-4o-mini
# export LOG_LEVEL=debug

# Start the server. This creates data/ai-helloworld.db when SQLite is enabled.
make run

# Or run a one-shot local startup/API smoke on a temporary port and database.
make local-smoke
```

Configuration values come from environment variables, `configs/config.yaml`, or defaults in `internal/infra/config`. Local persistence is SQLite by default at `data/ai-helloworld.db`, and SQLite database files are intentionally ignored by git.

`JWT_SECRET` is required for local login and protected-route testing. `LLM_API_KEY` is only required for real LLM-backed summarization, FAQ answers, embeddings, and Upload & Ask responses; without it, local AI routes use deterministic offline responses so the backend can still start for frontend/backend联调. The recovery checks and most local persistence tests run without live AI, OAuth, R2, Postgres, pgvector, Valkey, Redis, or GCP credentials. UV advice uses `UV_API_BASE_URL` (defaults to data.gov.sg) and can be tuned via `UV_PROMPT`.

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

- **Auth**: `JWT_SECRET` secures login tokens and is required for local联调. `LLM_API_KEY` is optional for startup and only needed for real LLM quality. The `/login` frontend route captures email/password/nickname, stores both tokens plus the nickname in `localStorage`, and silently exchanges refresh tokens when the access token expires.
- **Prompt overrides**: Provide `prompt` in the request body to customize ChatGPT instructions; otherwise the default prompt in config is used.
- **Logging**: Set `LOG_LEVEL=debug` to see raw LLM responses (logged before parsing). Logs are JSON to stdout.
- **Timeouts**: HTTP read/write timeouts are configurable under the `http` section in config; ensure they exceed typical LLM latency and embedding latency.
- **Protection**: Rate limits (`http.rateLimit`) and retry behavior (`http.retry`) are configurable so you can tune resiliency per environment.
- **UV Advisor config**: Override `UV_API_BASE_URL` to point at a different data source or `UV_PROMPT` to change how the AI structures advice.
- **Testing**: Run `./init.sh` for the repository recovery contract. It uses temporary Go caches and runs `go test ./...`.
- **Local database**: the default SQLite file is `data/ai-helloworld.db`. Delete it to reset local Auth, FAQ, and Upload & Ask state.
- **Re-run upload document processing** (legacy Redis/Valkey queue only): push a job back onto the queue with the document and user IDs:
  `redis-cli -u "$UPLOADASK_REDIS_ADDR" LPUSH 'uploadask:jobs' '{"name":"process_document","payload":{"document_id":"<doc-uuid>","user_id":<user-id>}}'`
  If the document is marked `failed`, you can reset it first: `UPDATE upload_documents SET status='pending', failure_reason=NULL WHERE id='<doc-uuid>';`
- **Trigger a chat summary** (legacy Redis/Valkey queue): enqueue a `summarize_session` job to force a long-term memory summary for a session (memory must be enabled):
  `redis-cli -u "$UPLOADASK_REDIS_ADDR" LPUSH 'uploadask:jobs' '{"name":"summarize_session","payload":{"session_id":"<session-uuid>","user_id":<user-id>}}'`

## Upload & Ask API

Endpoints live under `/api/v1/upload-ask/*` and require auth:

- Local fallback and response-shape contract: [`docs/upload-ask/local-capability-contract.md`](docs/upload-ask/local-capability-contract.md).
- `POST /documents` (multipart) — upload a file; stored in memory by default, metadata persisted in SQLite locally.
- `GET /documents` — list documents for the user.
- `GET /documents/:id` — fetch document metadata.
- `POST /qa/query` — embed the question, run local SQLite-backed similarity over processed chunks, and call the LLM to answer with citations.
- `GET /qa/sessions` — list previous QA sessions.
- `GET /qa/sessions/:id/logs` — view prior Q/A exchanges.

### Dependencies

- **SQLite**: default local persistence for document metadata, file metadata, chunks, QA sessions, query logs, chat messages, and memories.
- **Valkey/Redis** (legacy optional): queues background document processing (`uploadask:jobs` list). The local default is the immediate in-process queue.
- **Object storage**: in-memory storage by default for local dev. R2 remains available as an optional legacy/integration adapter.
- **Postgres + pgvector**: retained as an optional legacy/integration adapter; it is no longer required for ordinary local use.

### Configuration

Set via `configs/config.yaml` or env:

- `SQLITE_ENABLED` / `SQLITE_PATH` — local persistence toggle and database path; defaults to enabled and `data/ai-helloworld.db`.
- `UPLOADASK_POSTGRES_DSN` — optional legacy Postgres DSN.
- `UPLOADASK_REDIS_ENABLED` / `UPLOADASK_REDIS_ADDR` — optional legacy Valkey/Redis queue.
- `UPLOADASK_STORAGE_*` — optional R2 endpoint/access/secret/bucket; otherwise in-memory.
- `UPLOADASK_VECTOR_DIM` — embedding vector dimension (defaults to 1536 for `text-embedding-3-small`).
- `HTTP_WRITE_TIMEOUT` — ensure this exceeds worst-case embed + chat latency; otherwise clients see socket hangups even if the handler finishes.

### Behavior

1. Upload: store metadata + blob, enqueue processing.
2. Process: chunk text, embed via OpenAI-compatible embeddings, persist chunks in SQLite, mark document processed.
3. Query: embed question, search SQLite-stored embeddings in-process, return top chunks + LLM answer with inline citations.

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

Answer a question using one of four lookup strategies (exact, semantic hash, similarity or hybrid). The local default stores questions, cached answers, and trending counts in SQLite, falls back to the LLM if needed, and records the question for the trending list.

Looking for implementation details? See the [FAQ spec](docs/faq/faq-spec.md) for the ranking heuristics, cache flows, and data contracts shared with the frontend.

Legacy/integration deployments can still connect to Postgres and Valkey/Redis, but `configs/config.yaml` now centers the local SQLite path.

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

The Smart FAQ service uses SQLite locally. A Valkey/Redis-compatible cache remains available for legacy/integration environments; enable it by setting `faq.redis.enabled=true` (see `configs/config.yaml` or the `FAQ_REDIS_*` env vars) and point `faq.redis.addr` at your Valkey connection string. The address may be a raw `host:port` pair or a URL (e.g. `rediss://user:pass@hostname:port/db`).

Environment overrides:

- `FAQ_REDIS_ENABLED=true`
- `FAQ_REDIS_ADDR=rediss://default:***@valkey.example.com:12954`

If the cache is unreachable the service automatically falls back to the in-memory store.

## Project Layout

- `cmd/app`: Wire setup, providers, HTTP server entrypoint.
- `internal/domain`: Core business logic for summarizer, UV advisor, FAQ, auth, and upload-ask.
- `internal/infra`: Integrations (ChatGPT client, SQLite/Postgres repositories, Valkey queues, R2 storage, config loading).
  - `sqlite`: shared local SQLite migrations.
  - `uploadask/*`: chunker, embedder, queue, storage, SQLite repositories, and optional legacy pgvector repositories.
  - `faqrepo/*`: FAQ SQLite repository plus optional legacy pg/pgvector repository.
- `internal/interface/http`: Gin handlers, router, middleware, error handling.
- `configs/config.yaml`: Default runtime configuration (overridable via env).
- `docs/`: Specs and schemas (FAQ, upload-ask, login).

## License

MIT-style; see project owner for details.
