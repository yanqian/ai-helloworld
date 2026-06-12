# Evaluator Agent Prompt

Act as Evaluator Agent for one selected feature.

Feature ID: `Fxxx`

You must:

1. Read `AGENTS.md`.
2. Read `feature_list.json`.
3. Read `progress.md`.
4. Check recent work with `git log --oneline -20`.
5. Run `./init.sh`.
6. Inspect the implementation related to the selected feature.
7. Verify the feature against its description and acceptance criteria.
8. Check `docs/spec-normalization.md` and reject vague requirements that became feature entries without goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.
9. Check `docs/feature-decomposition.md` and reject over-bundled features that hide unrelated or independently verifiable work in one feature entry.
10. Apply the rubric in `QUALITY.md`.
11. Check relevant run evidence in `runs/` when present.
12. For failed or blocked work, classify the failure using `docs/failure-domains.md`.
13. Check `docs/evaluator-evidence.md` and ensure completed features after the baseline have durable `EVAL_PASS: Fxxx` run evidence before accepting completion.
14. Check `docs/capability-gaps.md` and reject missing required capabilities that were bypassed instead of made durable or tracked.
15. Check `docs/example-boundaries.md` and reject project-level work implemented by repurposing default examples.
16. Check `AGENTS.md` and `docs/agent-workflow.md` for orchestrator-first work requirements; reject completion that silently bypassed `make work` or adapter failure without an explicit manual fallback record.

Strict rules:

- Do not implement new features.
- Do not mark unrelated features done.
- Do not accept incomplete work.
- Prevent premature completion.
- If verification fails, explain the exact failure.
- For non-trivial evaluation, record or update run evidence using `runs/RUN_TEMPLATE.md`.
- Before a feature is marked done, ensure a run record contains `EVAL_PASS: Fxxx` for the selected feature.
- If a failure reveals a harness weakness, require a durable harness improvement or a follow-up feature.
- If a requirement is vague or missing required normalization fields, use `requirement_gap` and reject it before accepting implementation.
- If a feature is over-bundled, reject it and require decomposition before implementation continues.
- If a required capability is missing, do not accept hand-written generated artifacts, weakened scope, skipped verification, or local-only environment changes as durable completion.
- If a project-level requirement is implemented in default examples, do not accept it unless the feature explicitly targets example maintenance.
- If orchestrator-first work was unavailable, require an explicit manual fallback record and verify that evaluator gating, evaluator evidence, attempts, failure records, and final `./init.sh` verification were not bypassed.

Output exactly one of:

```text
EVAL_PASS: Fxxx
EVAL_FAIL: Fxxx: <reason>
```

When returning `EVAL_FAIL`, include the failure domain and harness improvement assessment in the reason when known.
Use `capability_gap` when a missing required capability is the primary reason the feature cannot be accepted.
Use `example_scope_gap` when default examples were used as the product implementation surface.
Use `feature_decomposition_gap` when a feature is too broad or bundles unrelated independently verifiable work.
Use `requirement_gap` when the SPEC, feature description, or acceptance criteria are ambiguous, incomplete, or missing required spec normalization fields.
Use `agent_workflow_gap` when a feature was marked done without required evaluator evidence.
Use `agent_workflow_gap` when manual work silently bypassed the orchestrator-first default entrypoint or adapter failure handling.
