# VS Code + Codex CLI Development Workflow

This document describes the end-to-end workflow I use in VS Code with Codex CLI to plan, implement, review, and ship changes. It is written to be shareable as a high-level, repeatable process.

## Goals

- Keep changes traceable from idea to merged PR.
- Use lightweight artifacts (issues, design docs) to align before coding.
- Keep reviews tight with focused commits and clear follow-up.

## Core Tools

- VS Code for editing, navigation, and quick context.
- Codex CLI for guided implementation and tooling.
- GitHub CLI (`gh`) for issues, PRs, labels, and reviews.

## Codex Skills Used

- `issue-intake`: create a main task issue plus design/implementation sub-issues.
- `design-doc`: draft and PR the feature design doc; close the design sub-issue after merge.
- `implement-from-spec`: implement code changes from the approved design; close the implementation sub-issue after merge.
- `deliver-and-fix`: drive reviews, fix CI, merge when approved; manage labels and close issues at the right time.

## Workflow Summary

1) Start with a tracked task
   - Create a main GitHub Task issue with goal, context, scope, and acceptance criteria.
   - Create two sub-issues: design doc and implementation.
   - Add a checklist in the main issue linking both sub-issues.
   - Apply `status/todo` labels for clarity.

2) Draft a design doc
   - Set the design sub-issue to `status/doing`.
   - Create a concise design doc in `docs/<feature>/design.md`.
   - Link the doc to the main issue and design sub-issue, then open a PR for early review.
   - After merge, close the design sub-issue (do not close the main task).

3) Implement from spec
   - Set the implementation sub-issue to `status/doing`.
   - Implement in a feature branch with small, scoped commits.
   - Add or update tests where appropriate.
   - After merge, close the implementation sub-issue.

4) Review and polish
   - Address review comments directly in threads.
   - Keep comments formatted with proper newlines (use `--body-file`).
   - Rebase or add follow-up commits as needed.

5) Merge and update status
   - Merge only after reviews are complete.
   - Close the main task issue only after both sub-issues are closed.

## Detailed Steps (Template)

### 1) Create a main issue with sub-issues

- Capture goal, context, scope, acceptance criteria, and test plan in the main issue.
- Create two sub-issues: design doc and implementation.
- Add a checklist in the main issue linking the sub-issues.
- Apply `status/todo` or `status/doing` as appropriate.

### 2) Write a design doc

- Set the design sub-issue to `status/doing`.
- Keep it short and clear.
- Cover: Summary, Goals, Non-Goals, Proposed Design, API/Data changes, Security, Test Plan.
- Close the design sub-issue after the doc PR is merged.

### 3) Implement

- Set the implementation sub-issue to `status/doing`.
- Use a dedicated branch, e.g., `feature/<name>`.
- Keep commits focused and readable.
- Run tests when reasonable.
- Close the implementation sub-issue after the implementation PR is merged.

### 4) Review

- Reply directly under review comments when possible.
- Use body files to avoid broken formatting in comments or PR descriptions.

### 5) Merge and close

- Merge the PR after approval.
- Close the main task issue only after both sub-issues are closed.

## Comment Formatting Tips

- Use body files for multi-line content:

```
cat <<'EOF' > /tmp/comment.md
Line 1
- Bullet
Line 3
EOF

gh pr comment <pr-number> --body-file /tmp/comment.md
```

- For PR descriptions:

```
cat <<'EOF' > /tmp/pr-body.md
Summary line

- Bullet A
- Bullet B

Refs #123
EOF

gh pr edit <pr-number> --body-file /tmp/pr-body.md
```

## Example Deliverables

- Issue with clear acceptance criteria.
- Design doc PR for early feedback.
- Implementation PR with test evidence.
- Final merge with issue marked done.

## Why This Works

- Keeps scope tight and visible.
- Lowers review friction.
- Makes progress easy to share publicly.
