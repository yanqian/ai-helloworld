# Progress

## Current System Status

The minimal AI agent harness template is runnable.

Implemented components:

- Durable agent rules in `AGENTS.md`.
- Repository knowledge map under `docs/`.
- Real-world usage notes in `docs/real-world-usage.md`.
- Evaluator rubric in `QUALITY.md`.
- Run artifact template under `runs/`.
- Failure-domain improvement loop in `docs/failure-domains.md` and `scripts/check-failure-domains.sh`.
- Practiced AGENTS guardrails for state safety, external behavior verification, tool schema handling, and anti-pattern prevention.
- Requirements in `SPEC.md`.
- Feature state in `feature_list.json`.
- Feature schema in `schemas/feature_list.schema.json`.
- Prompt templates in `prompts/`.
- Validation scripts in `scripts/`.
- Clean-state reset command in `scripts/clean-state.py` and `make clean`.
- Layered test coverage under `test/unit`, `test/contract`, and `test/smoke`.
- Contract tests that lock AI agent obligations and harness boundaries.
- Verification planning in `test_plan.md`.
- Root verification wrapper at `./init.sh`.
- Make targets in `Makefile`.
- GitHub Actions workflow in `.github/workflows/ci.yml`.
- OSS readiness files: `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, and issue templates.
- Vendor-neutral lightweight orchestrator in `orchestrator.py`.
- Explicit Coding Agent and Evaluator Agent adapters under `scripts/`.
- Distributable AI Agent Harness skill under `skills/ai-agent-harness/` with initializer, workflow guidance, and explicit finalize-and-commit rules.
- Manifest-aware skill initializer checks for new, adopt, repair, and check modes, including drift diagnostics and runnable installed-harness validation.
- README guidance that separates installing the AI Agent Harness skill, invoking the installed skill, and manually running the fallback initializer script.
- Portable Codex, Claude Code, and Cursor installation guidance for the AI Agent Harness skill without machine-specific absolute paths.
- Capability-gap governance that requires missing tools, permissions, generators, dependencies, services, credentials, runtime settings, CI resources, and verification fixtures to become durable project capabilities or tracked blocked/follow-up work.
- Example-boundary governance that keeps default examples as harness demonstrations instead of project implementation shortcuts.
- Installation layout profiles, with hidden layout as the default for user projects and visible layout retained for template maintenance.
- Feature-decomposition governance that requires broad requirements to become independently verifiable feature entries instead of over-bundled features.
- Feature-linked commit message governance that ties approved feature commits back to `feature_list.json`.
- Installer executable-bit repair that makes generated shell entrypoints executable even when source template modes are lost.
- Project recovery init governance so installed projects distinguish harness verification from the root project recovery contract.
- Minspec-to-SPEC normalization governance so planners must clarify goal, scope, flows, constraints, ambiguities, capabilities, implementation paths, and verification before appending features.
- Evaluator-evidence guardrail so features cannot be marked done without durable `EVAL_PASS: Fxxx` run evidence.
- Orchestrator-first work entrypoint so implementation and evaluation default to `make work` instead of manual state edits, with adapter-unavailable flows failing closed.
- Explicit agent-provider configuration so orchestrator adapters can use Codex, Claude Code, Cursor Agent, or a custom provider without unsafe guessing.
- New-project flow guide with a one-screen Mermaid diagram linking skill initialization, minspec planning, runnable skeleton, provider setup, orchestrator work, evaluator evidence, init verification, and commit approval.
- Tiny dependency-free Python CLI example in `examples/tiny-cli/`.
- Dependency-free Go server example in `examples/go-server/`.

## Last Completed Feature

`F032` - Document the new-project skill flow.

## Next Feature

`F011` - Explore concurrent agent execution. This is a P2 backlog item only; implement it only if the harness needs parallel agent throughput.

## Known Issues

- The template orchestrator is intentionally lightweight and vendor-neutral.
- `agent-provider.json` is intentionally absent by default; copy `agent-provider.example.json` and select an explicit provider before real `make work` execution.
- `F011` remains a P2 backlog item and should not preempt the new P0 orchestrator-first work.

## Recovery Notes

- F030 used explicit manual fallback because role adapters are still template adapters until F031.
- `make work` was verified to run the startup protocol and fail closed before mutating F030 state when adapters are unconfigured.
- F031 verified Codex command shape with `codex exec --help`; Claude Code and Cursor Agent command shapes remain intentionally unconfigured until verified on a machine that has those CLIs.
- F032 added `docs/new-project-flow.md` and README onboarding link so new users can see the full skill-assisted path at a glance.
