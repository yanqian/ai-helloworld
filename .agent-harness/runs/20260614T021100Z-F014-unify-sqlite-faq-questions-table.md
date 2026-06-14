# Run Record: F014 - Unify SQLite FAQ questions table

## Summary

- Date: 20260614T021100Z
- Agent role: Manual Coding Agent fallback plus Evaluator Agent
- Feature: F014
- Result: pass

## Repository State

- Starting commit: 61bdaf4
- Ending commit: 61bdaf4
- Note: `make work` selected F014 but the configured Codex provider failed before business work due a runtime permission issue.

## Commands Run

```bash
make work
GOCACHE=/tmp/ai-helloworld-go-build GOMODCACHE=/tmp/ai-helloworld-go-mod go test -count=1 ./internal/infra/sqlite ./internal/infra/faqrepo ./internal/infra/faqstore ./cmd/app ./internal/interface/http
./init.sh
git diff --check
```

## Evidence

- `make work` selected F014 but failed before coding because the Codex provider could not write `/Users/armstrong/.codex/state_5.sqlite` and could not initialize the in-process app-server client.
- Manual fallback implemented the SQLite FAQ schema unification after orchestrator-first execution failed for provider/runtime reasons.
- Focused tests passed and cover:
  - fresh SQLite schema creates `questions` and does not create `faq_questions`;
  - legacy `questions` gains `created_at` without losing rows;
  - legacy `faq_questions` rows migrate into `questions`;
  - duplicate `faq_questions.question_text` rows do not overwrite existing `questions` rows;
  - `faq_answer_cache` is rebuilt to reference `questions(id)` and preserves rows;
  - SQLite FAQ repository/store behavior continues through `questions`;
  - app/provider and HTTP contract tests continue to pass.

## Files Changed

- `.agent-harness/SPEC.md`
- `.agent-harness/feature_list.json`
- `.agent-harness/progress.md`
- `.agent-harness/runs/20260614T020323Z-F014-failure.md`
- `README.md`
- `docs/faq/faq-spec.md`
- `internal/infra/sqlite/db.go`
- `internal/infra/sqlite/db_test.go`
- `internal/infra/faqrepo/sqlite_repository.go`
- `internal/infra/faqstore/sqlite_store_test.go`

## Evaluator Result

```text
EVAL_PASS: F014
```
