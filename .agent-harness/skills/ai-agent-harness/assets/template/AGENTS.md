# AGENTS.md

This repository uses a long-running AI agent harness.

Agents must behave like stateless workers:

- No durable memory between sessions.
- All durable state lives in repository files.
- Every session reconstructs context from files and git history.

## Core Principle

Never rely on chat history. Always rely on project state.

## Required Startup Protocol

Before planning, coding, evaluating, or resuming work, agents must:

1. Read `progress.md`.
2. Read `feature_list.json`.
3. Check recent work:

   ```bash
   git log --oneline -20
   ```

4. Run:

   ```bash
   ./init.sh
   ```

## Roles

### Initializer

Use this role when bootstrapping a new repository from the template.

Responsibilities:

- Create or adapt `SPEC.md`.
- Create or adapt `feature_list.json`.
- Create or adapt `progress.md`.
- Create or adapt `init.sh`.
- Initialize git when the repository has no git history.
- Do not implement business logic during initialization.
- Before a project minspec exists, `init.sh` may only verify the harness and must not claim project business behavior is runnable.

### Planning Agent

Use this role for new requirements before implementation begins.

Responsibilities:

- Read `AGENTS.md`.
- Read `SPEC.md`.
- Read `feature_list.json`.
- Normalize the new requirement into explicit SPEC additions.
- Decompose broad requirements into independently verifiable feature entries.
- Use `docs/project-recovery-init.md` when planning a fresh project or runnable skeleton.
- Append new feature entries to `feature_list.json`.
- Preserve all existing feature IDs, order, status fields, attempts, errors, and unknown fields.

Strict rules:

- Do not modify existing feature IDs.
- Do not reorder existing features.
- Do not reset existing feature state.
- Only append new feature entries unless explicitly instructed otherwise.
- Use `docs/spec-normalization.md` before feature decomposition.
- Do not turn vague requirements into feature entries without explicit goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.
- Do not collapse unrelated or independently verifiable work into one feature.
- Use `docs/feature-decomposition.md` to decide whether to split or intentionally merge work.
- If a minspec has been accepted and the project has no runnable skeleton, append a project recovery feature before product behavior depends on runtime assumptions.
- Ensure `feature_list.json` remains valid JSON.
- Ensure feature IDs remain unique.

### Coding Agent

Use this role to implement exactly one feature.

Default entrypoint:

- For one-feature implementation and evaluation, use the orchestrator first:

  ```bash
  make work
  ```

- Use manual or interactive Coding Agent work only as an explicit fallback when role adapters are not configured, unavailable, or the user explicitly asks for manual work.
- Manual fallback must be recorded in `progress.md` or `runs/` and must not bypass evaluator pass, evaluator evidence, attempts, failure records, or final `./init.sh` verification.

Responsibilities:

- Follow the startup protocol.
- Implement only the selected feature.
- Keep the system runnable.
- Update `progress.md`.
- Update only the selected feature in `feature_list.json`.
- Preserve unknown feature fields and feature ordering.
- Do not stage or commit during orchestrated runs.
- Do not modify unrelated pre-existing working-tree changes.

The Coding Agent must not mark unrelated features as done.

### Evaluator Agent

Use this role to verify one feature.

Responsibilities:

- Follow the startup protocol.
- Inspect the implementation for the selected feature.
- Run relevant tests or harness checks.
- Verify the feature against its description and acceptance criteria.

Strict rules:

- Do not implement new features.
- Do not mark unrelated features done.
- Do not accept incomplete work.
- Prevent premature completion.
- If verification fails, explain the exact failure.

The Evaluator Agent must output exactly one of:

```text
EVAL_PASS: Fxxx
EVAL_FAIL: Fxxx: <reason>
```

### Orchestrator

`orchestrator.py` owns the default one-feature work entrypoint and optional unattended feature execution.

Responsibilities:

- Follow the startup protocol before doing anything else.
- Pick one unfinished feature per round.
- Mark the selected feature `status="in_progress"`.
- Increment the selected feature's `attempts`.
- Run a Coding Agent prompt for that feature.
- Run an Evaluator Agent prompt for that feature.
- Mark the feature done only after evaluator pass.
- Mark the feature failed or blocked after coding or evaluation failure.
- Write a failure run record when unattended coding or evaluation fails.

The template orchestrator is vendor-neutral. By default it supports `--dry-run` prompt preview. To execute real agents, replace the role adapters:

- `scripts/run-coding-agent.sh`
- `scripts/run-evaluator-agent.sh`

The orchestrator sends each role prompt to the corresponding adapter on stdin.

If either role adapter is missing, not executable, or still the template adapter, real orchestrator work must fail closed with clear configuration guidance before silently treating manual state edits as completion.

Default adapters delegate to `scripts/run-agent-provider.py`. Configure them by copying `agent-provider.example.json` to `agent-provider.json` and selecting an explicit provider. Do not guess between Codex, Claude Code, Cursor Agent, or custom providers; missing, ambiguous, or unavailable provider setup is a capability gap.

## State Files

## Repository Knowledge Map

`AGENTS.md` is the entry point, not the whole manual.

Use these durable knowledge files:

- `docs/README.md` for the repository knowledge index.
- `docs/architecture.md` for structure and boundaries.
- `docs/testing.md` for verification layers.
- `docs/agent-provider-configuration.md` for explicit Codex, Claude Code, Cursor Agent, and custom provider configuration.
- `docs/spec-normalization.md` for converting vague requirements into clear SPEC additions before feature decomposition.
- `docs/feature-decomposition.md` for requirement-to-feature splitting rules.
- `docs/project-recovery-init.md` for separating harness verification from project recovery.
- `docs/evaluator-evidence.md` for requiring durable evaluator pass evidence before feature completion.
- `docs/commit-messages.md` for linking commits to feature IDs.
- `docs/external-behavior.md` for CLI, API, runtime, and tool-output verification rules.
- `docs/capability-gaps.md` for missing tools, permissions, dependencies, generators, environment setup, and required project capabilities.
- `docs/example-boundaries.md` for rules that keep default examples from becoming project implementation shortcuts.
- `docs/agent-workflow.md` for planning, coding, evaluation, continuation, and run artifacts.
- `docs/failure-domains.md` for failure classification and harness improvement rules.
- `QUALITY.md` for evaluator criteria.
- `runs/` for per-run evidence and handoff records.

### `feature_list.json`

`feature_list.json` is the machine-readable feature scope and state.

Feature entries must be independently verifiable units of work. Planning Agents must split broad requirements when multiple user-visible behaviors, capability dependencies, implementation boundaries, risk domains, or verification surfaces are present. If a broad requirement is intentionally represented by one feature, the planning output must explain why it remains independently verifiable.

Each feature must include:

- `id` string, such as `F001`
- `title` string
- `description` string
- `acceptance` array of strings
- `passes` boolean
- `status` string: `todo`, `in_progress`, `done`, or `blocked`
- `attempts` integer
- `last_error` string
- optional metadata such as `priority`

Rules:

- `passes=true` means complete.
- `passes=false` means incomplete.
- `status` is orchestration state and must not conflict with `passes`.
- `status=blocked` means temporarily skipped after repeated failures.
- `attempts` is incremented when the orchestrator starts a round for that feature.
- Agents must not delete unknown fields.
- Agents must preserve unknown fields.

## State Safety Rules

- Do not overwrite the entire `feature_list.json` unnecessarily.
- Update only the current feature during Coding Agent work.
- Preserve feature ordering and existing fields.
- Do not remove metadata fields.
- Do not reset existing fields such as `passes`, `status`, `attempts`, or `last_error` unless explicitly instructed.

## Spec Normalization

Planning Agents must normalize new requirements before appending feature entries. The normalized SPEC addition must define goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, project-owned implementation paths, and verification surface.

If a requirement cannot be normalized without guessing, the Planning Agent must ask for clarification, record explicit assumptions, mark planning risks, or append capability, blocker, or follow-up features before product behavior work. Vague requirements must not become executable features merely because they sound actionable.

### `progress.md`

`progress.md` must include:

- Current system status.
- Last completed feature.
- Next feature.
- Known issues.
- Recovery notes when useful.

The Coding Agent updates `progress.md` after implementation work.

## Failure Improvement Loop

Failures must improve the harness when they reveal a weak loop.

When a feature fails, is blocked, or exposes repeated evaluator feedback:

- Record the concrete failure in `last_error`.
- Record run evidence in `runs/`.
- Assign one primary failure domain from `docs/failure-domains.md`.
- Assess whether the failure requires a harness improvement.
- Convert harness failures into durable changes such as docs, prompts, scripts, schemas, contract tests, smoke tests, or a new feature entry.
- If no harness improvement is required, record why the failure is only an implementation issue.

Repeated failures in the same domain must not remain only retries. They should become an explicit harness improvement feature or a committed harness rule/test change.

## Capability Gap Handling

A missing tool, permission, generator, dependency, service, credential, runtime setting, CI resource, or verification fixture is a capability gap when it is required to implement or verify the selected feature durably.

Rules:

- Verify the gap with a primary source, real command output, official documentation, or captured logs before relying on it.
- Do not bypass a capability gap by hand-writing generated artifacts, weakening acceptance criteria, skipping verification, or making local-only environment changes that future agents and CI cannot reproduce.
- Resolve the gap with a durable project capability such as setup documentation, scripts, adapters, fixtures, CI configuration, dependency declarations, or tests.
- If the durable capability is out of scope for the selected feature, mark the feature blocked or append a follow-up feature instead of treating the workaround as completion.
- Temporary workarounds must be recorded in `runs/`, `progress.md`, or `last_error`, and the feature must not be marked complete until the capability is provided or explicitly scoped out.

## Example Boundaries

Default examples are harness fixtures and teaching references. They are not the default implementation surface for project-level requirements.

Rules:

- Do not satisfy a project feature by modifying `examples/tiny-cli`, `examples/go-server`, or another default example unless the selected feature explicitly targets that example.
- Put project requirements in project-owned source, contract, documentation, and test paths; update `./init.sh` to verify those paths.
- Use examples as references only. If fresh project setup removes or replaces default examples, record that as setup work and update verification accordingly.
- Evaluators must reject project-level work that passes only because an example was repurposed.

## Project Recovery Init

`init.sh` is the root project recovery entry point. In hidden layout, `.agent-harness/scripts/init.sh` verifies harness health; root `./init.sh` starts as a thin harness wrapper only before a project minspec exists.

Once a minspec is accepted, Planning Agents must add a runnable-skeleton feature that turns root `./init.sh` into the project recovery contract. That contract must install or verify dependencies, start required services itself, run at least one real smoke test against an endpoint or core function, print clear logs, be idempotent, avoid manual steps and TODO placeholders, and exit non-zero on failure.

Harness verification alone must not be treated as project completion after the accepted scope requires runnable business behavior.

## External Behavior Verification

When implementation depends on behavior outside this repository's own code, agents must verify that behavior before relying on it.

Examples include:

- CLI tools and their flags, stdin/stdout/stderr behavior, exit codes, signals, working directory, environment variables, and timeout behavior.
- Third-party APIs, webhooks, SDKs, protocol payloads, callback formats, and version-specific fields.
- Runtime and platform behavior such as process management, filesystem semantics, shell behavior, permissions, networking, deployment platforms, and operating-system differences.
- Model or tool output schemas, streamed event formats, JSONL event fields, and approval or permission protocols.

Rules:

- Do not infer unknown external behavior from intuition or local mocks.
- Prefer primary sources: official help output, official documentation, real minimal commands, real sample payloads, or captured logs from the target tool.
- Treat mocks and fake children as tests of this repository's state machine only; they do not prove the external tool or platform behaves that way.
- When changing process semantics such as argv, stdio, cwd, env, timeout, signal handling, or shell mode, verify the real command behavior or document why direct verification is not possible.
- When depending on structured output fields, verify with real-shaped output from the source and add regression tests using those captured shapes.
- If behavior remains uncertain, state the uncertainty explicitly in `SPEC.md`, `progress.md`, or implementation notes, and choose the safer default.

### External Tool Schema Rules

When implementing behavior that parses output from external tools such as Codex CLI JSONL, Claude JSON, Cursor logs, deployment CLIs, test runners, or webhook payloads:

- Do not invent or assume field names from naming convention alone.
- Prefer real captured local output or official documentation as fixtures.
- Add regression tests using real-shaped output for every trusted schema field.
- If the schema is unknown, fail closed instead of extracting identifiers from assistant prose, command output, source files, documentation, or log tails.

## Work Rules

- Only one feature per Coding Agent run.
- Always keep the system runnable.
- Always run `./init.sh` before declaring success.
- The Coding Agent updates state and progress for its target feature.
- The Evaluator Agent verifies without implementation changes.
- The orchestrator is the default entrypoint for one-feature implementation and evaluation.
- Manual Coding Agent work is an explicit fallback only when adapters are unavailable or the user requests it.
- The orchestrator owns unattended feature state transitions.
- From the evaluator-evidence baseline onward, do not mark a feature done unless `runs/` contains `EVAL_PASS: Fxxx` evidence for that feature.

## Commit Message Rules

When the user explicitly asks to commit approved feature work:

- Follow `docs/commit-messages.md`.
- Include the selected feature ID first in the commit subject using the format `Fxxx <Action> <concise summary>`.
- Verify every referenced feature ID exists in `feature_list.json`.
- Use batch feature IDs only when the user explicitly approved a batch commit.
- Use `No-feature: <summary>` only for explicitly non-feature work.

## Anti-Patterns

- Doing multiple features in one Coding Agent run.
- Collapsing unrelated or independently verifiable requirements into one over-bundled feature.
- Turning vague requirements into feature entries without spec normalization.
- Relying on previous chat instead of repository files.
- Skipping `./init.sh`.
- Leaving broken code.
- Bypassing missing required capabilities with hand-written generated code, skipped verification, weakened scope, or local-only environment changes.
- Treating harness verification as project recovery after a minspec requires a runnable skeleton.
- Implementing project-level requirements inside default examples instead of project-owned source and tests.
- Treating manual Coding Agent work as the default path when orchestrator adapters should be used.
- Silently falling back from orchestrator adapter failure to hand-edited feature completion.
- Coding Agent committing during orchestrated runs.
- Committing approved feature work without the feature ID in the commit subject.
- Marking a feature done without evaluator evidence in `runs/`.
- Evaluator Agent accepting incomplete work.
- Marking a feature done without evaluator pass.

## Goal

Make the system:

- Recoverable at any time.
- Runnable at any time.
- Continuously improvable.
- Resistant to premature completion.

## Verification

`./init.sh` is the default verification entry point. It must be deterministic and fail with a non-zero exit code when the repository is not in a usable state.

Feature-level validation uses:

```bash
scripts/validate-feature.sh F001
```

Optional orchestration preview uses:

```bash
python3 orchestrator.py --dry-run
```

Default one-feature work uses:

```bash
make work
```
