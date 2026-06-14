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
      "runtime_check_command": ["codex", "exec", "--ephemeral", "Reply exactly: PROVIDER_CHECK_OK"],
      "verified": "2026-06-13: codex exec --help says instructions are read from stdin when PROMPT is omitted or - is used, and documents --ephemeral for runs without persisted session files."
    }
  }
}
```

`agent-provider.json` is intentionally not committed by default because provider choice is local to a project or team. Commit it only when the team wants the same provider contract everywhere.

Rules:

- `provider` must be explicit. Do not use `auto`.
- `providers.<name>.command` must be a non-empty JSON array of strings.
- Optional `coding_command` and `evaluator_command` may override `command` for one role.
- Optional `runtime_check_command` verifies that the configured provider can actually start before the orchestrator mutates feature state.
- Optional `coding_runtime_check_command` and `evaluator_runtime_check_command` may override `runtime_check_command` for one role.
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
- A configured runtime check command fails.
- A configured runtime check command reports a permission error such as `Operation not permitted`, `Permission denied`, provider state-file access failure, or app-server access failure.
- Provider JSON is invalid.

The orchestrator preflights provider configuration before marking a feature `in_progress`, so missing provider setup does not silently mutate feature state.

When runtime preflight detects a provider permission problem, the adapter emits a machine-readable marker:

```text
PROVIDER_RUNTIME_PERMISSION_REQUIRED: provider=<name> role=<coding|evaluator> ...
```

The harness does not automatically escalate permissions. The outer agent or user must explicitly approve escalated provider runtime execution before retrying. This keeps provider access to user-level state directories, credentials, app-server sockets, or similar resources visible and intentional.

## Provider Notes

Codex:

- Verified locally on 2026-06-13 with `codex exec --help`.
- `codex exec -` reads instructions from stdin.
- `codex exec --help` documents `--ephemeral` for runs without persisted session files.
- The sample command is `["codex", "exec", "-"]`.
- The sample runtime check command is `["codex", "exec", "--ephemeral", "Reply exactly: PROVIDER_CHECK_OK"]`.
- If Codex cannot access `$CODEX_HOME` state such as `~/.codex/state_5.sqlite` or app-server resources, treat it as a provider runtime permission gap and ask the user to approve escalated execution.

Claude Code:

- Do not copy the Codex command shape.
- Verify the local Claude Code CLI help or official documentation before configuring `command` or `runtime_check_command`.
- Record the verified stdin, stdout, stderr, and exit-code behavior in `verified`.

Cursor Agent:

- Do not copy the Codex command shape.
- Verify the local Cursor Agent CLI help or official documentation before configuring `command` or `runtime_check_command`.
- Record the verified stdin, stdout, stderr, and exit-code behavior in `verified`.

Custom providers:

- Use a wrapper script when the provider needs flags, environment, credentials, or output normalization.
- The wrapper must accept the full role prompt on stdin and exit non-zero on failure.
- Add `runtime_check_command` when the provider needs user-level state, credentials, sockets, app servers, or external runtime access that may be denied by the current sandbox.
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
