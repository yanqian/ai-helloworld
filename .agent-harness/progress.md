# Progress

## Current System Status

AI Agent Harness 0.3.3 is installed in hidden layout for the backend repository. The harness self-check passes after synchronizing final role verdict normalization from the template and bundled skill.

The backend minspec has been added to `.agent-harness/SPEC.md`. Feature state tracks recovery and contract work instead of treating historical product code as already evaluator-gated harness work.

F001 is complete: root `./init.sh` runs harness verification, verifies the Go toolchain, uses non-committed `/tmp` defaults for `GOCACHE` and `GOMODCACHE`, and runs `go test ./...`.

F005 is complete: local SQLite configuration defaults to `data/ai-helloworld.db`, SQLite database files under `data/` are ignored, local schema creation is idempotent, Auth uses SQLite in local mode before Postgres/memory fallbacks, and SQLite Auth persistence is covered by repository reopen tests.

F006 is complete: Smart FAQ questions, embeddings, semantic hashes, cached answers, and trending counts persist through SQLite in local mode, with repository/store reopen tests and provider wiring coverage. Earlier orchestrator/provider contradiction records are preserved as run evidence, but F006 has durable `EVAL_PASS: F006` records and is restored to `passes=true`, `status=done` after the harness verdict-normalization fix.

F007 is complete: Upload & Ask documents, file metadata, chunks, QA sessions, query logs, conversation messages, and long-term memories now persist through SQLite in local mode. Local chunk and memory similarity search runs deterministically in Go over JSON-stored embeddings, and provider wiring selects SQLite before Postgres or memory fallbacks.

F008 is complete: local setup docs now center SQLite and `./init.sh`/`make run`; Makefile deploy and GCP init are demoted behind explicit legacy targets; GitHub Actions no longer deploys to Cloud Run on main pushes; GCP/R2/Postgres/pgvector/Valkey docs are marked legacy or optional integration references.

F002 is complete: router-level contract smoke coverage now verifies protected route registration across Auth, summarizer, UV advisor, Smart FAQ, and Upload & Ask; missing bearer tokens return structured `unauthorized` errors; representative invalid bearer tokens return structured `invalid_token` errors; and public auth error contracts are covered without live external services.

F003 is complete: Upload & Ask now has a durable local capability contract documenting SQLite defaults, memory blob storage, immediate queues, deterministic embeddings, Echo LLM, legacy Postgres/pgvector/R2/Valkey capability gaps, and frontend-sensitive response shapes. A deterministic HTTP smoke covers upload response metadata, pending-to-processed status, document listing, QA session creation, citation sources, and query log shapes without live external services.

F004 is complete: the backend records the sibling frontend path `/Users/armstrong/Project/ai-helloworld-fe`, shared API surfaces, and contract-sensitive fields in durable API contract docs and SPEC notes. A reflection-based JSON tag drift guard now protects frontend-sensitive fields, and the sibling frontend repository has matching `docs/api-contract.md` notes.

F009 is complete: the installed hidden harness files and bundled skill template now match the template fix for final role verdict normalization, including `CODING_PASS` / `CODING_FAIL` prompt output, final matching evaluator verdict parsing, provider contradiction docs, and unit regression tests.

F010 is complete: local backend startup now only requires `JWT_SECRET` for auth signing and no longer requires `LLM_API_KEY`; the ChatGPT-compatible client enters deterministic offline mode for chat, stream, and embedding calls when no live key is configured. `make local-smoke` starts the backend on a temporary port and SQLite database, registers and logs in a user, calls `/api/v1/auth/me`, and exercises `/api/v1/summaries`.

F011 is complete: SQLite Auth user and identity scans now parse both current RFC3339Nano timestamps and legacy/database-style timestamp text such as `2025-11-21 14:10:45.570822+00`, while invalid timestamp strings still return explicit parse errors. Focused regression coverage protects user reads, identity reads, current persisted Auth behavior, and invalid timestamp failures.

F012 is complete: Upload & Ask SQLite repository and memory scans now parse both current RFC3339Nano timestamps and legacy/database-style timestamp text such as `2025-12-05 15:06:46.339153+00`, while invalid timestamp strings still return explicit parse errors. Focused regression coverage protects document, file, chunk, QA session, query log, message, and memory reads.

F013 is complete: the installed hidden harness now supports provider runtime preflight checks, permission-required escalation markers, and vendor-neutral runtime check entries for Codex, Claude Code, Cursor Agent, and custom providers.

F014 is complete: local SQLite Smart FAQ now uses `questions` as the canonical question table, adds/backfills `created_at` on legacy `questions`, migrates rows from `faq_questions`, rebuilds `faq_answer_cache` with a foreign key to `questions(id)`, and drops `faq_questions`. The schema audit also recorded the separate empty Auth naming split `user_identities` versus `auth_identities` without changing Auth in this feature.

F015 is complete: local SQLite Auth now uses `user_identities` as the canonical identity table, migrates existing `auth_identities` rows, drops the legacy table, and aligns SQLite Auth tests and docs with the Postgres/login schema name.

F016 is complete: the local Upload & Ask ImmediateQueue now detaches handler execution from upload request cancellation, focused queue plus HTTP/router tests cover the regression, and durable `EVAL_PASS: F016` evaluator evidence is recorded.


## Last Completed Feature

F016 Detach Upload Ask background context.

## Next Feature

None.

## Known Issues

- Local-first SQLite requirement and backend recovery feature list are complete.
- Real LLM and optional Google OAuth verification still require explicit credentials.
- Remote Postgres, pgvector, Valkey, R2, and GCP deployment are no longer desired for ordinary local operation, but existing adapters remain until local replacements are complete.
- `modernc.org/sqlite` raised the Go directive to 1.25; current local verification uses Go 1.26.
- Local `.agent-harness/agent-provider.json` is configured for Codex CLI and ignored by git. The harness now normalizes final structured role verdicts, but provider-specific task-complete event schemas remain intentionally unparsed until verified with fixtures.
- The sibling frontend repository is `/Users/armstrong/Project/ai-helloworld-fe` and needs matching contract awareness.
- Local backend联调 startup requires `JWT_SECRET`; `LLM_API_KEY` is optional and only needed for real LLM quality. `make local-smoke` verifies the no-live-LLM startup path.
- Local Auth rows may contain legacy/database-style timestamp text such as `2025-11-21 14:10:45.570822+00`; F011 makes SQLite Auth reads compatible while preserving explicit errors for invalid timestamps.
- Upload & Ask rows may contain legacy/database-style timestamp text such as `2025-12-05 15:06:46.339153+00`; F012 makes Upload & Ask SQLite reads compatible while preserving explicit errors for invalid timestamps.
- `make work` for F011 invoked the orchestrator successfully but the Codex provider failed before coding because it could not write `/Users/armstrong/.codex/state_5.sqlite` and could not initialize the in-process app-server client. Manual fallback completed F011 with durable evaluator evidence.
- `make work` for F012 hit the same Codex provider runtime permission failure before business coding; manual fallback completed F012 with durable evaluator evidence.
- Local SQLite Smart FAQ table naming is unified on `questions` as of F014; older `faq_questions` tables are migrated and dropped during SQLite startup.
- Local SQLite Auth table naming is unified on `user_identities` as of F015; older `auth_identities` tables are migrated and dropped during SQLite startup.


- F013 syncs the template provider runtime preflight fix so future provider permission gaps can stop before feature attempts are mutated and ask the outer agent or user for explicit escalation.

## Recovery Notes

- F006 had repeated orchestrator failure records from provider exit-code contradictions despite durable implementation and evaluator pass evidence.
- The template fix was verified in `/Users/armstrong/Project/ai-agent-harness-template` as F033 before synchronizing installed files here.
- Root `./init.sh` passed after the installed harness synchronization.
- The template provider runtime preflight fix was verified in `/Users/armstrong/Project/ai-agent-harness-template` as F034 before synchronizing installed files here as F013.
