# Failure Domains

The harness improves when failures are classified and converted into durable changes.

`last_error` explains what failed. A failure domain explains what kind of loop improvement may prevent similar failures.

## Domains

- `requirement_gap` - the spec, feature description, or acceptance criteria were ambiguous, incomplete, or missing required spec normalization fields such as goal, scope, core flows, constraints, assumptions, capabilities, implementation paths, or verification surface.
- `feature_decomposition_gap` - broad requirements were collapsed into one over-bundled feature instead of independently verifiable feature entries.
- `implementation_gap` - the product code did not satisfy a clear requirement.
- `test_gap` - existing tests missed behavior that should have been caught earlier.
- `contract_gap` - harness rules, prompts, schema, or state contracts allowed unsafe or ambiguous behavior.
- `external_behavior_gap` - CLI, API, runtime, or structured tool output behavior was assumed without evidence.
- `capability_gap` - a required tool, permission, generator, dependency, service, credential, runtime setting, CI resource, or verification fixture was missing or implicit and was not turned into a durable project capability.
- `example_scope_gap` - product work was implemented in default examples instead of project-owned source, contracts, documentation, and tests.
- `state_recovery_gap` - progress, run records, feature state, or docs were insufficient for resuming work.
- `agent_workflow_gap` - role boundaries, orchestration, evaluator behavior, continuation flow, or evaluator-evidence gating were unclear or bypassed.
- `environment_gap` - local dependencies, permissions, network, OS behavior, or CI/runtime setup blocked verification.

## Improvement Loop

For every failed or blocked run:

1. Record the exact failure in `last_error` and the run record.
2. Assign one primary failure domain.
3. Decide whether the harness needs an improvement.
4. If yes, update the relevant durable artifact: `AGENTS.md`, `QUALITY.md`, `docs/`, `prompts/`, `scripts/`, `schemas/`, tests, or `feature_list.json`.
5. If no, record why the failure is only a product implementation issue.
6. Add or update tests when the failure should be automatically caught next time.

Repeated failures in the same domain should become a harness improvement feature, not only another retry.

## Evidence

Use `runs/RUN_TEMPLATE.md` to record:

- Failure domain.
- Failure summary.
- Harness improvement assessment.
- Follow-up feature ID when an improvement is deferred.
