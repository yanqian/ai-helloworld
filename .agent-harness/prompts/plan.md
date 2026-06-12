# Planning Agent Prompt

Act as Planning Agent for this repository.

You must:

1. Read `AGENTS.md`.
2. Read `SPEC.md`.
3. Read `feature_list.json`.
4. Check recent work with `git log --oneline -20`.
5. Run `./init.sh`.
6. Use `docs/spec-normalization.md` to normalize the new requirement into clear additions to `SPEC.md`.
7. The SPEC addition must include goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.
8. Reject vague requirements instead of turning them directly into feature entries; ask for clarification, record explicit assumptions, or create capability/blocker/follow-up work when needed.
9. Use `docs/feature-decomposition.md` to split broad requirements into independently verifiable feature entries.
10. Use `docs/project-recovery-init.md` when the requirement initializes a fresh project, accepts a minspec, or changes root `./init.sh`.
11. Identify required capabilities such as tools, permissions, generators, dependencies, services, credentials, runtime settings, CI resources, and verification fixtures.
12. Identify project-owned implementation and verification paths; use `examples/` only when the requirement explicitly targets example maintenance.
13. Append one or more new features to `feature_list.json`, including explicit capability features when the requirement depends on missing or unclear capabilities.
14. Preserve existing feature IDs, ordering, `passes`, `status`, `attempts`, `last_error`, and unknown fields.
15. Validate JSON and uniqueness with `./init.sh`.

Decomposition rules:

- Spec normalization happens before feature decomposition.
- Split features by independently testable user-visible behavior, required capability, implementation boundary, verification surface, risk domain, or deferrable dependency.
- If a feature would need more than five acceptance criteria, consider it over-bundled and split it unless there is a clear reason not to.
- Do not append a generic "implement all requirements" feature when the request contains multiple independently verifiable behaviors.
- If broad work is intentionally kept as one feature, record why it remains coherent and independently evaluable.
- If a minspec has been accepted and no runnable skeleton exists, append a project recovery feature before product behavior features depend on runtime assumptions.
- A project recovery feature must require root `./init.sh` to install dependencies, start required services, run a real endpoint or core-function smoke test, emit clear logs, be idempotent, avoid manual steps or TODO placeholders, and exit non-zero on failure.
- Harness verification alone is acceptable only before a minspec exists; after minspec acceptance, root `./init.sh` must become the project recovery contract.

New feature defaults:

- `passes=false`
- `status="todo"`
- `attempts=0`
- `last_error=""`

Do not implement business logic during planning.

Return:

- SPEC sections changed.
- Spec normalization decisions, including assumptions, ambiguities, and excluded scope.
- Feature IDs appended.
- Decomposition decisions and any intentionally merged broad work.
- Validation commands run.
- Remaining planning risks.
