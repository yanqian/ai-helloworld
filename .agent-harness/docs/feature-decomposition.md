# Feature Decomposition

Planning Agent output must turn user requirements into independently verifiable feature entries. Feature count is determined by acceptance boundaries, not by how many sentences the user wrote.

## Core Rule

A feature is the smallest valuable behavior or capability that a Coding Agent can implement in one focused run and an Evaluator Agent can independently pass or fail.

The Planning Agent must first identify:

- user-visible behaviors;
- required project capabilities;
- implementation boundaries;
- verification boundaries;
- dependencies between the work items.

Then it appends one or more features to `feature_list.json`.

## Split Triggers

Split a requirement into multiple features when any item is true:

- The work has multiple independently testable user-visible behaviors.
- Different capabilities are required, such as database setup, API contracts, UI flows, external integrations, credentials, CI resources, or deployment configuration.
- Different implementation boundaries are involved, such as CLI, backend, frontend, data migration, documentation, test harness, or infrastructure.
- One part can fail, be blocked, or be deferred while another part could still be accepted.
- Acceptance criteria describe several natural completion points.
- The work crosses distinct risk domains such as auth, payment, security, data persistence, external APIs, or runtime operations.
- The feature would need more than five acceptance criteria to be evaluated clearly.

## Allowed Merges

Keep related work in one feature when all items are true:

- It forms one coherent behavior or capability.
- It can be implemented and evaluated in one focused run.
- Its acceptance criteria share the same verification surface.
- Splitting it would create entries with no independent user or project value.

Small documentation, test, and setup changes may stay with the feature they directly verify.

## Required Planning Output

For non-trivial requirements, Planning Agent output must explain the decomposition decision in `SPEC.md` or the planning response:

- which features were appended;
- why they are separate or intentionally merged;
- any capability feature that must happen before product behavior work;
- the expected implementation and verification path for each feature.

If the agent intentionally creates one feature from a broad requirement, it must state why the feature remains independently verifiable and not over-bundled.

## Evaluation

Evaluators must reject an over-bundled feature when unrelated or independently verifiable work was hidden inside one feature entry. Use `feature_decomposition_gap` when poor feature boundaries are the primary failure.

Examples of failures:

- A single feature combines user auth, billing, admin UI, and deployment.
- A feature acceptance list bundles unrelated backend, frontend, and infrastructure work without dependency boundaries.
- A Planning Agent appends one generic "implement app requirements" feature instead of discrete behavior and capability features.

Examples of acceptable single features:

- Add one CLI command with its parser, tests, and README usage.
- Add one API endpoint with validation, persistence, tests, and contract documentation.
- Add a required code generation workflow before product features depend on generated artifacts.
