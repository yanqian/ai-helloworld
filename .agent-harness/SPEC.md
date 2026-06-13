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
- Upload & Ask document ingestion, object storage, chunking, embeddings, pgvector retrieval, QA sessions, query logs, and optional conversational memory.
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
- Upload & Ask persistence requires Postgres with pgvector for production-like retrieval; memory fallbacks are for local development only.
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
- Optional Postgres + pgvector, Valkey/Redis, Cloudflare R2, and Google OAuth credentials for production-like integration work.
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
