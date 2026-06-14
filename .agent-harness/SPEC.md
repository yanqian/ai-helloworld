# AI Helloworld Backend SPEC

## Project Minspec

### Goal

Maintain a recoverable Go HTTP API for the AI Helloworld product. The backend provides authenticated AI workflows for summarization, UV advice, Smart FAQ, and Upload & Ask document question answering, and it is paired with the React frontend in `/Users/armstrong/Project/ai-helloworld-fe`.

### Scope Included

- HTTP API under `/api/v1` using Gin.
- Email/password authentication, JWT access tokens, refresh tokens, logout, profile, and Google OAuth.
- Summarization endpoints, including sync JSON responses and Server-Sent Events streaming.
- Singapore UV advice powered by the data.gov.sg UV source and an OpenAI-compatible chat model.
- Smart FAQ search using exact, semantic hash, similarity, and hybrid strategies with cache-backed trending queries.
- Upload & Ask document ingestion, object storage, chunking, embeddings, local SQLite retrieval, QA sessions, query logs, and optional conversational memory.
- Configuration through `configs/config.yaml` and environment variables.
- Local and CI verification through Go tests, focused smoke checks, and the AI Agent Harness recovery protocol.

### Scope Excluded

- Frontend rendering and browser UX, which live in the sibling frontend repository.
- Production Cloud Run, R2, Postgres, Valkey, and OAuth credential provisioning beyond backend configuration contracts.
- Automatic commits, pushes, or provider-specific unattended agent execution without explicit configuration.

### Core Flows

- A user registers or logs in, receives access and refresh tokens, and calls protected API routes.
- The frontend sends summary text and optionally receives a sync response or SSE chunks.
- The frontend asks for UV advice for a date; the backend fetches UV data and returns normalized AI guidance.
- The frontend searches Smart FAQ; the backend resolves cache/repository hits or asks the LLM and records trending queries.
- The frontend uploads a document; the backend stores metadata and file content, processes chunks and embeddings, then answers questions with citations.
- A future agent resumes work by reading `AGENTS.md`, `.agent-harness/progress.md`, `.agent-harness/feature_list.json`, recent git history, and running `./init.sh`.

### Constraints

- Protected business endpoints require bearer-token authentication.
- `LLM_API_KEY` is required for real chat and embedding calls; tests should avoid depending on live credentials unless explicitly scoped.
- Upload & Ask persistence uses SQLite for ordinary local development. Postgres with pgvector remains an optional legacy/integration path rather than a default requirement.
- Redis or Valkey queues are optional; the service must still be understandable and testable without them.
- Verification must not rely on chat history or hidden agent memory.

### Ambiguities Or Assumptions

- The sibling frontend path is assumed to be `/Users/armstrong/Project/ai-helloworld-fe`.
- Local smoke verification should prefer deterministic tests and mocks over live LLM, OAuth, R2, Postgres, or Valkey services until those capabilities are explicitly configured.
- Cross-repository API contract drift should be tracked durably in both repositories.

### Required Capabilities

- Go 1.24-compatible toolchain.
- Local build and test cache outside committed source.
- Optional OpenAI-compatible API credentials for live AI flows.
- Optional Postgres + pgvector, Valkey/Redis, Cloudflare R2, GCP, and Google OAuth credentials for legacy or production-like integration work.
- Agent provider configuration before `make work` can run real unattended Coding Agent or Evaluator Agent adapters.

### Implementation Paths

- Runtime entrypoint: `cmd/app`.
- HTTP transport: `internal/interface/http`.
- Domain logic: `internal/domain`.
- Infrastructure adapters: `internal/infra`.
- Configuration: `configs/config.yaml` and `internal/infra/config`.
- Tests: `internal/**/**/*_test.go` and `tests/unit`.
- Harness state: `.agent-harness/SPEC.md`, `.agent-harness/feature_list.json`, `.agent-harness/progress.md`, `.agent-harness/runs/`, and root `AGENTS.md` / `init.sh`.

### Verification Surface

- `./init.sh` for harness recovery until F001 upgrades it to backend recovery.
- `GOCACHE="${TMPDIR:-/tmp}/ai-helloworld-go-build" go test ./...` for backend unit and integration-style tests.
- Focused HTTP router tests for auth requirements and endpoint contracts.
- Future smoke checks that start the service with deterministic or mocked dependencies and hit at least one real endpoint.

## Feature Decomposition

Feature count is determined by independently verifiable behavior and capability boundaries, not by how much text the user wrote. Planning must split broad requirements into multiple features when there are separate user-visible behaviors, required capabilities, implementation boundaries, risk domains, or verification surfaces.

- F001 covers backend recovery because it changes the root startup contract and verification path.
- F002 covers auth/API contract protection because it is a distinct security boundary.
- F003 covers Upload & Ask local capability because it spans persistence, retrieval, and background processing risks.
- F004 covers cross-repository API drift because it is shared with the frontend and should be verified as a contract rather than hidden in either side.
- F005 covers SQLite foundation plus Auth persistence because local login state is the first durable storage capability needed for a useful local backend.
- F006 covers Smart FAQ SQLite persistence because it has separate question, answer-cache, semantic search, and trending behavior.
- F007 covers Upload & Ask SQLite persistence because document metadata, chunks, sessions, query logs, messages, and memories form a larger RAG-specific persistence boundary.
- F008 covers local-only runtime cleanup because removing GCP deployment assumptions affects Makefile, README, config defaults, and recovery docs rather than repository adapters.

## Local SQLite Backend Requirement

### Goal

Make the backend a local-first application that can run on a developer machine with SQLite-backed persistence instead of depending on GCP, Cloud Run, remote Postgres, pgvector, Valkey, or R2 for ordinary local use.

### Scope Included

- Add SQLite configuration and a shared local database file path, defaulting to an ignored `data/` location.
- Use SQLite for local Auth persistence first, then extend SQLite persistence to Smart FAQ and Upload & Ask in separate features.
- Keep existing Postgres, Valkey, and R2 adapters available as optional legacy/integration paths unless explicitly removed later.
- Update project-owned docs and recovery scripts so local run instructions no longer center GCP deployment.
- Keep local verification deterministic and free of live remote database credentials.

### Scope Excluded

- Migrating existing remote production data into SQLite.
- Removing LLM/API-key requirements for real AI calls.
- Building a frontend-visible data migration UI.
- Implementing high-performance vector indexing; local SQLite retrieval may use deterministic in-process similarity over stored embeddings.
- Pushing or deploying to GCP.

### Core Flows

- A local developer starts the backend with a local `.env` or defaults, and the app opens or creates a SQLite database file.
- A user registers, logs in, refreshes a token, and remains persisted after backend restart.
- Smart FAQ questions, generated answers, and trending counts persist locally through SQLite once F006 is complete.
- Upload & Ask documents, chunks, sessions, logs, messages, and memories persist locally through SQLite once F007 is complete.
- `./init.sh` verifies local mode without requiring GCP, remote Postgres, Valkey, R2, or pgvector.

### Constraints

- SQLite database files under `data/` must not be committed.
- Local tests must use temporary SQLite files or in-memory databases.
- SQLite schema creation must be idempotent.
- Existing interfaces should remain domain-owned; SQLite should be an infrastructure adapter.
- F005 must not bundle all FAQ and Upload & Ask persistence work.

### Ambiguities Or Assumptions

- "No GCP deployment" means the local developer workflow should stop relying on GCP; existing deploy scripts can be removed or demoted in F008 rather than immediately deleting every legacy cloud reference.
- The default local SQLite file may be `data/ai-helloworld.db`.
- Postgres adapters can remain available for compatibility until the user asks to delete them.

### Required Capabilities

- A Go SQLite driver available in normal `go test ./...` verification.
- Idempotent SQLite migrations for every table added.
- Tests that prove persistence survives repository re-instantiation against the same SQLite file.
- Documentation that names remote-service gaps instead of hiding them.

### Implementation Paths

- Config: `internal/infra/config/config.go`, `configs/config.yaml`, `.gitignore`.
- SQLite infrastructure: `internal/infra/sqlite`.
- Auth adapter: `internal/infra/userrepo`.
- Future FAQ adapters: `internal/infra/faqrepo` and `internal/infra/faqstore`.
- Future Upload & Ask adapters: `internal/infra/uploadask/repo` and `internal/infra/uploadask/memory`.
- Providers: `cmd/app/providers.go`.
- Tests: focused `*_test.go` files plus root `./init.sh`.

### Verification Surface

- `go test ./...` through root `./init.sh`.
- SQLite auth persistence tests using a temporary database file.
- Later feature tests for FAQ and Upload & Ask persistence.
- `git status --short` should keep SQLite database files untracked/ignored.

## Cross Repository API Contract

- Sibling frontend repository: `/Users/armstrong/Project/ai-helloworld-fe`.
- Durable backend contract notes live in `docs/api-contract.md`.
- Durable frontend matching notes should live in `/Users/armstrong/Project/ai-helloworld-fe/docs/api-contract.md`.
- Shared API surfaces include Auth, summarizer sync and stream, UV advisor, Smart FAQ, and Upload & Ask document/session/query routes.
- Contract-sensitive JSON fields include `refreshToken`, `durationMs`, `tokenUsage`, `sessionId`, `partial_summary`, `documentId`, `chunkIndex`, `score`, `preview`, and `failureReason`.
- Backend drift verification includes `TestFrontendContractJSONFields`, router protected contract smoke coverage, and Upload & Ask local contract smoke coverage.

## Local Backend Startup Smoke

### Goal

Make local backend startup for frontend/backend联调 deterministic and traceable after the SQLite local-first migration.

### Included Scope

- Local startup with SQLite and a local JWT secret.
- Startup without `LLM_API_KEY`, using deterministic local AI fallbacks or explicit local-only behavior.
- A smoke verification path that starts the backend, registers/logs in a local user, calls a protected route, and exercises at least one AI route without live external services.
- Documentation for the exact local command and required environment.

### Excluded Scope

- Removing the requirement for `JWT_SECRET`; authenticated local flows still need a signing secret.
- Browser E2E across the frontend and backend.
- Live LLM quality verification, Google OAuth, R2, Valkey, Postgres, pgvector, or GCP deployment.

### Core Flows

- Developer runs a documented local command with `JWT_SECRET` and no `LLM_API_KEY`.
- Backend opens SQLite, wires local storage, starts listening on `:8080`, and serves auth plus protected APIs.
- Smoke script registers a user, logs in, calls `/api/v1/auth/me`, and calls `/api/v1/summaries`.

### Constraints

- Local smoke must use temporary Go caches and must not require committed database files.
- The default project recovery check must remain deterministic and not leave a long-running server.
- Live LLM credentials, when provided, should continue to use the real ChatGPT-compatible client.

### Ambiguities Or Assumptions

- `JWT_SECRET` remains the one required local secret because token signing cannot be meaningfully disabled for protected-route联调.
- Without `LLM_API_KEY`, local AI responses may be deterministic placeholders; real answer quality remains a capability gap.

### Required Capabilities

- A no-network local AI client or provider fallback compatible with summarizer, Smart FAQ, UV advisor, and Upload & Ask embedding/chat call sites.
- A scriptable startup smoke that can allocate and clean up a backend process.

### Implementation Paths

- ChatGPT-compatible client or provider wiring under `internal/infra/llm/chatgpt` and `cmd/app/providers.go`.
- Local scripts under `scripts/`.
- README local联调 instructions.
- Focused tests in `internal/infra/llm/chatgpt`, `cmd/app`, or startup smoke fixtures.

### Verification Surface

- Focused Go tests for no-key local AI behavior.
- `scripts/local_smoke.sh` or equivalent starts the built backend, waits for readiness via real API calls, and exits non-zero on failure.
- Root `./init.sh` continues to pass.

## SQLite Auth Timestamp Compatibility

### Goal

Ensure local SQLite-backed registration, login, and profile fetches keep working when auth rows contain timestamp strings written in either the current RFC3339 format or legacy/database-style space-separated UTC offset formats.

### Included Scope

- SQLite Auth user reads from `users.created_at`.
- SQLite Auth identity reads from `auth_identities.created_at` and `auth_identities.updated_at`.
- Compatibility with the observed failing value shape `2025-11-21 14:10:45.570822+00`.
- Regression tests that reproduce the parsing failure and prove user and identity reads succeed.

### Excluded Scope

- Migrating or rewriting existing SQLite database files.
- Changing the public Auth API response contract.
- Changing Postgres timestamp handling.
- Broad timestamp normalization for every non-Auth SQLite table unless a failing Auth flow requires it.

### Core Flows

- A user registers or logs in against a local SQLite database that already contains database-style timestamp text.
- Auth repository scans the row, parses the timestamp, and returns a valid domain user or identity instead of failing the request.
- Current RFC3339 timestamp writes and reads continue to work.

### Constraints

- New writes should continue using the existing RFC3339Nano format for deterministic local storage.
- Compatibility parsing must be explicit and covered by focused tests.
- Invalid timestamp strings should still return an error instead of silently zeroing time.

### Ambiguities Or Assumptions

- The failing timestamp likely came from an older local database, manual insert, or database-style default rather than the current `Create` method, because current writes use RFC3339Nano.
- The immediate user-visible bug is in Auth; other SQLite adapters can receive follow-up compatibility work if they surface the same legacy timestamp shape.

### Required Capabilities

- Focused Go tests for SQLite Auth repository timestamp parsing.
- Temporary SQLite databases or in-memory files for deterministic verification.

### Implementation Paths

- Auth SQLite adapter: `internal/infra/userrepo/sqlite_repository.go`.
- Auth SQLite tests: `internal/infra/userrepo/sqlite_repository_test.go`.
- Harness state and run evidence under `.agent-harness/`.

### Verification Surface

- A focused regression test fails before the parser change and passes after it.
- `go test -count=1 ./internal/infra/userrepo`.
- Root `./init.sh` continues to pass.

## Upload Ask SQLite Timestamp Compatibility

### Goal

Ensure Upload & Ask frontend flows keep working when local SQLite Upload & Ask rows contain timestamp strings written in either the current RFC3339 format or legacy/database-style space-separated UTC offset formats.

### Included Scope

- SQLite Upload & Ask document, file, chunk, QA session, and query log reads.
- SQLite Upload & Ask conversation message and long-term memory reads.
- Compatibility with the observed failing value shape `2025-12-05 15:06:46.339153+00`.
- Regression tests that reproduce database-style timestamps across the Upload & Ask repository and memory adapters.

### Excluded Scope

- Migrating or rewriting existing SQLite database files.
- Changing frontend code or public Upload & Ask response shapes.
- Changing Postgres timestamp handling.
- General timestamp compatibility for non-Upload & Ask domains beyond the already-completed Auth fix.

### Core Flows

- The frontend lists uploaded documents or loads document details from a local SQLite database containing database-style timestamp text.
- The frontend asks a question and the backend reads chunks, sessions, query logs, messages, or memories that contain database-style timestamp text.
- Backend Upload & Ask SQLite adapters parse compatible timestamps and return domain objects instead of surfacing parse errors to the frontend.

### Constraints

- New writes should continue using the existing RFC3339Nano format for deterministic local storage.
- Invalid timestamp strings should still return explicit errors rather than silently zeroing time.
- Verification should remain local and deterministic without live LLM, R2, Valkey, Postgres, pgvector, or GCP.

### Ambiguities Or Assumptions

- The failing frontend error is produced by the backend when an Upload & Ask SQLite scan reads existing database-style timestamp text.
- Existing local databases may contain mixed timestamp formats; compatibility parsing is safer than requiring users to delete local data.

### Required Capabilities

- Focused Go tests for `internal/infra/uploadask/repo` and `internal/infra/uploadask/memory`.
- Temporary SQLite databases for deterministic regression fixtures.

### Implementation Paths

- Upload & Ask SQLite repositories: `internal/infra/uploadask/repo/sqlite.go`.
- Upload & Ask SQLite memory: `internal/infra/uploadask/memory/sqlite.go`.
- Focused tests: `internal/infra/uploadask/repo/sqlite_test.go` and `internal/infra/uploadask/memory/sqlite_test.go`.
- Harness state and run evidence under `.agent-harness/`.

### Verification Surface

- Focused regression tests fail before the parser change and pass after it.
- `go test -count=1 ./internal/infra/uploadask/repo ./internal/infra/uploadask/memory`.
- Root `./init.sh` continues to pass.

## SQLite FAQ Questions Table Unification

### Goal

Make the local SQLite Smart FAQ schema use the same canonical `questions` table name as the FAQ/Postgres repository so developers do not need to know two physical table names for the same domain concept.

### Included Scope

- Change the SQLite Smart FAQ question repository from `faq_questions` to `questions`.
- Change local SQLite migrations so `questions` has `id`, `question_text`, `embedding`, `semantic_hash`, and `created_at`.
- Migrate existing local SQLite data from `faq_questions` into `questions` when needed.
- Add `created_at` to an existing legacy `questions` table when it is missing.
- Repoint `faq_answer_cache.question_id` foreign key from `faq_questions(id)` to `questions(id)`.
- Drop `faq_questions` after successful migration so the local DB no longer keeps two Smart FAQ question tables.
- Update tests and docs to describe `questions` as the local SQLite Smart FAQ table.
- Audit other duplicate local table names and record any findings without changing unrelated domains.

### Excluded Scope

- Preserving `faq_questions` as a compatibility table or view.
- Changing Postgres FAQ repository behavior; it already uses `questions`.
- Migrating Auth `user_identities`/`auth_identities` naming differences in this feature.
- Changing Upload & Ask table names.
- Editing committed or ignored SQLite database files directly as source artifacts.

### Core Flows

- A fresh local SQLite database initializes with `questions`, `faq_answer_cache`, and `faq_trending_queries`.
- An existing local database with only legacy `questions` gains `created_at` and continues using existing rows.
- An existing local database with `faq_questions` migrates its rows into `questions`, preserves answer cache referential integrity, and drops `faq_questions`.
- Smart FAQ exact, semantic hash, nearest-neighbor, insert, answer cache, and trending flows continue to work through SQLite.

### Constraints

- Migrations must be idempotent across fresh and existing SQLite files.
- Existing `questions` data should not be overwritten by `faq_questions` rows with duplicate `question_text`.
- Local verification must remain deterministic and avoid live LLM, Postgres, Valkey, pgvector, R2, or GCP.
- Invalid or unusual user data should fail explicitly rather than silently corrupting rows.

### Ambiguities Or Assumptions

- `questions` is the canonical Smart FAQ question table name because both the Postgres adapter and historical FAQ schema use it.
- The empty `faq_questions` table in the current local database can be removed after the migration path is implemented.
- The local DB also contains an empty `user_identities` table while SQLite Auth uses `auth_identities`; this is a separate Auth naming issue and not part of F014.

### Required Capabilities

- SQLite schema migration tests that can construct legacy `questions`, `faq_questions`, and cache states.
- Focused Go tests for FAQ repository/store behavior after migration.
- Durable docs and harness evidence for the schema audit.

### Implementation Paths

- SQLite migration: `internal/infra/sqlite/db.go`.
- SQLite FAQ repository: `internal/infra/faqrepo/sqlite_repository.go` and tests.
- SQLite FAQ store: `internal/infra/faqstore/sqlite_store.go` and tests.
- Documentation: `README.md`, `docs/faq/faq-spec.md`, and any local capability docs that name Smart FAQ SQLite tables.
- Harness state and run evidence under `.agent-harness/`.

### Verification Surface

- Focused SQLite migration and FAQ tests.
- `go test -count=1 ./internal/infra/sqlite ./internal/infra/faqrepo ./internal/infra/faqstore ./cmd/app`.
- Root `./init.sh` continues to pass.

## Harness Governance

### Skill Assisted Workflow

The AI Agent Harness skill is a convenience layer for initializing, repairing, planning, implementing, evaluating, and committing approved work while the repository remains the durable source of truth. This project must preserve the template's vendor-neutral boundary: `skills/ai-agent-harness/` is embedded for recovery, but durable state remains in `AGENTS.md`, `.agent-harness/SPEC.md`, `.agent-harness/feature_list.json`, `.agent-harness/progress.md`, `.agent-harness/docs/`, `.agent-harness/runs/`, and git history.

Initialization supports `new`, `adopt`, `repair`, and `check` modes, installation layouts, the default `hidden` layout, root `AGENTS.md` and `init.sh` as thin entry points, `visible` layout for template-maintenance work, version drift handling, and semantically valid state checks. installed skill usage is preferred; Manual `python3 skills/.../init_harness.py` commands are a repository-checkout or vendor-neutral fallback usage path.

### New Project Flow

The New Project Flow provides a visual map from skill invocation through minspec input, SPEC normalization, feature decomposition, runnable skeleton, provider setup, `make work`, evaluator pass, root `./init.sh`, and approved commit. Human inputs include the project location, clarification for ambiguous requirements, provider choice, permission for real agent work, and approved commit boundaries.

### Project Recovery Init

root `./init.sh` is the project recovery entry point. In hidden layout, `.agent-harness/scripts/init.sh` verifies harness health, while root `./init.sh` starts as a thin wrapper before a minspec exists. After a minspec is accepted, a runnable-skeleton feature must make root `./init.sh` install or verify dependencies, run project-owned checks or smoke tests, emit clear logs, and exit non-zero on failure. Harness verification alone must not be treated as project completion.

### Spec Normalization

Spec Normalization is required before planning feature entries. New requirements must capture goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface. Vague requirements must not become executable features without clarification, recorded assumptions, or a durable capability/blocker/follow-up.

### Evaluator Evidence

Completed features must have durable evaluator evidence. From the evaluator-evidence baseline onward, `status=done` and `passes=true` are valid only when a run record contains `EVAL_PASS: Fxxx` for that feature.

### Feature-Linked Commits

Approved feature commits must start with `Fxxx <Action> <concise summary>` and reference feature IDs that exist in `feature_list.json`. Use `No-feature:` only for explicitly non-feature work.

## Harness Provider Runtime Preflight

### Goal

Keep orchestrator-first work usable from agent-driven workflows by detecting provider runtime permission gaps before feature state is mutated.

### Included Scope

- Sync provider runtime preflight support from the AI Agent Harness template.
- Support `runtime_check_command`, `coding_runtime_check_command`, and `evaluator_runtime_check_command` in installed provider configuration.
- Emit `PROVIDER_RUNTIME_PERMISSION_REQUIRED` when provider runtime checks hit permission-like failures such as Codex state-file or app-server access denial.
- Preserve equivalent entry points for Codex, Claude Code, Cursor Agent, and custom providers without guessing unverified provider command shapes.

### Excluded Scope

- Automatically escalating permissions.
- Parsing private provider task-complete schemas.
- Changing backend product behavior.

### Verification Surface

- Root `./init.sh` after synchronizing the installed harness files.
- Installed harness unit tests for runtime preflight pass, permission-required failure, and role-specific runtime check selection.
