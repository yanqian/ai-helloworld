# AI Agent Harness Template

[![CI](https://github.com/yanqian/ai-agent-harness-template/actions/workflows/ci.yml/badge.svg)](https://github.com/yanqian/ai-agent-harness-template/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Make AI coding projects resumable.

AI agents are powerful, but long-running coding work breaks when:

- the session is interrupted;
- context becomes too long;
- the weekly quota is exhausted;
- tomorrow's agent forgets yesterday's decisions;
- the agent changes unrelated files;
- the agent marks work done too early.

This is not a prompt collection. It is a repository-state protocol for making AI coding work resumable, auditable, and evaluator-gated.

This template keeps durable project state inside the repository:

- `SPEC.md` for requirements
- `feature_list.json` for executable feature state
- `progress.md` for recovery notes
- `AGENTS.md` for agent rules
- `QUALITY.md` for evaluator criteria
- `runs/` for evidence and handoff records

It is designed for Codex, Claude Code, Cursor Agent, and similar coding agents without binding the harness to one vendor.

The repository also includes a distributable [AI Agent Harness skill](skills/ai-agent-harness/SKILL.md). The skill is a convenience layer for initialization and day-to-day harness use: install or repair the harness, plan requirements, work one feature, evaluate completion, and commit approved progress. It does not replace repository state; `AGENTS.md`, `SPEC.md`, `feature_list.json`, `progress.md`, `docs/`, `QUALITY.md`, `runs/`, and git history remain the source of truth.

## Announcement

Read the project announcement: [I Built a Small Harness to Stop AI Coding Projects From Forgetting State](https://yanqian.github.io/posts/publish/i-built-a-small-harness-to-stop-ai-coding-projects-from-forgetting-state/).

## Sources

This template is informed by:

- OpenAI, [Harness engineering: leveraging Codex in an agent-first world](https://openai.com/index/harness-engineering/)
- Anthropic, [Harness design for long-running application development](https://www.anthropic.com/engineering/harness-design-long-running-apps)
- Geoff Huntley, [everything is a ralph loop](https://ghuntley.com/loop/)

## Why This Exists

AI coding breaks down less because models cannot write code, and more because long-running work loses project state.

This harness keeps the durable state in files that agents, humans, and CI can all inspect. The template dogfoods its own state model: every change is tracked as a feature with acceptance criteria, validation status, attempts, and recovery notes.

The workflow was extracted from real agent projects such as [home-guard-tg](https://github.com/yanqian/home-guard-tg) and [agent-remote-tg](https://github.com/yanqian/agent-remote-tg). See [docs/real-world-usage.md](docs/real-world-usage.md).

## Control Boundary

- `SPEC.md` records requirements.
- `feature_list.json` records executable feature state.
- `progress.md` records human-readable recovery status.
- `AGENTS.md` defines agent rules.
- `docs/` stores durable repository knowledge.
- `QUALITY.md` defines the evaluator rubric.
- `runs/` stores run evidence and handoff records.
- `docs/failure-domains.md` and `scripts/check-failure-domains.sh` turn failures into harness improvement pressure.
- `prompts/` contains role prompts.
- `scripts/` validates state and examples.
- `test/` contains unit, contract, and smoke tests.
- `orchestrator.py` previews or runs one-feature coding/evaluation loops.

## Quick Start

For the complete first-project path, see [New Project Flow](docs/new-project-flow.md). It includes a one-screen diagram of what the skill does and what humans must provide.

### Use This Template

1. Copy this template into your project root.
2. Run `make clean` to reset template state.
3. Replace `SPEC.md` with your project goal.
4. Add your first feature to `feature_list.json`.
5. Run `make init`.
6. Configure role adapters when you want unattended agent execution, then run one feature through `make work`.
7. Ask Codex, Claude Code, Cursor Agent, or another coding agent to follow `AGENTS.md` only as an explicit manual fallback when adapters are unavailable or interactive work is requested.
8. Validate one feature with `make validate FEATURE=F001`.

### Install The Skill

The skill lives in `skills/ai-agent-harness/`. Install that directory into your skill directory, then restart the agent surface if it loads skills at startup.

#### Codex

Install from this repository path:

```bash
python3 ~/.codex/skills/.system/skill-installer/scripts/install-skill-from-github.py \
  --repo yanqian/ai-agent-harness-template \
  --path skills/ai-agent-harness
```

After installation: Restart Codex to pick up the skill.

#### Claude Code

Claude Code supports Agent Skills stored as `SKILL.md` directories. Install as either a personal skill:

```bash
mkdir -p ~/.claude/skills
cp -R skills/ai-agent-harness ~/.claude/skills/ai-agent-harness
```

or as a project skill shared through git:

```bash
mkdir -p .claude/skills
cp -R skills/ai-agent-harness .claude/skills/ai-agent-harness
git add .claude/skills/ai-agent-harness
```

Restart Claude Code after installing or updating the skill.

#### Cursor

For Cursor, use project rules as the stable project-level entry point:

```bash
mkdir -p .cursor/rules
cp skills/ai-agent-harness/SKILL.md .cursor/rules/ai-agent-harness.mdc
```

Then reference the rule from Cursor Agent or ask Cursor to follow the AI Agent Harness workflow. Cursor users can still run the manual initializer script below to install or check harness files.

### Use The Installed Skill

Once installed, invoke it by name:

```text
Use $ai-agent-harness to initialize this project.
```

You can also ask naturally about installing a harness, adopting `AGENTS.md`, repairing `feature_list.json`, or checking a resumable AI coding workflow.

The skill is designed to remain vendor-neutral even when loaded by Codex as `SKILL.md`.

### Manual Script Usage

If you are using this repository checkout directly, or using another agent tool without a skill loader, call the bundled initializer script manually:

```bash
python3 skills/ai-agent-harness/scripts/init_harness.py --root /path/to/project --mode adopt
python3 skills/ai-agent-harness/scripts/init_harness.py --root /path/to/project --mode check
```

Manual `python3 skills/ai-agent-harness/scripts/init_harness.py` commands are repository-checkout or vendor-neutral fallback usage, not the primary installed-skill experience.

The initializer supports `new`, `adopt`, `repair`, and `check` modes. It does not overwrite conflicting files unless `--force` is used after explicit approval.

The initializer also supports layout profiles:

- `--layout hidden` is the default for installed projects. Root keeps thin `AGENTS.md` and `init.sh` entry points, while harness state, prompts, docs, scripts, tests, runs, schemas, and examples live under `.agent-harness/`.
- `--layout visible` is the template-maintenance layout. Harness files stay at the repository root for direct inspection and development.

In hidden layout, commit `.agent-harness/` when you want resumability across sessions and machines. Ignore only temporary caches or logs you intentionally place under paths such as `.agent-harness/tmp/`.

The initializer writes `.agent-harness/manifest.json` in installed projects and uses `.agent-harness-template.json` from the template to classify files:

- harness-owned static files are copied and drift-checked by hash;
- project-owned state such as `SPEC.md`, `feature_list.json`, and `progress.md` is validated semantically after initialization;
- merge-sensitive files such as `AGENTS.md`, `README.md`, and `Makefile` are not overwritten by default;
- optional integrations such as GitHub workflow files and examples are reported separately.

A project counts as an installed harness when `./init.sh` succeeds, the installed layout's `feature_list.json` is valid, progress and agent rules contain the required workflow sections, scripts and prompts are present, run templates are available, and skill `check` reports `runnable_harness=true`. This is stronger than checking for file existence alone.

For version drift handling, run skill `check` first. Use `repair` to restore missing files and write a manifest. Review drift before using explicit overwrite or a future upgrade flow.

After installation, distinguish harness recovery from project recovery. In hidden layout, root `./init.sh` starts as a thin wrapper around `.agent-harness/scripts/init.sh`; before a minspec exists, that only proves the harness can plan and resume. Once a minspec is accepted, the first project setup feature should turn root `./init.sh` into the project recovery contract: install dependencies, start required services, run a real endpoint or core-function smoke test, print clear logs, and fail non-zero on setup, startup, or verification failure. See [docs/project-recovery-init.md](docs/project-recovery-init.md).

The skill also defines a finalize-and-commit workflow. It stages and commits only after the user explicitly says they are satisfied or asks to commit.

Feature commits use feature-linked subjects such as `F025 Add feature-linked commit messages`. Batch commits include every approved feature ID, and explicitly non-feature commits use `No-feature: <summary>`. See `docs/commit-messages.md`.

Completed features after the evaluator-evidence baseline must also have run-level `EVAL_PASS: Fxxx` evidence. See [docs/evaluator-evidence.md](docs/evaluator-evidence.md).

### Verify This Repository

Run the same command used by CI:

```bash
make ci
```

Expected result:

- required files are present;
- `feature_list.json` is valid;
- feature IDs are unique;
- status and `passes` fields are consistent;
- the tiny example tests pass.
- the Go server example tests pass when Go is installed.
- the orchestrator can run the startup protocol and preview prompts.
- the unit, contract, and smoke layers pass.
- completed features after the evaluator-evidence baseline have matching evaluator pass evidence.

## Development Flow

1. Add new requirements to `SPEC.md`.
2. Decompose broad requirements into independently verifiable features using `docs/feature-decomposition.md`.
3. For a fresh project with an accepted minspec, add a runnable-skeleton feature using `docs/project-recovery-init.md`.
4. Append new features to `feature_list.json`.
5. Implement and evaluate one feature through `make work`.
6. Use manual Coding Agent work only as an explicit fallback when adapters are unavailable or the user requests interactive work.
7. Run `make init`.
8. Run `make validate FEATURE=Fxxx`.
9. Update `progress.md`.
10. Commit only after verification passes.

## Make Targets

- `make init` runs `./init.sh`.
- `make validate FEATURE=Fxxx` validates one feature.
- `make unit`, `make contract`, and `make smoke` run individual test layers.
- `make go-example` runs the Go server example tests.
- `make work` runs one orchestrator round for the next unfinished feature.
- `make dry-run` previews the next orchestrator round.
- `make summarize` prints progress and run summaries.
- `make clean` resets project-specific state after copying the template.
- `make ci` runs the same commands used by GitHub Actions.

## Orchestrator

The default one-feature work entrypoint is:

```bash
make work
```

`make work` runs `python3 orchestrator.py --max-rounds 1`. The orchestrator selects one unfinished feature, marks it in progress, increments attempts, dispatches Coding Agent and Evaluator Agent role prompts, and marks the feature done only after evaluator pass.

Preview the next one-feature round:

```bash
python3 orchestrator.py --dry-run
```

Run evaluator prompt preview only:

```bash
python3 orchestrator.py --eval-only F001 --dry-run
```

The template does not assume a specific AI coding tool. To execute a real agent instead of previewing prompts, replace `scripts/run-coding-agent.sh` and `scripts/run-evaluator-agent.sh` with project-specific adapters. The orchestrator sends the selected role prompt to the adapter on stdin. Orchestrator dry-run and `scripts/validate-feature.sh` are manual checks outside `./init.sh` because they run `./init.sh` as part of their own protocol.

If adapters are missing, not executable, or still the template adapters, real orchestrator work fails closed with setup guidance before silently treating manual state edits as completion. Manual Coding Agent work is an explicit fallback only when adapters are unavailable or the user asks for interactive work; record the fallback in `progress.md` or `runs/`, and do not bypass evaluator gating, evaluator evidence, attempts, failure records, or final `./init.sh` verification.

The default adapters delegate to `scripts/run-agent-provider.py`. To configure them, copy `agent-provider.example.json` to `agent-provider.json`, set an explicit provider, and verify the provider command behavior before relying on it. Codex can use the verified sample `["codex", "exec", "-"]`; Claude Code, Cursor Agent, and custom providers must be configured only after checking their local CLI help or official documentation. See [docs/agent-provider-configuration.md](docs/agent-provider-configuration.md).

The orchestrator is intentionally boring: it does not make agents smarter. It only selects one feature, enforces the startup protocol, dispatches role prompts, and records state transitions.

Adapter examples:

```bash
codex exec "$(cat)"
claude "$(cat)"
```

## Agent Prompt Files

- `prompts/plan.md` converts new requirements into spec and feature entries.
- `prompts/work.md` implements one selected feature.
- `prompts/continue.md` resumes from repository state.
- `prompts/evaluate.md` verifies one selected feature.

## Test Layers

- `test/unit/` covers deterministic helper behavior.
- `test/contract/` locks repository rules, schema, prompts, and orchestrator contracts.
- `test/harness/` is optional and intended for downstream workflow tests.
- `test/smoke/` runs the main verification commands end to end.

All default layers are run by:

```bash
make init
```

## Knowledge, Quality, And Runs

Keep `AGENTS.md` short enough to be a map. Put durable detail in `docs/`.

Use `QUALITY.md` when evaluating features. It defines correctness, completeness, maintainability, test coverage, recoverability, and safety criteria.

Use `runs/RUN_TEMPLATE.md` for non-trivial work, failures, external behavior verification, and evaluator handoff.

Failed or blocked run records must include a failure domain and harness improvement assessment. Check that with:

```bash
scripts/check-failure-domains.sh
```

Summarize run records with:

```bash
scripts/summarize-runs.sh
```

## Copying The Template

For a new project, copy the template files into the project root, then create project-specific code and tests in project-owned paths. Keep `./init.sh` as the single verification entry point. Default examples under `examples/` are harness demonstrations; remove or replace them only as explicit setup work, and do not implement project-level requirements by repurposing them.

## Tiny Example

The included example lives in `examples/tiny-cli/`. It exposes a small Python function and test suite so the harness can prove validation works without third-party dependencies.

## Go Server Example

The included Go service example lives in `examples/go-server/`. It is a dependency-free HTTP server with:

- `GET /healthz`
- `GET /greet?name=Codex`

Run it with:

```bash
cd examples/go-server
go test ./...
PORT=8080 go run .
```
