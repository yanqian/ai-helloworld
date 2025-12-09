# Upload-and-Ask Conversational Memory & Vector Memory Spec

Goal: add multi-turn memory so Ask can ground answers in recent conversation + long-term vectorized memories alongside document chunks.

## 1) Objectives
- Preserve short-term chat context across turns (session-scoped) with token budgeting.
- Persist long-term memories as semantic vectors to resurface relevant prior Q&A or facts.
- Keep retrieval composition predictable: docs + memories + trimmed history.
- Stay swappable: use existing ports-first design; memory store behind an interface.

## 2) Domain Additions
- `ConversationMessage { id uuid, session_id uuid, user_id int64, role (user|assistant|system), content text, token_count int, created_at time }`
- `MemoryRecord { id uuid, session_id uuid, user_id int64, source (qa_turn|summary|manual), content text, embedding vector(cfg.VectorDim), importance smallint, created_at time }`
- Interfaces:
  - `MessageLog { Append(ctx, msg ConversationMessage) error; ListRecent(ctx, userID, sessionID, maxTokens, maxMessages) ([]ConversationMessage, error) }`
  - `MemoryStore { Upsert(ctx, mem MemoryRecord) error; Search(ctx, userID, sessionID uuid, embedding []float32, k int) ([]MemoryRecord, error); Prune(ctx, userID int64, sessionID *uuid.UUID, limit int) error }`
  - Reuse existing `ChunkRepository` search for documents; keep `QASessionRepository` as-is.

## 3) Persistence (Postgres + pgvector)
- Tables:
  - `upload_qa_messages`: pk uuid, session_id fk qa_sessions(id), user_id, role text, content text, token_count int, created_at timestamptz; indexes on (session_id, created_at desc).
  - `upload_qa_memories`: pk uuid, session_id fk, user_id, source text, content text, importance smallint default 0, embedding vector(cfg.VectorDim), created_at timestamptz; ivfflat index on embedding scoped by user_id; secondary index on (session_id, created_at desc).
- Migration strategy: additive migrations via existing tool; backfill: derive memories from past `query_logs` by embedding question+answer pairs.

## 4) Request/Response Shape
- Extend `POST /api/v1/upload-ask/query` (existing Ask) payload:
  - `sessionId?` (reuse), `topKDocs?`, `topKMems?`, `maxHistoryTokens?`, `includeHistory?` default true.
- Response additions:
  - `memories`: `{ id, source, preview, score }[]`
  - `usedHistoryTokens`: int for observability.
- Back-compat: defaults keep current behavior when `includeHistory` is false and `topKMems` is zero.

## 5) Query Pipeline (per request)
1) **Input**: latest user message + sessionId; fetch recent message history (trim by token budget).
2) **Query construction**:
   - Build a semantic query string: concatenate latest user msg + brief summary of trimmed history (or last assistant answer) → `semantic_query`.
   - Embed `semantic_query` for both doc search and memory search.
3) **Retrieval**:
   - Docs: `chunks.SearchSimilar` with `k=topKDocs`, existing filters.
   - Memories: `memoryStore.Search` with same embedding, `k=topKMems`; optional recency/importance boost.
4) **Context assembly** (token-capped):
   - System instructions (cite docs/mems; refuse hallucination).
   - Relevant doc chunks (with doc+chunk ids).
   - Relevant memories (content + source + score).
   - Short conversation history (trimmed) or precomputed summary.
   - Latest user question (verbatim).
5) **Generation**:
   - Call LLM with structured messages. On error, fall back to current heuristic answer (no mems).

## 6) Write Path After Answer
- Append user + assistant messages to `upload_qa_messages` with token counts.
- Log query/answer to existing `query_logs` (unchanged).
- Long-term memory upsert:
  - Strategy A (cheap): embed `[Question]\n[Answer]` per turn when answer succeeds → store as `source=qa_turn`.
  - Strategy B (summaries): when session token size > threshold or every N turns, summarize prior history into a condensed note, embed, store as `source=summary`, then optionally prune oldest low-importance records.

## 7) Prompt Structure (example)
- `system`: “You are a helpful assistant. Use documents, memories, and recent chat. Cite documents as Doc <id>/Chunk <n>. If unsure, say you don’t know.”
- `context` message: serialized doc chunks and memories.
- `history` message: recent conversation (role-tagged).
- `user`: latest question.
- Guardrails: max tokens for context and history; truncate with ellipses; avoid leaking other users’ data via user/session scoping.

## 8) Ranking & Scoring
- Doc chunks: existing similarity score; optional MMR unchanged.
- Memories: `score = sim * (1 + 0.05*importance) * recencyBoost`; cap boosts to avoid overpowering docs.
- Deduplicate overlapping content: drop memories whose text overlaps a chosen doc chunk >80% by Jaccard.

## 9) Config & Ops
- New config fields under `uploadask.memory.*`: `enabled`, `topKMems` default 3, `maxHistoryTokens` default 800, `memoryVectorDim` (reuse cfg.VectorDim), `summaryEveryNTurns` (e.g., 6), `pruneLimit` (e.g., keep 200 memories/user).
- Metrics: memory search latency, hit counts, tokens used for history/memories, memory upsert failures.
- Feature flag: if disabled, skip memory search and memory writes but keep message log for future enablement.

## 10) Integration Notes
- Backend: add adapters for `MemoryStore` (pgvector-backed, in-memory fallback) and `MessageLog` (Postgres table + in-memory fallback) under `internal/infra/uploadask/memory`.
- Service layer: extend `Ask` to fetch history + memory search + compose prompt; ensure unit tests cover no-mem fallback and mixed retrieval.
- Frontend: optional toggles in Ask UI to include history and long-term memory; show which answers used memories vs docs.
- Privacy/tenancy: enforce `user_id` scoping on both tables; session_id optional filter in memory search to allow cross-session recall when desired.

## 11) Relationship to `upload_query_logs`
- Keep `upload_query_logs` as the immutable telemetry/audit log (latency, answer text, sources) tied to `upload_qa_sessions`.
- `upload_qa_messages` is the structured chat history for prompt assembly; it stores roles and token counts and can diverge (e.g., retries without logging).
