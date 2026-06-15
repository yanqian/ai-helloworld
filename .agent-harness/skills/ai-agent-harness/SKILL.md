---
name: ai-agent-harness
description: Use when initializing, adopting, repairing, or operating an AI Agent Harness repository; when the user mentions AGENTS.md, feature_list.json, progress.md, resumable AI coding, Planning Agent, Coding Agent, Evaluator Agent, harness workflow, or committing approved harness work. Supports project initialization, new requirement planning, one-feature implementation, evaluation, and explicit finalize-and-commit workflows while keeping repository files as the durable source of truth.
metadata:
  short-description: Initialize and operate an AI agent harness
---

# AI Agent Harness

Use this skill as the convenience layer for the repository-local AI Agent Harness protocol.

The repository remains the source of durable state. Do not store durable project state in the skill, in chat history, or in hidden local memory.

When this skill is installed, prefer natural invocation such as `Use $ai-agent-harness to initialize this project.` Codex installs skills under `~/.codex/skills`, Claude Code supports personal `~/.claude/skills/` and project `.claude/skills/` locations, and Cursor can use `.cursor/rules` as a project-level rule entry point. Manual `python3 .../init_harness.py` commands are for repository checkouts, direct script testing, or agent tools without a skill loader.

## Core Rule

Before planning, coding, evaluating, continuing, or committing harness work in a repository that already has `AGENTS.md`, follow that repository's startup protocol. For this harness, that means:

1. Read `progress.md`.
2. Read `feature_list.json`.
3. Run `git log --oneline -20`.
4. Run `./init.sh`.

If the repository does not have the harness yet, use the initializer script below.

## Initialize Or Repair

Use `scripts/init_harness.py` from this skill.

Modes:

- `new`: install the harness into a new or empty project and reset project state.
- `adopt`: install missing harness files into an existing project and reset project state; default mode.
- `repair`: add missing harness files but preserve existing `feature_list.json` and `progress.md`.
- `check`: report missing files, merge-sensitive conflicts, harness-owned drift, project state changes, installed/template versions, semantic validity, runnable status, and next action guidance without writing.

Layouts:

- `hidden`: default for installed projects; root keeps thin `AGENTS.md` and `init.sh` entry points while harness files live under `.agent-harness/`.
- `visible`: template-maintenance layout with harness files at repository root.

Default behavior never overwrites conflicting files. Use `--force` only after the user explicitly approves overwriting conflicts.

The initializer writes `.agent-harness/manifest.json` into installed projects and reads `.agent-harness-template.json` from the template. File categories matter:

- harness-owned static files are hash checked;
- project-owned state is validated semantically after initialization;
- merge-sensitive files are not overwritten by default;
- optional integrations are reported separately from core harness validity.

Examples:

```bash
python3 /path/to/skill/scripts/init_harness.py --root /path/to/project --mode adopt
python3 /path/to/skill/scripts/init_harness.py --root /path/to/project --mode check
python3 /path/to/skill/scripts/init_harness.py --root /path/to/project --mode repair
```

## Workflows

For planning, one-feature work, evaluation, continuation, and final commit rules, read the skill-local workflow reference:

- When using an installed skill, resolve `references/workflows.md` relative to this `SKILL.md` file.
- In hidden-layout repositories with an embedded recovery copy, that file is normally `.agent-harness/skills/ai-agent-harness/references/workflows.md`.

Use the workflow names as intent detectors:

- Initialize Harness
- Plan Requirement
- Work One Feature
- Evaluate Feature
- Continue Harness Work
- Finalize And Commit

When an installed harness reports a missing tool, permission, generator, dependency, service, credential, runtime setting, CI resource, or verification fixture, follow the target repository's `docs/capability-gaps.md`. Do not treat local-only workarounds as durable completion.

When planning new requirements, follow the target repository's `docs/spec-normalization.md` before feature decomposition. Normalize vague user input into explicit goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface. Do not turn vague requirements directly into feature entries.

When planning broad requirements, follow the target repository's `docs/feature-decomposition.md`. Split independently verifiable behavior, capability, implementation-boundary, risk-domain, and verification-surface work into separate features.

When a project has just been initialized or a minspec has just been accepted, follow the target repository's `docs/project-recovery-init.md`. Treat `.agent-harness/scripts/init.sh` as harness verification and root `./init.sh` as the project recovery entry point. Before a minspec exists, root `./init.sh` may only prove the harness is runnable. After minspec acceptance, plan a runnable-skeleton feature that installs dependencies, starts required services, runs a real endpoint or core-function smoke test, emits clear logs, and exits non-zero on failure.

When completing or evaluating a feature, follow the target repository's `docs/evaluator-evidence.md`. From the enforcement baseline onward, a feature should not be marked done unless `runs/` contains `EVAL_PASS: Fxxx` for that feature.

When project work touches `examples/`, follow the target repository's `docs/example-boundaries.md`. Default examples are references, not the default place for product requirements.

For one-feature implementation and evaluation, default to the repository's orchestrator-first entrypoint, normally `make work`. Manual or interactive Coding Agent work is an explicit fallback only when role adapters are unavailable or the user asks for manual work; it must not bypass evaluator gating, evaluator evidence, or final `./init.sh` verification.

When `make work` needs real agent execution, configure the target repository's provider contract from `agent-provider.example.json` using `docs/agent-provider-configuration.md`. Do not guess between Codex, Claude Code, Cursor Agent, or custom providers; missing or ambiguous provider setup is a capability gap.

## Commit Boundary

Only commit after the user explicitly says they are satisfied or asks to commit.

Before committing:

1. Run the harness startup protocol.
2. Run the relevant verification command, normally `./init.sh`.
3. Inspect `git status --short`.
4. Stage only files related to the approved work.
5. Stop and ask if unrelated changes are present.
6. Commit with a subject that follows `docs/commit-messages.md`, normally `Fxxx <Action> <concise summary>`.

Default to commit only. Push or pull request creation requires a separate explicit request.
