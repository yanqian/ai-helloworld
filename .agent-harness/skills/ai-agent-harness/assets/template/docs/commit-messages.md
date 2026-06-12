# Commit Messages

Commit messages are part of the harness evidence trail. They must keep completed work traceable to `feature_list.json`.

## Subject Format

For feature work, use this subject format:

```text
Fxxx <Action> <concise summary>
```

Examples:

```text
F025 Add feature-linked commit messages
F014 Fix clean-state reset behavior
F020 Document portable skill installation
```

Rules:

- Include the exact feature ID from `feature_list.json` in the subject.
- Put the feature ID first so logs can be searched, sorted, and parsed by feature.
- Use an imperative action such as `Add`, `Implement`, `Fix`, `Document`, `Update`, or `Remove`.
- Keep the subject concise and specific.
- Prefer the feature title when it is short enough; otherwise use a concise summary that still includes the feature ID.

## Multiple Features

The default commit boundary is one approved feature. Only combine multiple feature IDs in one commit when the user explicitly approved a batch commit.

For approved batch commits, include every feature ID in the subject:

```text
F021-F024 Update manual acceptance governance
```

If the IDs are not contiguous, list them:

```text
F021,F023 Update harness acceptance rules
```

## Non-Feature Commits

Use a non-feature commit only for work that is explicitly outside `feature_list.json`, such as repository metadata or an emergency typo fix. The subject must make that clear:

```text
No-feature: update repository metadata
```

Prefer adding a feature entry when the work affects harness behavior, rules, tests, prompts, scripts, or user-visible project behavior.

## Finalize Checklist

Before committing, the agent must:

1. Identify the approved feature ID or approved batch of feature IDs.
2. Verify each referenced ID exists in `feature_list.json`.
3. Use a subject that includes the referenced ID or IDs.
4. Avoid committing unrelated changes merely to match the selected feature.
