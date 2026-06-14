#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Optional

CONFIG_ENV = "HARNESS_AGENT_PROVIDER_CONFIG"
DEFAULT_CONFIG = Path("agent-provider.json")
KNOWN_PROVIDER_EXECUTABLES = {
    "codex": ["codex"],
    "claude-code": ["claude"],
    "cursor-agent": ["cursor-agent", "cursor"],
}
PERMISSION_ERROR_PATTERNS = [
    "operation not permitted",
    "permission denied",
    "not permitted",
    "state_",
    "state_5.sqlite",
    "app-server",
    "app server",
]
RUNTIME_CHECK_PROMPT = "Reply exactly: PROVIDER_CHECK_OK\n"


def fail(message: str) -> int:
    print(f"agent provider error: {message}", file=sys.stderr)
    return 2


def config_path() -> Path:
    return Path(os.environ.get(CONFIG_ENV, str(DEFAULT_CONFIG)))


def detect_candidates() -> list[str]:
    available = []
    for provider, executables in KNOWN_PROVIDER_EXECUTABLES.items():
        if any(shutil.which(executable) for executable in executables):
            available.append(provider)
    return available


def missing_config_message(path: Path) -> str:
    candidates = detect_candidates()
    if len(candidates) > 1:
        return (
            "multiple agent provider candidates detected but no provider is configured: "
            f"{', '.join(candidates)}. Copy agent-provider.example.json to {path} and set an explicit provider."
        )
    if len(candidates) == 1:
        return (
            f"agent provider candidate detected ({candidates[0]}), but provider execution must be explicit. "
            f"Copy agent-provider.example.json to {path} and set provider={candidates[0]!r}."
        )
    return f"no agent provider configured. Copy agent-provider.example.json to {path} and set an explicit provider."


def load_config(path: Path) -> dict:
    try:
        data = json.loads(path.read_text())
    except FileNotFoundError as exc:
        raise ValueError(missing_config_message(path)) from exc
    except json.JSONDecodeError as exc:
        raise ValueError(f"{path} is not valid JSON: {exc}") from exc
    if not isinstance(data, dict):
        raise ValueError(f"{path} must contain a JSON object.")
    return data


def provider_name(config: dict) -> str:
    provider = config.get("provider")
    if not isinstance(provider, str) or not provider.strip() or provider in {"auto", "unconfigured"}:
        raise ValueError("provider must be an explicit provider name; auto or unconfigured is not executable.")
    return provider


def provider_settings(config: dict) -> tuple[str, dict]:
    provider = provider_name(config)
    providers = config.get("providers")
    if not isinstance(providers, dict):
        raise ValueError("providers must be an object keyed by provider name.")

    settings = providers.get(provider)
    if not isinstance(settings, dict):
        raise ValueError(f"configured provider {provider!r} is missing from providers.")
    return provider, settings


def validate_command(command: object, provider: str, key: str) -> list[str]:
    if not isinstance(command, list) or not command or not all(isinstance(item, str) and item for item in command):
        raise ValueError(f"provider {provider!r} must define {key} as a non-empty string array.")
    executable = command[0]
    if shutil.which(executable) is None:
        raise ValueError(f"configured provider {key} is missing or not executable: {executable}")
    return command


def command_for_role(config: dict, role: str) -> list[str]:
    provider, settings = provider_settings(config)
    role_command = settings.get(f"{role}_command")
    command = role_command if role_command is not None else settings.get("command")
    key = f"{role}_command" if role_command is not None else "command"
    return validate_command(command, provider, key)


def runtime_check_command_for_role(config: dict, role: str) -> Optional[list[str]]:
    provider, settings = provider_settings(config)
    role_command = settings.get(f"{role}_runtime_check_command")
    command = role_command if role_command is not None else settings.get("runtime_check_command")
    if command is None:
        return None
    key = f"{role}_runtime_check_command" if role_command is not None else "runtime_check_command"
    return validate_command(command, provider, key)


def provider_env(config: dict) -> dict[str, str]:
    _, settings = provider_settings(config)
    env = os.environ.copy()
    extra = settings.get("env", {})
    if extra:
        if not isinstance(extra, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in extra.items()):
            raise ValueError("provider env must be an object with string keys and string values.")
        env.update(extra)
    return env


def provider_cwd(config: dict) -> str:
    _, settings = provider_settings(config)
    cwd = settings.get("cwd", ".")
    if not isinstance(cwd, str) or not cwd:
        raise ValueError("provider cwd must be a non-empty string when set.")
    return cwd


def looks_like_permission_error(output: str) -> bool:
    lowered = output.lower()
    return any(pattern in lowered for pattern in PERMISSION_ERROR_PATTERNS)


def run_runtime_check(command: Optional[list[str]], role: str, provider: str, cwd: str, env: dict[str, str]) -> None:
    if command is None:
        return
    result = subprocess.run(command, input=RUNTIME_CHECK_PROMPT, text=True, capture_output=True, cwd=cwd, env=env)
    output = "\n".join(part.strip() for part in [result.stdout, result.stderr] if part.strip())
    if result.returncode == 0:
        print(f"agent_provider_runtime_check=ok role={role} provider={provider}")
        return
    if looks_like_permission_error(output):
        raise ValueError(
            "PROVIDER_RUNTIME_PERMISSION_REQUIRED: "
            f"provider={provider} role={role} runtime check exited with code {result.returncode}. "
            "Ask the outer agent or user to approve escalated provider runtime execution before retrying. "
            f"{output[:1000]}"
        )
    raise ValueError(
        "PROVIDER_RUNTIME_CHECK_FAILED: "
        f"provider={provider} role={role} runtime check exited with code {result.returncode}. "
        f"{output[:1000] or 'no output'}"
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Dispatch an orchestrator role prompt to a configured agent provider.")
    parser.add_argument("--role", choices=["coding", "evaluator"], required=True)
    parser.add_argument("--check", action="store_true", help="validate provider configuration without running the provider")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    path = config_path()

    try:
        config = load_config(path)
        provider = provider_name(config)
        command = command_for_role(config, args.role)
        runtime_check_command = runtime_check_command_for_role(config, args.role)
        env = provider_env(config)
        cwd = provider_cwd(config)
    except ValueError as exc:
        return fail(str(exc))

    if args.check:
        print(f"agent_provider_check=ok role={args.role} command={command[0]} cwd={cwd}")
        try:
            run_runtime_check(runtime_check_command, args.role, provider, cwd, env)
        except ValueError as exc:
            return fail(str(exc))
        return 0

    prompt = sys.stdin.read()
    result = subprocess.run(command, input=prompt, text=True, cwd=cwd, env=env)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
