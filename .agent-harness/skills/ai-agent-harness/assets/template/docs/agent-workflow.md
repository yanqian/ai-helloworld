# Agent Workflow

## Planning

Use `prompts/plan.md` for new requirements.

Planning appends to `SPEC.md` and `feature_list.json`. It does not implement business logic.

Planning follows `docs/spec-normalization.md` before decomposition. New requirements must become SPEC additions with goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.

Planning follows `docs/feature-decomposition.md` so broad requirements become independently verifiable feature entries instead of one over-bundled feature.

Planning follows `docs/project-recovery-init.md` for fresh projects and accepted minspecs. Before a minspec exists, root `./init.sh` may verify only the harness. After minspec acceptance, planning must add a runnable-skeleton feature that turns root `./init.sh` into the project recovery contract before product behavior depends on runtime assumptions.

## Coding

Use the orchestrator as the default entrypoint for one selected feature:

```bash
make work
```

`make work` runs one orchestrator round for the next unfinished feature. The orchestrator owns feature selection, `in_progress` state, attempt increments, Coding Agent dispatch, Evaluator Agent dispatch, pass/fail state transitions, failure records, and evaluator-gated completion.

Before real provider execution, configure `agent-provider.json` from `agent-provider.example.json` using `docs/agent-provider-configuration.md`. Missing, ambiguous, or unavailable provider setup is a capability gap and must fail closed before completion.

Use `prompts/work.md` manually only as an explicit fallback when role adapters are not configured, unavailable, or the user asks for interactive work.

Manual fallback still follows the Coding Agent contract: run `./init.sh` before and after changes, update only the selected feature state, record progress, and note that manual work was a fallback in `progress.md` or `runs/`.

Manual fallback must not bypass evaluator pass, evaluator evidence, attempts, failure records, or final `./init.sh` verification. If adapter failure blocks `make work`, treat that as a capability gap or follow-up feature instead of silently hand-editing completion state.

When a required tool, dependency, generator, permission, service, credential, runtime setting, CI resource, or verification fixture is missing, follow `docs/capability-gaps.md`. Do not convert the missing capability into an untracked local workaround.

When implementing project requirements, follow `docs/example-boundaries.md`. Use default examples as references only unless the selected feature explicitly targets example maintenance.

## Evaluation

Use `prompts/evaluate.md`.

The Evaluator Agent checks the feature against acceptance criteria and `QUALITY.md`. It must output exactly one pass or fail line.

Evaluation follows `docs/evaluator-evidence.md`. From the evaluator-evidence baseline onward, a feature cannot be marked done unless a run record contains `EVAL_PASS: Fxxx` for that feature.

Evaluation rejects over-bundled features that should have been decomposed before implementation.

Evaluation rejects features whose SPEC source was vague and should have been normalized before feature entries were appended.

Evaluation rejects features that bypass required capability gaps instead of making them durable or tracking them as blocked or follow-up work.

Evaluation rejects project-level features that pass only by repurposing default examples.

Evaluation rejects project-level completion when accepted minspec work only verifies the harness and leaves root `./init.sh` without dependency setup, service startup, and a real smoke test.

## Continuation

Use `prompts/continue.md` after interruptions.

Continuation reconstructs context from repository files and git history only.

When implementation or evaluation remains, continue through `make work` first. Use manual continuation only as an explicit fallback when adapters are unavailable or the user asks for interactive/manual work.

## Finalize And Commit

Use `docs/commit-messages.md` for approved commits. Feature work must include the feature ID in the commit subject so git history can be analyzed against `feature_list.json`.

## Run Artifacts

For non-trivial work, create a run note from `runs/RUN_TEMPLATE.md`.

Run notes capture commands, evidence, decisions, failures, and next actions.

## Failure Improvement

When work fails, classify the failure using `docs/failure-domains.md`.

Every failed or blocked run should record:

- Failure domain.
- Failure summary.
- Harness improvement assessment.
- Follow-up feature ID if the improvement is deferred.

Use `scripts/check-failure-domains.sh` to verify failed run records include the classification and improvement assessment.

Use `scripts/check-evaluator-evidence.sh` to verify completed features after the enforcement baseline have durable evaluator pass evidence.
