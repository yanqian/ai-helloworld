# Progress

## Current System Status

AI Agent Harness 0.3.3 is installed in hidden layout for the backend repository. The harness self-check passes after synchronizing final role verdict normalization from the template and bundled skill.

The backend minspec has been added to `.agent-harness/SPEC.md`. Feature state tracks recovery and contract work instead of treating historical product code as already evaluator-gated harness work.

F001 is complete: root `./init.sh` runs harness verification, verifies the Go toolchain, uses non-committed `/tmp` defaults for `GOCACHE` and `GOMODCACHE`, and runs `go test ./...`.

F005 is complete: local SQLite configuration defaults to `data/ai-helloworld.db`, SQLite database files under `data/` are ignored, local schema creation is idempotent, Auth uses SQLite in local mode before Postgres/memory fallbacks, and SQLite Auth persistence is covered by repository reopen tests.

F006 is complete: Smart FAQ questions, embeddings, semantic hashes, cached answers, and trending counts persist through SQLite in local mode, with repository/store reopen tests and provider wiring coverage. Earlier orchestrator/provider contradiction records are preserved as run evidence, but F006 has durable `EVAL_PASS: F006` records and is restored to `passes=true`, `status=done` after the harness verdict-normalization fix.

F009 is complete: the installed hidden harness files and bundled skill template now match the template fix for final role verdict normalization, including `CODING_PASS` / `CODING_FAIL` prompt output, final matching evaluator verdict parsing, provider contradiction docs, and unit regression tests.

## Last Completed Feature

F009 Sync harness final verdict normalization.

## Next Feature

F007 Upload Ask SQLite persistence.

## Known Issues

- New local-first SQLite requirement has been planned. F007 should move Upload & Ask persistence to SQLite.
- Real LLM and optional Google OAuth verification still require explicit credentials.
- Remote Postgres, pgvector, Valkey, R2, and GCP deployment are no longer desired for ordinary local operation, but existing adapters remain until local replacements are complete.
- `modernc.org/sqlite` raised the Go directive to 1.25; current local verification uses Go 1.26.
- Local `.agent-harness/agent-provider.json` is configured for Codex CLI and ignored by git. The harness now normalizes final structured role verdicts, but provider-specific task-complete event schemas remain intentionally unparsed until verified with fixtures.
- The sibling frontend repository is `/Users/armstrong/Project/ai-helloworld-fe` and needs matching contract awareness.

## Recovery Notes

- F006 had repeated orchestrator failure records from provider exit-code contradictions despite durable implementation and evaluator pass evidence.
- The template fix was verified in `/Users/armstrong/Project/ai-agent-harness-template` as F033 before synchronizing installed files here.
- Root `./init.sh` passed after the installed harness synchronization.
