# Security Policy

## Supported Versions

The `main` branch is the supported version of this template.

## Reporting Security Issues

Please report security issues privately by opening a GitHub security advisory or contacting the maintainer through GitHub.

Do not include exploit details in a public issue before the maintainer has had time to investigate.

## Scope

Security-sensitive areas include:

- agent adapter scripts that execute external tools;
- orchestrator process execution and prompt dispatch;
- parsing of external CLI, model, API, or JSONL output;
- CI workflow permissions and shell commands;
- run records that may capture logs or environment details.

## Expectations For Changes

Security-related changes should include tests or documented verification evidence. If external tool behavior is involved, follow `docs/external-behavior.md`.
