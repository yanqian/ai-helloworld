# Quality Rubric

Evaluator Agents use this rubric when deciding whether a feature is complete.

The feature must satisfy its `feature_list.json` acceptance criteria and preserve the repository harness contracts. Tests passing is necessary but not always sufficient.

## Criteria

### Correctness

- The implementation satisfies the feature description and acceptance criteria.
- The implementation handles expected success and failure paths.
- The implementation does not depend on unverified external behavior.

### Completeness

- The implemented surface matches the requested scope.
- The source requirement was normalized with clear goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.
- The selected feature is not over-bundled with unrelated or independently verifiable work.
- Documentation, prompts, scripts, examples, and tests are updated when affected.
- No required follow-up is hidden as "future work" unless explicitly accepted.
- Required capabilities are provided, documented, automated, or explicitly tracked as blocked or follow-up work.
- Project-level requirements are implemented in project-owned paths, not default examples, unless the feature explicitly targets example maintenance.

### Maintainability

- The change follows existing repository structure and naming.
- The change avoids unnecessary abstractions.
- Durable knowledge is stored in repository files, not chat history.

### Test Coverage

- Unit, contract, smoke, and relevant harness tests cover the behavior.
- Tests assert meaningful outcomes, not only file existence.
- Real-shaped fixtures or primary-source verification support external tool assumptions.

### Recoverability

- A future agent can resume from `AGENTS.md`, `SPEC.md`, `feature_list.json`, `progress.md`, `docs/`, `QUALITY.md`, `runs/`, and git history.
- State files remain valid and internally consistent.
- Feature commits can be traced back to `feature_list.json` through their commit subject.
- Completed features after the evaluator-evidence baseline have `EVAL_PASS: Fxxx` evidence in `runs/`.
- Run evidence is recorded when the work involves non-trivial verification or failure analysis.
- Failed or blocked work is classified with a failure domain and assessed for harness improvement.

### Safety

- The change preserves unrelated user work.
- The change does not broaden execution authority accidentally.
- The evaluator rejects premature completion.
- The implementation does not hide missing tools, permissions, generators, dependencies, or environment setup behind local-only workarounds.
- The implementation does not repurpose default examples as project product code.

## Pass Guidance

Return `EVAL_PASS: Fxxx` only when:

- All relevant verification commands pass.
- Acceptance criteria are met.
- The rubric above has no unresolved critical issue.

Return `EVAL_FAIL: Fxxx: <reason>` when:

- A required behavior is missing.
- The feature depends on a vague or incomplete SPEC entry that should have been normalized before implementation.
- The feature is over-bundled and should have been decomposed before implementation.
- State is inconsistent.
- Verification was skipped or inconclusive.
- External behavior was assumed without evidence.
- A required capability gap was bypassed instead of made durable or tracked.
- A project-level requirement was implemented inside default examples instead of project-owned source and tests.
- A failed run lacks failure-domain classification or harness improvement assessment.
- A feature was marked done without required evaluator evidence.
