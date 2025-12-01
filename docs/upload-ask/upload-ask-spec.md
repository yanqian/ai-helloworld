# Upload-and-Ask Specification (Postgres-first, R2/Supabase-ready)

Goal: deliver a cheap, incremental workflow to upload files, embed them, and answer questions with a clean architecture that swaps storage/LLM/embedding providers easily.

## 1) Architecture & Layers
- **Domain**: entities, value objects, interfaces (ports) for storage, embedding, LLM, queue, repositories.
- **Application**: use-cases (uploadDocument, processDocument, askQuestion), transactional boundaries, orchestration; depends only on domain interfaces.
- **Infrastructure**: Postgres/pgvector repos, object storage adapter (S3/R2/Supabase), embedder client, LLM client, queue impl, text extraction/chunker.
- **Interfaces**: HTTP handlers (REST/GraphQL), worker entrypoints, CLI/cron; thin validation and mapping.
- **Frontend**: React 18 + TypeScript + Vite + React Router + Tailwind + Zustand; upload + chat + history views; calls HTTP APIs via typed fetch helpers.
- **DI/Wiring**: composition root wires concrete infra into app; interfaces enable swapping (R2 vs Supabase, OpenAI vs local, etc.).

## 2) Domain Models
- `Document { id: uuid, user_id, title, source: (upload|url), status: (pending|processing|processed|failed), failure_reason?, created_at, updated_at }`
- `FileObject { id: uuid, document_id, storage_key, size_bytes, mime_type, etag, created_at }`
- `DocumentChunk { id: uuid, document_id, chunk_index: int, content: text, token_count: int, embedding: vector, created_at }`
- `QASession { id: uuid, user_id, created_at }`
- `QueryLog { id: uuid, session_id, query_text, response_text, latency_ms, created_at }`
- Multi-tenant: prefer `workspace_id` over `user_id` if needed later.

## 3) Persistence (Postgres + pgvector)
- Tables:
  - `documents`: pk uuid, user_id, title, source, status enum, failure_reason, created_at, updated_at
  - `file_objects`: pk uuid, document_id fk, storage_key, size_bytes int8, mime_type, etag, created_at
  - `document_chunks`: pk uuid, document_id fk, chunk_index int, content text, token_count int, embedding vector(1536 or 3072), created_at
  - `qa_sessions`: pk uuid, user_id, created_at
  - `query_logs`: pk uuid, session_id fk, query_text text, response_text text, latency_ms int, created_at
- Indexes: ivfflat/ls for `document_chunks.embedding` (cosine or l2); btree on document_id, user_id; partial index on documents(status='processed'); unique on (document_id, chunk_index).
- Migrations: managed tool (e.g., Atlas/Flyway/Goose/Prisma/Kysely). Add migration tests.

## 4) Interfaces (Ports)
- `ObjectStorage { put(key, bytes, mime): Meta; getStream(key): Stream; delete(key): void }`
- `Embedder { embed(texts: string[]): float[][] }`
- `LLM { chat(messages, options): string }`
- `Retriever { retrieve(query_embedding, filters?): Chunk[] }` (infra uses pgvector)
- `JobQueue { enqueue(name, payload); poll(name): Job; ack(job); fail(job, reason) }`
- Repositories: `DocumentRepo`, `ChunkRepo`, `FileObjectRepo`, `QASessionRepo`, `QueryLogRepo`.

## 5) Upload → Process Pipeline
1) **Upload**: HTTP multipart; store raw file via `ObjectStorage.put` using key `uploads/{user}/{doc}/{filename}`; record `Document` (status=pending) + `FileObject` + ETag for dedup.
2) **Enqueue**: `JobQueue.enqueue(process_document, { document_id })`.
3) **Worker**:
   - Load file stream; text extraction (PDF/Doc/TXT) with size/type allowlist.
   - Chunk by tokens (e.g., 500–800 tokens, 50–100 overlap); keep token_count.
   - `Embedder.embed` batched with max tokens guard; retry with backoff.
   - Insert `document_chunks` (idempotent on document_id + chunk_index) in batch transaction.
   - Update document status to processed; on error set failed + failure_reason.
4) **Cost controls**: max file size (e.g., 20MB), per-user daily token cap, dedup by ETag, rate limit uploads.

## 6) Query Flow
- Input: `user_id`, `query_text`, optional `session_id`, filters (document_ids, status processed only).
- Steps: validate → embed query → pgvector similarity (`k=8–12`, similarity threshold) → optional MMR rerank → assemble context (cap tokens) → `LLM.chat` for RAG → log `QueryLog` with latency and citations.
- Output: answer + sources `{ document_id, chunk_index, score, preview }`.
- Safety: max query length, max retrieved chunks, rate limit queries, redact PII in logs.

## 7) APIs (REST exemplar)
- `POST /api/documents` (multipart file) → { document_id, status }
- `GET /api/documents/:id` → metadata + status + failure_reason
- `GET /api/documents` → paginated list, filter by status
- `POST /api/qa/query` { query, session_id?, filters? } → { answer, sources[] }
- `GET /api/qa/sessions/:id/logs` → past queries/answers
- Health: `/health`, `/ready`; Auth: bearer/JWT; scope queries/storage by user/workspace.

## 8) Frontend (helloworld-fe stack)
- Stack: React 18 + TypeScript + Vite + React Router v6; Tailwind utilities for styling; Zustand for global app/session state; clsx for class merges; typed fetch wrappers for APIs.
- Layout: `App` routes for `/upload`, `/ask`, `/history`; shared layout (nav/status) in `components/layout`.
- Views:
  - Upload: drag/drop or file picker; list of documents with status chips; polling `/api/documents/:id`; show size/type/errors.
  - Ask: chat UI with document filters; display citations (title + preview + score); latency chip; disable send while pending.
  - History: list sessions; load past answers and sources.
- State/async: local state with hooks; global session/doc filters in Zustand; API hooks wrap fetch with abort/timeouts; keep derived loading/error flags.
- Styling/UX: Tailwind tokens + CSS variables for theme; focus-visible, keyboard nav, ARIA labels; toasts for errors; skeletons/spinners for loading.

## 9) Testing
- Backend unit: use-cases (uploadDocument, processDocument, askQuestion), chunker, rate-limit/quota logic; mock Embedder/LLM/Storage/Queue.
- Backend integration: migrations, pgvector similarity, storage adapter (local mock), API handlers with test DB.
- Frontend: Jest + React Testing Library for pages/components (upload flow, chat message send/disable states, citation rendering); store hooks with Zustand using test helpers.
- E2E (light): upload → worker → query returns expected chunk (mock embedder/LLM); can be Playwright/Cypress later.
- Contract tests for `ObjectStorage` and `Embedder` adapters to ensure swap safety.

## 10) Observability & Ops
- Structured logs with request_id/job_id; log step latencies.
- Metrics: job durations, queue depth, embedding latency, query latency, chunk counts, token usage.
- Tracing hooks around DB/LLM/storage; add `/health` and `/ready` probes.

## 11) Config
- Use GitHub Actions variables/secrets (no `.env` files committed). Name suggestions:
  - Secrets: `DB_URL`, `LLM_API_KEY`, `EMBEDDINGS_API_KEY`, `R2_ENDPOINT`, `R2_ACCESS_KEY`, `R2_SECRET_KEY`, `QUEUE_URL`, `JWT_SECRET`.
  - Vars: `VECTOR_DIM`, `MAX_FILE_MB`, `DAILY_EMBED_TOKENS`, `ENABLE_RATE_LIMITS`, `ENABLE_MMR`, `USE_MOCK_EMBEDDINGS=false`.
- Map these into the runtime config loader in Go and into the frontend build-time env injection (Vite `import.meta.env` via action).

## 12) R2 Storage (greenfield)
- Default storage provider: Cloudflare R2 via S3-compatible API; configure bucket/endpoint/keys via GitHub secrets.
- Use presigned PUT for large files if needed; otherwise direct upload through the backend proxy is fine for MVP.
- Apply lifecycle rules later (expiration/cold storage) when volume grows; no migration path needed.

## 13) Delivery Plan (MVP first)
1) DB schema + migrations (pgvector).
2) Storage adapter (S3-compatible for R2/Supabase) + upload endpoint.
3) Worker: extract → chunk → embed → store chunks; status updates + retries.
4) Query endpoint: retrieval + LLM call + logging.
5) Frontend upload + ask UI; basic history.
6) Tests (unit/integration/E2E) + metrics/health.

## 14) Integration: ai-helloworld (Go backend)
- **API namespace**: mount under `/api/v1/upload-ask` to match existing `/api/v1` routes; reuse existing auth middleware (JWT) and error envelope.
- **Packages**: keep everything under `internal/domain/uploadask` (entities, interfaces, service/use-cases), `internal/infra/uploadask` (pg/pgvector repos, storage adapter, embedder/LLM clients, queue), and HTTP handlers in `internal/interface/http/uploadask`.
- **Config**: extend `configs/config.yaml` + env overrides for `uploadask.*` (DB DSN, storage S3 endpoint/key/secret/bucket, embedding/LLM keys, limits). Wire defaults in `internal/infra/config`.
- **Queue backend**: Valkey/Redis (`uploadask.redis.*`) with fallback to in-memory immediate queue for local/dev.
- **Queue/worker**: if no existing worker process, add a `cmd/worker` entrypoint that reuses the same DI wiring; if reusing `cmd/app`, start worker as a background goroutine only when enabled by config flag.
- **Logging/metrics**: reuse logger and middleware; add Prometheus counters/histograms under existing metrics endpoint if present.
- **Auth/tenancy**: scope documents/files/chunks by authenticated user/workspace id from the existing context helper; enforce in queries and storage keys.

## 15) Integration: ai-helloworld-fe (React/Vite)
- **Routing**: add a protected route `/upload-ask` in `AppRoutes` using `ProtectedRoute`; keep default redirect to `/` intact.
- **Feature module**: create `src/features/uploadAsk` with `pages` (UploadPage, AskPage, HistoryPage or a tabbed single page), `components` (Dropzone, DocumentList, ChatPanel, CitationList), `api.ts` (typed fetch helpers hitting `/api/v1/upload-ask/...`), and `store.ts` (Zustand slices for documents, sessions, ask state).
- **Navigation**: surface the feature alongside Summarizer/UV/Smart FAQ in the shared layout/header.
- **Styling/UX**: reuse Tailwind + clsx conventions; keep skeletons/spinners consistent with existing features; maintain ARIA labels and keyboard focus patterns already used in auth/smart-faq pages.
- **Auth/token reuse**: reuse the existing auth provider/localStorage tokens; API helpers should attach the Bearer token like other services.
- **Tests**: Jest + RTL for upload list rendering, status polling, send/disable behavior in chat, and citation rendering; mocks for fetch with happy/error paths.
