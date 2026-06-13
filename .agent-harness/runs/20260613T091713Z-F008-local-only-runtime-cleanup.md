# Run Record: F008 - Local-only runtime cleanup

## Summary

- Date: 20260613T091713Z
- Agent role: Manual Coding Agent fallback and Evaluator Agent fallback
- Feature: F008 Local-only runtime cleanup
- Result: pass

## Repository State

- Starting commit: 73de2ae
- Ending commit: working tree
- Working tree status: F008 documentation, Makefile, CI, config, legacy GCP script, and harness state updates are present in the working tree.

## Commands Run

```bash
make init-local
make -n deploy
make -n legacy-gcp-deploy
bash -n scripts/setup_gcp_project.sh
./init.sh
```

## Evidence

- Tests: `make init-local` created or confirmed the local `data/` directory and printed the default SQLite database path.
- Tests: `make -n deploy` shows the default deploy target now refuses GCP deployment and points to `legacy-gcp-deploy`.
- Tests: `make -n legacy-gcp-deploy` preserves the old Cloud Run command under an explicit legacy target.
- Tests: `bash -n scripts/setup_gcp_project.sh` passed.
- Tests: final root `./init.sh` passed, including harness verification and full backend `go test ./...`.
- Logs: README, config defaults, Upload Ask/FAQ docs, SPEC notes, and CI deploy trigger were updated so local SQLite is the ordinary path and GCP/R2/Postgres/pgvector/Valkey are optional legacy/integration paths.
- Screenshots or traces: not applicable.
- External behavior verification: no live external services were required.
- Capability gaps: real LLM and optional Google OAuth credentials are still needed for live AI/OAuth behavior outside deterministic local recovery.

## Failure Analysis

- Failure domain: none
- Failure summary: no implementation or evaluator failure in this run.
- Harness improvement: none required.
- Follow-up feature: F002 Protected API contract smoke coverage.

## Files Changed

- `README.md`
- `Makefile`
- `.github/workflows/ci.yml`
- `configs/config.yaml`
- `scripts/setup_gcp_project.sh`
- `docs/faq/faq-spec.md`
- `docs/upload-ask/architecture.md`
- `docs/upload-ask/conversational-memory-spec.md`
- `docs/upload-ask/r2-setup.md`
- `docs/upload-ask/schema.sql`
- `docs/upload-ask/upload-ask-spec.md`
- `.agent-harness/SPEC.md`
- `.agent-harness/feature_list.json`
- `.agent-harness/progress.md`
- `.agent-harness/runs/20260613T091713Z-F008-local-only-runtime-cleanup.md`

## Evaluator Result

```text
EVAL_PASS: F008
```

## Follow-Up

- Continue with F002 to add protected API contract smoke coverage.
