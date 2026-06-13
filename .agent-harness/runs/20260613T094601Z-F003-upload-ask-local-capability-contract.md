# Run Record: F003 - Upload Ask local capability contract

## Summary

- Date: 20260613T094601Z
- Agent role: Manual Coding Agent fallback and Evaluator Agent fallback
- Feature: F003 Upload Ask local capability contract
- Result: pass

## Repository State

- Starting commit: 4724e15
- Ending commit: working tree
- Working tree status: F003 Upload & Ask HTTP contract smoke, local capability docs, and harness state updates are present in the working tree.

## Commands Run

```bash
./init.sh
GOCACHE=/tmp/ai-helloworld-go-build GOMODCACHE=/tmp/ai-helloworld-go-mod go test -count=1 ./internal/interface/http ./internal/domain/uploadask ./tests/unit
./init.sh
```

## Evidence

- Tests: root `./init.sh` passed before implementation, confirming the repository recovery baseline.
- Tests: focused Upload & Ask and HTTP tests passed without live LLM, OAuth, R2, Postgres, pgvector, Valkey, Redis, or GCP credentials.
- Tests: `TestRouter_UploadAskLocalContractSmoke` uploads a document through the HTTP route, verifies the `document` response shape, manually processes the document, verifies the processed status shape, performs a QA query, and verifies `sessionId`, `sources[]`, sessions, and query log JSON shapes.
- Docs: `docs/upload-ask/local-capability-contract.md` documents local SQLite defaults, in-memory blob storage, immediate queues, deterministic embeddings, Echo LLM, and when memory fallbacks are used.
- Docs: the same contract records Postgres + pgvector, Valkey/Redis, R2/S3, embedding credentials, and LLM credentials as explicit legacy/integration capability gaps when missing.
- Docs: README links to the local capability contract, and the Upload & Ask architecture diagram now labels the default local SQLite/memory paths instead of implying Postgres/R2/Valkey are required.
- External behavior verification: no live external services were required.
- Capability gaps: live LLM answer quality, provider token usage, embedding quality, R2 persistence, Valkey queue behavior, and Postgres + pgvector integration remain unverified unless those optional services are explicitly configured.

## Failure Analysis

- Failure domain: none
- Failure summary: no implementation or evaluator failure in this run.
- Harness improvement: none required.
- Follow-up feature: F004 Cross repository API contract alignment.

## Files Changed

- `internal/interface/http/router_test.go`
- `docs/upload-ask/local-capability-contract.md`
- `docs/upload-ask/architecture.md`
- `README.md`
- `.agent-harness/feature_list.json`
- `.agent-harness/progress.md`
- `.agent-harness/runs/20260613T094601Z-F003-upload-ask-local-capability-contract.md`

## Evaluator Result

```text
EVAL_PASS: F003
```

## Follow-Up

- Continue with F004 to align shared API contract notes with the sibling frontend repository.
