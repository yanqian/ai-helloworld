# Architecture

The template is a repository-local harness, not a hosted service.

## Core Files

- `AGENTS.md` - agent entry point and workflow rules.
- `SPEC.md` - product and harness requirements.
- `feature_list.json` - machine-readable feature state.
- `progress.md` - human-readable recovery status.
- `QUALITY.md` - evaluator rubric.
- `orchestrator.py` - optional one-feature-at-a-time role orchestrator.

## Extension Points

- `scripts/run-coding-agent.sh` invokes the project-specific Coding Agent.
- `scripts/run-evaluator-agent.sh` invokes the project-specific Evaluator Agent.
- `scripts/init.sh` is the root verification implementation.
- `test/harness/` is reserved for project-specific workflow tests.
- `runs/` stores per-run evidence and handoff records.

## Boundaries

- The orchestrator owns unattended state transitions.
- Role adapters own vendor-specific agent invocation.
- Contract tests own harness invariants.
- Downstream projects own product-specific implementation and verification.
- Default examples under `examples/` are demonstration fixtures, not the default location for downstream product features.
