# Contributing

Thanks for helping improve AI Agent Harness Template.

This project is a repository-level harness for long-running AI coding work. Contributions should preserve the core boundary: durable state belongs in repository files, and verification should be runnable by agents, humans, and CI.

## Development Workflow

1. Read `AGENTS.md`, `SPEC.md`, `feature_list.json`, and `progress.md`.
2. Add or update a feature entry in `feature_list.json` when changing behavior.
3. Keep changes scoped to one feature where possible.
4. Run:

   ```bash
   make ci
   ```

5. Open a pull request with the feature ID, verification commands, and any remaining risks.

## Contribution Areas

- Better examples for real project types.
- Stronger contract tests for agent behavior.
- Safer adapter patterns for Codex, Claude Code, Cursor Agent, and other tools.
- Better documentation for recovery, evaluation, and failure-domain handling.

## Design Rules

- Do not rely on chat history for durable project state.
- Do not weaken evaluator gating.
- Do not make the orchestrator vendor-specific.
- Prefer small, testable changes over broad rewrites.
