# Test Plan

## Verification Entry Point

All automated verification must run through:

```bash
./init.sh
```

`init.sh` must exit non-zero on failure.

## Baseline Checks

The initialization baseline verifies:

- Required state files exist.
- `feature_list.json` is valid and internally consistent.
- Feature IDs are unique.
- `AGENTS.md` contains the core guardrails.
- `docs/README.md`, `QUALITY.md`, and `runs/RUN_TEMPLATE.md` are present and indexed.
- Failed run records include failure-domain classification and harness improvement assessment.
- `orchestrator.py` compiles.
- The tiny example tests pass.
- The Go server example tests pass when Go is installed.
- Unit tests pass.
- Contract tests pass.
- Optional harness tests pass when `test/harness/` exists.
- Smoke tests pass.
- `make ci` runs the same verification path as GitHub Actions.
- README positioning and OSS readiness files are covered by contract tests.
- `make clean` behavior is covered by unit tests using a temporary project root.
- Real-world usage references are covered by contract tests.
- AI Agent Harness skill initialization modes, manifests, drift diagnostics, repair behavior, and installed-harness semantics are covered by harness tests using temporary project roots.

## Test Layers

### Unit

Unit tests cover deterministic helper behavior and should not call external services.

Current command:

```bash
python3 -m unittest discover -s test/unit -p 'test_*.py'
```

### Contract

Contract tests lock repository-level promises that should not drift accidentally:

- AGENTS guardrail sections and external behavior verification obligations.
- Feature schema and feature state safety expectations.
- Prompt role requirements and AI履约 restrictions.
- Orchestrator CLI, startup, role-adapter, state-transition, and eval-only contracts by static inspection.

Current command:

```bash
python3 -m unittest discover -s test/contract -p 'test_*.py'
```

### Harness

Harness tests cover multi-step repository workflows that require temporary project roots or copied harness installations.

When `test/harness/` exists, `./init.sh` runs:

```bash
python3 -m unittest discover -s test/harness -p 'test_*.py'
```

### Smoke

Smoke tests run primary non-recursive helper commands end to end. They do not run `orchestrator.py` or `scripts/validate-feature.sh`, because both commands run `./init.sh`; invoking them from tests that are themselves run by `./init.sh` would create recursive verification.

Current command:

```bash
python3 -m unittest discover -s test/smoke -p 'test_*.py'
```

## Feature Verification Matrix

| Feature | Verification Requirement |
| --- | --- |
| F001 | State validation, feature validation, progress summary, and tiny example tests pass. |
| F002 | Contract tests statically verify orchestrator CLI and startup contract; manual verification may run dry-run and eval-only dry-run outside `./init.sh`. |
| F003 | Contract validation proves AGENTS guardrails remain present. |
| F004 | `./init.sh` runs unit, contract, smoke, and optional harness layers. |
| F005 | Contract tests verify AI agent obligations and harness boundaries, not only file presence. |
| F006 | Contract tests verify explicit Coding Agent and Evaluator Agent adapters replace generic agent command dispatch. |
| F007 | Contract and state validation verify docs knowledge map, quality rubric, and run artifact template. |
| F008 | Contract, unit, and smoke tests verify failure-domain classification and harness improvement checks. |
| F009 | `./init.sh` verifies the Go server example when Go is installed. |
| F010 | `Makefile` and GitHub Actions run shared CI verification. |
| F012 | Contract tests verify README positioning and onboarding for resumable AI coding. |
| F013 | Contract tests verify OSS readiness files and issue templates. |
| F014 | Unit and contract tests verify clean-state reset behavior and Makefile wiring. |
| F015 | Contract tests verify real-world usage references in README and docs. |
| F016 | Contract tests verify the README announcement article link. |
| F017 | Contract and unit tests verify the AI Agent Harness skill exists, documents workflows, and initializes projects safely. |
| F018 | Harness tests verify skill initializer `new`, `adopt`, `repair`, and `check` modes, manifest-based drift diagnostics, non-overwrite behavior, and runnable installed-harness semantics. |

## Manual Orchestrator Verification

Run these outside `./init.sh` when changing `orchestrator.py` behavior:

```bash
python3 orchestrator.py --dry-run
python3 orchestrator.py --eval-only F001 --dry-run
scripts/validate-feature.sh F001
```
