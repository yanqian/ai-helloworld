# Testing

All automated verification starts with:

```bash
./init.sh
```

## Layers

- Unit tests: deterministic helper behavior.
- Contract tests: repository rules, schema, prompts, and agent obligations.
- Harness tests: multi-step workflow behavior, including temporary project initialization and repair scenarios when present.
- Smoke tests: non-recursive command checks.
- Failure-domain checks: failed run records must include domain and harness improvement assessment.

## Recursive Command Boundary

Do not run commands from `./init.sh` tests when those commands call `./init.sh` themselves.

Run these manually or from CI jobs outside `./init.sh`:

```bash
python3 orchestrator.py --dry-run
python3 orchestrator.py --eval-only F001 --dry-run
scripts/validate-feature.sh F001
```

Failure-domain checks are non-recursive and run inside `./init.sh`:

```bash
scripts/check-failure-domains.sh
```

## Adding Tests

When adding a feature:

1. Add unit tests for pure logic.
2. Add contract tests for durable behavior boundaries.
3. Add harness tests for multi-step workflows.
4. Add smoke tests for stable top-level commands.
