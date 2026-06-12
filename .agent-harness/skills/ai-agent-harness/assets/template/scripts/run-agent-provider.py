#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

CONFIG_ENV = "HARNESS_AGENT_PROVIDER_CONFIG"
DEFAULT_CONFIG = Path("agent-provider.json")
KNOWN_PROVIDER_EXECUTABLES = {
    "codex": ["codex"],
    "claude-code": ["claude"],
    "cursor-agent": ["cursor-agent", "cursor"],
}


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


def command_for_role(config: dict, role: str) -> list[str]:
    provider = config.get("provider")
    if not isinstance(provider, str) or not provider.strip() or provider in {"auto", "unconfigured"}:
        raise ValueError("provider must be an explicit provider name; auto or unconfigured is not executable.")

    providers = config.get("providers")
    if not isinstance(providers, dict):
        raise ValueError("providers must be an object keyed by provider name.")

    settings = providers.get(provider)
    if not isinstance(settings, dict):
        raise ValueError(f"configured provider {provider!r} is missing from providers.")

    role_command = settings.get(f"{role}_command")
    command = role_command if role_command is not None else settings.get("command")
    if not isinstance(command, list) or not command or not all(isinstance(item, str) and item for item in command):
        raise ValueError(f"provider {provider!r} must define command or {role}_command as a non-empty string array.")

    executable = command[0]
    if shutil.which(executable) is None:
        raise ValueError(f"configured provider command is missing or not executable: {executable}")
    return command


def provider_env(config: dict) -> dict[str, str]:
    provider = config.get("provider")
    settings = config.get("providers", {}).get(provider, {})
    env = os.environ.copy()
    extra = settings.get("env", {})
    if extra:
        if not isinstance(extra, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in extra.items()):
            raise ValueError("provider env must be an object with string keys and string values.")
        env.update(extra)
    return env


def provider_cwd(config: dict) -> str:
    provider = config.get("provider")
    settings = config.get("providers", {}).get(provider, {})
    cwd = settings.get("cwd", ".")
    if not isinstance(cwd, str) or not cwd:
        raise ValueError("provider cwd must be a non-empty string when set.")
    return cwd


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
        command = command_for_role(config, args.role)
        env = provider_env(config)
        cwd = provider_cwd(config)
    except ValueError as exc:
        return fail(str(exc))

    if args.check:
        print(f"agent_provider_check=ok role={args.role} command={command[0]} cwd={cwd}")
        return 0

    prompt = sys.stdin.read()
    result = subprocess.run(command, input=prompt, text=True, cwd=cwd, env=env)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
