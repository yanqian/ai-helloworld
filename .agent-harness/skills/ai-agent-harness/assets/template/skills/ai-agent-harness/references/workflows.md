# AI Agent Harness Workflows

These workflows operate the repository-local harness. They are intentionally vendor-neutral; Codex, Claude Code, Cursor Agent, and other coding agents can follow the same file protocol.

If the skill is installed in an agent surface, invoke it by name, for example `Use $ai-agent-harness to initialize this project.` Codex uses `~/.codex/skills`, Claude Code supports `~/.claude/skills` and project `.claude/skills`, and Cursor can use `.cursor/rules` for a project-level rule. Manual `python3 skills/ai-agent-harness/scripts/init_harness.py` usage is for repository checkouts and tools without a skill loader.

## Initialize Harness

Use when the target project lacks harness files or the user asks to install, adopt, repair, or check the harness.

1. Run `scripts/init_harness.py --mode check --root <project>` when the user only wants inspection.
2. Run `scripts/init_harness.py --mode adopt --layout hidden --root <project>` for an existing project unless the user asks for visible layout.
3. Run `scripts/init_harness.py --mode new --layout hidden --root <project>` for a new project unless the user asks for visible layout.
4. Run `scripts/init_harness.py --mode repair --root <project>` for an existing harness with missing files.
5. Do not use `--force` unless the user explicitly approved overwriting conflicts.
6. After initialization, run `<project>/init.sh` if present.

`new` and `adopt` reset project feature state to an empty `feature_list.json` and fresh `progress.md`. `repair` preserves existing project state.

Installed projects record `.agent-harness/manifest.json`. The template records `.agent-harness-template.json`. Use `check` before repair or upgrade decisions: it reports installed layout, installed version, template version, missing files, merge-sensitive conflicts, harness-owned drift, project-owned state changes, semantic validity, runnable status, and next action guidance.

`hidden` layout is the default for user projects: root keeps thin `AGENTS.md` and `init.sh` entry points while the harness body lives under `.agent-harness/`. `visible` layout is for harness development and direct template inspection.

A project is an installed harness when `./init.sh` succeeds, the installed layout's `feature_list.json` is valid, `progress.md` contains recovery sections, `AGENTS.md` contains the startup and safety rules, prompts and scripts are present, run templates are available, and `check` reports `runnable_harness=true`.

In hidden layout, root `./init.sh` starts as harness verification only. Before a project minspec exists, that is enough to prove a recoverable planning environment. After minspec acceptance, use `docs/project-recovery-init.md` to plan a runnable-skeleton feature that makes root `./init.sh` install dependencies, start required services, run a real endpoint or core-function smoke test, print clear logs, avoid manual steps and TODO placeholders, and exit non-zero on failure.

From the evaluator-evidence baseline onward, `./init.sh` also verifies that completed features have matching `EVAL_PASS: Fxxx` run evidence.

## Plan Requirement

Use when the user describes new work before implementation.

1. Follow the repository startup protocol.
2. Read `AGENTS.md`, `SPEC.md`, `feature_list.json`, and `progress.md`.
3. Use `docs/spec-normalization.md` to append a normalized SPEC entry with goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface.
4. Reject vague requirements instead of turning them directly into feature entries; ask for clarification, record assumptions, or create capability, blocker, or follow-up work when needed.
5. Use `docs/feature-decomposition.md` to split broad requirements into independently verifiable feature entries.
6. Use `docs/project-recovery-init.md` when planning a fresh project, accepted minspec, runnable skeleton, or root `./init.sh` change.
7. Identify required capabilities, including tools, permissions, generators, dependencies, services, credentials, runtime settings, CI resources, and verification fixtures.
8. Identify project-owned implementation and verification paths; use `examples/` only when the requirement explicitly targets example maintenance.
9. Append new feature entries to `feature_list.json`, including explicit capability features when needed.
10. Preserve feature IDs, ordering, status, attempts, errors, and unknown fields.
11. Do not implement business logic during planning.
12. Run `./init.sh`.

## Work One Feature

Use when the user asks to implement, continue, or work on a harness feature.

1. Follow the repository startup protocol.
2. Default to orchestrator-first work:

   ```bash
   make work
   ```

3. Let the orchestrator select one unfinished feature, mark it `in_progress`, increment attempts, run Coding Agent and Evaluator Agent adapters, and mark done only after evaluator pass.
4. Configure `agent-provider.json` from `agent-provider.example.json` using `docs/agent-provider-configuration.md` before real provider execution.
5. If adapters or providers are missing, unavailable, ambiguous, or still unconfigured, treat that as a fail-closed adapter setup gap. Do not silently hand-edit feature completion.
6. Use manual or interactive Coding Agent work only as an explicit fallback when adapters are not configured or the user asks for manual work.
7. For manual fallback, select exactly one feature, implement only that feature, preserve unrelated working-tree changes, update `progress.md`, and update only the selected feature in `feature_list.json`.
8. Manual fallback must record that it was a fallback in `progress.md` or `runs/` and must not bypass evaluator pass, evaluator evidence, attempts, failure records, or final `./init.sh` verification.
9. Record a run note in `runs/` for non-trivial work, external behavior verification, failures, or evaluator handoff.
10. When a required capability is missing, follow `docs/capability-gaps.md`; make the capability durable, block the feature, or create follow-up work instead of relying on local-only workarounds.
11. When implementation touches `examples/`, follow `docs/example-boundaries.md`; do not use default examples as the product implementation surface.
12. Run `./init.sh` after changes.
13. Do not stage or commit during orchestrated Coding Agent work.

## Evaluate Feature

Use when the user asks whether a feature is complete or asks for evaluation.

1. Follow the repository startup protocol.
2. Inspect the selected feature's implementation.
3. Verify acceptance criteria in `feature_list.json`.
4. Apply `QUALITY.md`.
5. Check relevant run evidence in `runs/`.
6. Check `docs/evaluator-evidence.md` and record `EVAL_PASS: Fxxx` in `runs/` before completion.
7. Check `docs/feature-decomposition.md` and reject over-bundled features that should have been split into independently verifiable entries.
8. Check `docs/capability-gaps.md` and reject missing required capabilities that were bypassed instead of made durable or tracked.
9. Check `docs/example-boundaries.md` and reject project-level work implemented by repurposing default examples.
10. Do not implement new features.
11. Output exactly `EVAL_PASS: Fxxx` or `EVAL_FAIL: Fxxx: <reason>` when acting as an Evaluator Agent.

## Continue Harness Work

Use after interruption or when the user asks to resume.

1. Reconstruct context from repository files and git history only.
2. Read `progress.md`, `feature_list.json`, `AGENTS.md`, and recent commits.
3. Run `./init.sh`.
4. Identify the next safe action from repository state.
5. Continue implementation or evaluation through `make work` first.
6. Use manual continuation only as an explicit fallback when adapters are unavailable or the user asks for interactive/manual work.
7. Inspect `docs/capability-gaps.md` when prior work used local-only workarounds for missing capabilities.
8. Inspect `docs/example-boundaries.md` when prior work modified `examples/`.
9. Stop and report exact conflicts if state is unsafe.

## Finalize And Commit

Use only when the user explicitly says they are satisfied or asks to commit.

1. Follow the repository startup protocol.
2. Run relevant verification, normally `./init.sh`.
3. Inspect `git status --short`.
4. Identify files changed by the approved work.
5. If unrelated changes are present, ask the user before staging.
6. Stage only approved files.
7. Read `docs/commit-messages.md`.
8. Commit feature work with a subject that starts with the feature ID, normally `Fxxx <Action> <concise summary>`.
9. Verify every feature ID referenced in the commit subject exists in `feature_list.json`.
10. Use `No-feature: <summary>` only for explicitly non-feature work.
11. Default to no push. Push or pull request creation requires a separate explicit request.

Never commit merely because implementation finished. The commit boundary is user satisfaction.
