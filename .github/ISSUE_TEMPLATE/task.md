name: Task
description: Track a development task
labels: ["status/todo"]
body:
  - type: markdown
    attributes:
      value: |
        ## 🎯 Goal
        Clear, outcome-based goal.

  - type: textarea
    id: context
    attributes:
      label: Context
      description: Background, links, constraints
    validations:
      required: true

  - type: textarea
    id: scope
    attributes:
      label: Scope
      description: What is included / excluded
      placeholder: |
        Included:
        - ...

        Excluded:
        - ...

  - type: textarea
    id: acceptance
    attributes:
      label: Acceptance Criteria
      description: Codex will use this as “definition of done”
      placeholder: |
        - [ ] ...
        - [ ] ...
    validations:
      required: true

  - type: textarea
    id: commands
    attributes:
      label: Verify / Test Commands
      placeholder: |
        npm test
        npm run lint

  - type: textarea
    id: notes
    attributes:
      label: Notes / Logs
      description: Paste CI failures or link to Actions run
