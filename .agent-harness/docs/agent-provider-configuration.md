# Agent Provider Configuration

The orchestrator is the default work entrypoint, but it remains vendor-neutral. Real agent execution requires an explicit provider configuration.

## Contract

Copy `agent-provider.example.json` to `agent-provider.json` and set one provider:

```json
{
  "provider": "codex",
  "providers": {
    "codex": {
      "command": ["codex", "exec", "-"],
      "verified": "2026-06-05: codex exec --help says instructions are read from stdin when PROMPT is omitted or - is used."
    }
  }
}
```

`agent-provider.json` is intentionally not committed by default because provider choice is local to a project or team. Commit it only when the team wants the same provider contract everywhere.

Rules:

- `provider` must be explicit. Do not use `auto`.
- `providers.<name>.command` must be a non-empty JSON array of strings.
- Optional `coding_command` and `evaluator_command` may override `command` for one role.
- Optional `cwd` sets the provider working directory.
- Optional `env` adds string environment variables.
- Commands run without a shell; shell snippets, pipes, and implicit expansion are not part of the contract.

## Failure Behavior

The adapters fail closed when:

- `agent-provider.json` is missing.
- Multiple known provider CLIs are detected but no provider is selected.
- The configured provider is missing from `providers`.
- The configured command is empty.
- The configured executable is not available.
- Provider JSON is invalid.

The orchestrator preflights provider configuration before marking a feature `in_progress`, so missing provider setup does not silently mutate feature state.

## Provider Notes

Codex:

- Verified locally on 2026-06-05 with `codex exec --help`.
- `codex exec -` reads instructions from stdin.
- The sample command is `["codex", "exec", "-"]`.

Claude Code:

- Do not copy the Codex command shape.
- Verify the local Claude Code CLI help or official documentation before configuring `command`.
- Record the verified stdin, stdout, stderr, and exit-code behavior in `verified`.

Cursor Agent:

- Do not copy the Codex command shape.
- Verify the local Cursor Agent CLI help or official documentation before configuring `command`.
- Record the verified stdin, stdout, stderr, and exit-code behavior in `verified`.

Custom providers:

- Use a wrapper script when the provider needs flags, environment, credentials, or output normalization.
- The wrapper must accept the full role prompt on stdin and exit non-zero on failure.
- If coding output is normalized, preserve the final `CODING_PASS: Fxxx` or `CODING_FAIL: Fxxx: <reason>` line when present.
- If evaluator output is normalized, preserve the final `EVAL_PASS: Fxxx` or `EVAL_FAIL: Fxxx: <reason>` line.
- The orchestrator uses the last matching role verdict for a feature so historical run evidence echoed in provider output cannot override the final role decision.
- If a provider exits non-zero after a final `CODING_PASS` or `EVAL_PASS`, the orchestrator treats the final structured verdict as authoritative and logs the provider exit-code contradiction.

## Commands

Preflight the configured provider:

```bash
HARNESS_AGENT_PROVIDER_CHECK=1 scripts/run-coding-agent.sh
HARNESS_AGENT_PROVIDER_CHECK=1 scripts/run-evaluator-agent.sh
```

Run one orchestrator round:

```bash
make work
```

Preview without requiring a provider:

```bash
make dry-run
```
