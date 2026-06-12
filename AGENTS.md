# AGENTS.md

This repository uses the AI Agent Harness in hidden layout.

Agents must reconstruct context from repository files and git history, not chat history.

Before planning, coding, evaluating, or resuming work:

1. Read `.agent-harness/progress.md`.
2. Read `.agent-harness/feature_list.json`.
3. Check recent work:

   ```bash
   git log --oneline -20
   ```

4. Run:

   ```bash
   ./init.sh
   ```

Full harness rules live in `.agent-harness/AGENTS.md`.
Project-specific implementation should live in project-owned source and test paths, not in `.agent-harness/` unless the selected feature explicitly changes the harness.

Root `./init.sh` starts as harness verification only. Before a minspec exists, it proves the harness can plan and resume. After minspec acceptance, plan a runnable-skeleton feature that turns root `./init.sh` into the project recovery contract described in `.agent-harness/docs/project-recovery-init.md`.

Spec Normalization rules live in `.agent-harness/docs/spec-normalization.md`. Planning must define goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface before appending feature entries.
