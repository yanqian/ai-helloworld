# Real-World Usage

This template was extracted from real long-running AI coding workflows, not only from abstract prompt design.

## Projects

### home-guard-tg

Repository: <https://github.com/yanqian/home-guard-tg>

`home-guard-tg` is a local Telegram bot for checking home camera state, photos, status, alerts, and runtime logs from a Mac.

This kind of project exposed practical harness needs:

- local runtime and process behavior must be verified, not assumed;
- operational commands need clear acceptance criteria;
- logs and recovery notes matter because the system is used outside an IDE;
- agents must avoid unrelated edits when touching operational code.

### agent-remote-tg

Repository: <https://github.com/yanqian/agent-remote-tg>

`agent-remote-tg` is a Telegram-based workflow for running and supervising coding agents remotely.

It exposed the state recovery problem directly:

- remote agent work cannot rely on the current chat session staying alive;
- future sessions need to know the active feature, recent attempts, failures, and next action;
- evaluator output needs to be explicit enough for unattended or interrupted workflows;
- run evidence is necessary for handoff.

## Lessons Extracted Into This Template

- Durable state belongs in repository files, not chat history.
- Feature work needs machine-readable state and acceptance criteria.
- Agents should update only the current feature unless explicitly instructed.
- Evaluation should be a separate role that can reject premature completion.
- External behavior should be verified with real commands, official docs, logs, or real-shaped fixtures.
- Failures should improve the harness through docs, tests, prompts, scripts, or schemas.

## Current Scope

The linked projects are real-world references, not vendored examples. They are intentionally not copied into `examples/`.

The template keeps small examples under `examples/` and links real projects here to show where the workflow pressure came from.
