# Spec Normalization

Planning Agent work must turn vague user input into an explicit, durable SPEC addition before feature entries are appended.

## Required Fields

Every non-trivial new requirement must be normalized with these fields:

- Goal: the concrete outcome the project or feature should achieve.
- Scope included: behavior, surfaces, users, data, commands, endpoints, screens, jobs, or workflows included in this requirement.
- Scope excluded: adjacent behavior that is explicitly out of scope for this requirement.
- Core flows: the main user, system, or operator flows that must work.
- Constraints: technical, runtime, platform, dependency, security, privacy, data, performance, compatibility, or deployment limits.
- Ambiguities or assumptions: unclear points, decisions made by assumption, and questions that still need user input.
- Required capabilities: tools, permissions, generators, dependencies, services, credentials, runtime settings, CI resources, and verification fixtures required to implement or verify the work.
- Implementation paths: project-owned source, setup, contract, documentation, and test paths expected to change.
- Verification surface: tests, smoke checks, contract checks, commands, endpoints, or inspection steps that can prove the work.

## No Vague Features

Planning Agents must not convert fuzzy phrases directly into feature entries. Examples of vague input include "make it better", "support auth", "build the dashboard", "production ready", or "do the frontend" when goal, scope, flows, constraints, and verification are not clear enough to evaluate.

If the normalized SPEC cannot be written without guessing, the Planning Agent must do one of these:

- ask for clarification;
- record explicit assumptions in `SPEC.md`;
- mark the ambiguity as a planning risk;
- append a capability, blocker, or follow-up feature before product behavior work;
- keep the work unplanned until the missing information exists.

## Relationship To Feature Decomposition

Spec normalization happens before feature decomposition.

After the SPEC entry is clear, use `docs/feature-decomposition.md` to split work into independently verifiable feature entries. A feature acceptance list should trace back to the normalized goal, included scope, core flows, constraints, and verification surface.

## Evaluation

Evaluators must reject planned or implemented work when the accepted feature depends on ambiguous SPEC content that should have been normalized first. Use `requirement_gap` when missing goal, scope, flow, constraint, ambiguity, capability, implementation path, or verification detail is the primary problem.
