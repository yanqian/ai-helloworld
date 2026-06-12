# Coding Agent Prompt

Act as Coding Agent for one selected feature.

Feature ID: `Fxxx`

Default invocation: the Coding Agent prompt is normally dispatched by the orchestrator through `make work`. If you are running this prompt manually, treat that as an explicit fallback because role adapters are unavailable or the user requested interactive/manual work. Record the fallback in `progress.md` or `runs/`, and do not bypass evaluator gating, evaluator evidence, attempts, failure records, or final `./init.sh` verification.

You must:

1. Read `AGENTS.md`.
2. Read `progress.md`.
3. Read `feature_list.json`.
4. Check recent work with `git log --oneline -20`.
5. Run `./init.sh` before changing files.
6. Implement only the selected feature.
7. Preserve unrelated user changes.
8. Update `progress.md`.
9. Update only the selected feature in `feature_list.json`.
10. Run `./init.sh` after changes.
11. Record run evidence in `runs/` for non-trivial work, external behavior verification, failures, or evaluator handoff.
12. When work fails or is blocked, classify the failure using `docs/failure-domains.md` and assess whether it requires a harness improvement.
13. When a required capability is missing, follow `docs/capability-gaps.md` before using any workaround.
14. When implementation touches `examples/`, follow `docs/example-boundaries.md`.

Do not mark unrelated features done.
Do not overwrite `feature_list.json`.
Do not reset existing feature state.
Preserve existing feature IDs, ordering, `passes`, `status`, `attempts`, `last_error`, and unknown fields.
Do not stage or commit during orchestrated runs.
When relying on external CLI, API, runtime, or structured tool output behavior, verify it with a primary source or real-shaped fixture before depending on it.
Do not bypass missing tools, permissions, generators, dependencies, services, credentials, runtime settings, CI resources, or verification fixtures with hand-written generated artifacts, weakened scope, skipped verification, or local-only environment changes.
If a workaround is temporary, record it and keep the feature incomplete, blocked, or linked to a follow-up feature until the capability is durable or explicitly scoped out.
Do not implement project-level requirements by repurposing default examples such as `examples/tiny-cli` or `examples/go-server`; use project-owned source and tests unless the feature explicitly targets examples.
Do not leave repeated failures as retries only; convert harness weaknesses into docs, prompts, scripts, schemas, tests, or a new feature entry.

Return:

- Feature implemented.
- Files changed.
- Verification commands run.
- Remaining issues.
- Failure domain and harness improvement assessment when applicable.
- Capability gaps and durable capability changes or follow-up feature when applicable.
- Example-boundary assessment when `examples/` changed.
- Suggested commit message.
