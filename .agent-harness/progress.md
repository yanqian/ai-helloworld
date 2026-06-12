# Progress

## Current System Status

AI Agent Harness 0.3.3 is installed in hidden layout for the backend repository. The harness self-check passes after adding the embedded skill template assets required by contract tests.

The backend minspec has been added to `.agent-harness/SPEC.md`. Feature state tracks recovery and contract work instead of treating historical product code as already evaluator-gated harness work.

F001 is complete: root `./init.sh` now runs harness verification, verifies the Go toolchain, uses non-committed `/tmp` defaults for `GOCACHE` and `GOMODCACHE`, and runs `go test ./...`.

## Last Completed Feature

None.

## Next Feature

F002 Protected API contract smoke coverage.

## Known Issues

- Real LLM, Google OAuth, R2, Postgres, pgvector, and Valkey verification require explicit credentials or local services.
- `agent-provider.json` is not configured, so orchestrated `make work` should fail closed until a provider is selected.
- The sibling frontend repository is `/Users/armstrong/Project/ai-helloworld-fe` and needs matching contract awareness.
