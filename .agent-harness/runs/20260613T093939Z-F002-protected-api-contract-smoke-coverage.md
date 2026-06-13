# Run Record: F002 - Protected API contract smoke coverage

## Summary

- Date: 20260613T093939Z
- Agent role: Manual Coding Agent fallback and Evaluator Agent fallback
- Feature: F002 Protected API contract smoke coverage
- Result: pass

## Repository State

- Starting commit: eb4a035
- Ending commit: working tree
- Working tree status: F002 router contract smoke tests and harness state updates are present in the working tree.

## Commands Run

```bash
./init.sh
GOCACHE=/tmp/ai-helloworld-go-build GOMODCACHE=/tmp/ai-helloworld-go-mod go test -count=1 ./internal/interface/http
./init.sh
```

## Evidence

- Tests: root `./init.sh` passed before implementation, confirming the repository recovery baseline.
- Tests: focused router tests passed for `./internal/interface/http` without live LLM, OAuth, R2, Postgres, or Valkey credentials.
- Tests: protected contract smoke coverage now exercises Auth, summarizer, UV advisor, Smart FAQ, and Upload & Ask route registration.
- Tests: protected routes reject missing bearer tokens with structured `unauthorized` error responses.
- Tests: representative protected routes reject invalid bearer tokens with structured `invalid_token` error responses.
- Tests: public Auth error contracts for register, login, and refresh keep the documented structured error shape.
- External behavior verification: no live external services were required.
- Capability gaps: real LLM and optional Google OAuth credentials are still needed for live AI/OAuth behavior outside deterministic local recovery.

## Failure Analysis

- Failure domain: none
- Failure summary: no implementation or evaluator failure in this run.
- Harness improvement: none required.
- Follow-up feature: F003 Upload Ask local capability contract.

## Files Changed

- `internal/interface/http/router_test.go`
- `.agent-harness/feature_list.json`
- `.agent-harness/progress.md`
- `.agent-harness/runs/20260613T093939Z-F002-protected-api-contract-smoke-coverage.md`

## Evaluator Result

```text
EVAL_PASS: F002
```

## Follow-Up

- Continue with F003 to make Upload & Ask local capability behavior and response-shape coverage explicit.
