# Upload & Ask Local Capability Contract

Upload & Ask is local-first in this repository. A normal developer checkout must be understandable, runnable, and testable without GCP, Cloud Run, remote Postgres, pgvector, Valkey, Redis, R2, or live embedding credentials.

## Local Defaults

- Metadata persistence: SQLite, enabled by default at `data/ai-helloworld.db`.
- Blob storage: in-memory object storage when R2/S3 settings are incomplete.
- Document processing queue: immediate in-process queue when Valkey/Redis is not enabled.
- Embeddings: deterministic local embedder when a ChatGPT/OpenAI-compatible embedding client or embedding model is unavailable.
- Answers: Echo LLM fallback when the ChatGPT/OpenAI-compatible client is unavailable.
- Conversation messages and memories: SQLite when local SQLite is enabled; in-memory only if SQLite and legacy Postgres are both unavailable.

These defaults are intended for local recovery, API contract verification, and deterministic tests. They are not presented as production replacements for durable blob storage or a live LLM.

## Capability Gaps

| Capability | Local behavior | Durable gap when missing |
| --- | --- | --- |
| Postgres + pgvector | Not required for local mode; SQLite stores chunks and performs deterministic in-process similarity over JSON embeddings. | Required only for legacy/integration verification of the old pgvector adapter. A missing DSN or pgvector extension should be treated as an integration capability gap, not hidden by a local-only test. |
| Valkey/Redis | Not required for local mode; the immediate queue can process jobs in-process. | Required only for legacy queue integration. A missing address means Redis queue behavior is unverified. |
| R2/S3 | Not required for local mode; uploaded bytes stay in memory. | Required only for object-storage integration. Missing endpoint, bucket, or keys means R2 persistence and lifecycle behavior are unverified. |
| Embedding credentials | Not required for deterministic local tests; the hash embedder provides stable vectors. | Required for live retrieval quality checks. Missing credentials mean semantic quality and provider latency are unverified. |
| LLM credentials | Not required for deterministic local tests; Echo LLM keeps the API shape stable. | Required for real answer quality checks. Missing credentials mean answer quality and token usage are unverified. |

When a feature depends on a legacy/integration adapter, the run record should name the missing service as a capability gap. Local tests should not silently claim coverage for a service that was not configured.

## Response Shapes

The frontend depends on these Upload & Ask JSON shapes under `/api/v1/upload-ask`:

- `POST /documents` returns `{"document": Document}` with `id`, `userId`, `title`, `source`, `status`, `failureReason?`, `createdAt`, and `updatedAt`.
- `GET /documents` returns `{"items": Document[]}` and supports `status=pending,processing,processed,failed`.
- `GET /documents/:id` returns one `Document`; status moves through `pending`, `processing`, `processed`, or `failed`.
- `POST /qa/query` accepts `query`, optional `sessionId`, optional `documentIds`, `topK`, `topKMems`, `maxHistoryTokens`, and `includeHistory`; it returns `sessionId`, `answer`, `sources`, `memories?`, `usedHistoryTokens`, and `latencyMs`.
- `sources[]` contains citation fields `documentId`, `chunkIndex`, `score`, and `preview`.
- `GET /qa/sessions` returns `{"sessions": QASession[]}`.
- `GET /qa/sessions/:id/logs` returns `{"logs": QueryLog[]}` with `sessionId`, `queryText`, `responseText`, `latencyMs`, `sources`, and `createdAt`.

## Deterministic Verification

`TestRouter_UploadAskLocalContractSmoke` exercises the HTTP API with memory blob storage, memory repositories, deterministic embeddings, and Echo LLM. It verifies document upload, processing status, QA session creation, citation response fields, and query log response fields without live external services.

Root `./init.sh` runs this smoke as part of the normal Go test suite.
