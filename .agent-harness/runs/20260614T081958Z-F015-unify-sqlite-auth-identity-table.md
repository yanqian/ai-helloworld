# Run Record: F015 - unify SQLite Auth identity table

## Summary

- Date: 20260614T081958Z
- Agent role: Manual fallback after orchestrator provider runtime failure
- Feature: F015
- Result: pass

## Repository State

- Starting commit: ac38b1a
- Ending commit: ac38b1a
- Working tree status: F015 planning, migration, repository, tests, docs, and run evidence are present in the working tree.

## Commands Run

```bash
make work
GOCACHE=/tmp/ai-helloworld-go-build GOMODCACHE=/tmp/ai-helloworld-go-mod go test -count=1 ./internal/infra/sqlite ./internal/infra/userrepo ./cmd/app ./internal/interface/http
./init.sh
git diff --check
```

## Evidence

- Orchestrator-first entrypoint selected F015, but the configured Codex provider failed before coding because its local runtime could not write the Codex state database or initialize the app-server client. The failure is recorded in `20260614T081516Z-F015-failure.md`.
- Fresh SQLite initialization now creates `user_identities`, does not create `auth_identities`, and keeps the foreign key to `users`.
- Legacy SQLite databases with `auth_identities` migrate identity rows into `user_identities` and drop `auth_identities`.
- Existing canonical `user_identities` rows are preserved when legacy rows duplicate provider identity or user/provider identity.
- SQLite Auth repository lookup, lookup-by-user, update, and insert SQL now target `user_identities`.
- README now documents local Auth persistence as `users` plus `user_identities`, and notes that older `auth_identities` databases migrate and drop the legacy table.
- Focused Go tests passed for SQLite migrations, SQLite Auth repository behavior, app wiring, and interface HTTP contracts.
- Root `./init.sh` passed after implementation.
- `git diff --check` passed.

## Failure Analysis

- Failure domain: none
- Failure summary: implementation and verification passed after manual fallback.
- Harness improvement: none for product work; provider runtime permission failure remains tracked separately as `agent_workflow_gap`.

## Files Changed

- `.agent-harness/SPEC.md`
- `.agent-harness/feature_list.json`
- `.agent-harness/progress.md`
- `.agent-harness/runs/20260614T081516Z-F015-failure.md`
- `.agent-harness/runs/20260614T081958Z-F015-unify-sqlite-auth-identity-table.md`
- `README.md`
- `internal/infra/sqlite/db.go`
- `internal/infra/sqlite/db_test.go`
- `internal/infra/userrepo/sqlite_repository.go`
- `internal/infra/userrepo/sqlite_repository_test.go`

## Evaluator Result

```text
EVAL_PASS: F015
```
