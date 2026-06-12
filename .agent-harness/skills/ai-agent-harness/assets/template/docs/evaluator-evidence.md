# Evaluator Evidence

Evaluator gating must leave durable evidence in the repository.

## Rule

From the evaluator-evidence enforcement baseline onward, a feature may be marked complete only when both are true:

- `feature_list.json` has `passes=true` and `status="done"` for that feature.
- A run record under `runs/` contains the exact evaluator pass line `EVAL_PASS: Fxxx`.

Verification commands such as `./init.sh`, unit tests, contract tests, or smoke tests are necessary evidence, but they are not a substitute for the evaluator result. The evaluator pass line is the durable proof that the feature was judged against its acceptance criteria.

## Baseline

The default enforcement baseline is `F027`. Earlier features were completed before this guardrail existed and are not retroactively required to have run-level evaluator evidence.

Projects may set `HARNESS_EVALUATOR_EVIDENCE_BASELINE=Fxxx` when adopting the harness into an existing repository with a different baseline.

## Evaluation

Evaluators must record or update a run record for non-trivial evaluation before a feature is marked done. If a feature is marked done without matching evaluator evidence, verification must fail and the primary failure domain is `agent_workflow_gap`.
