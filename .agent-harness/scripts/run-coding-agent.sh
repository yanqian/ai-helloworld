#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "${HARNESS_AGENT_PROVIDER_CHECK:-}" == "1" ]]; then
  exec python3 scripts/run-agent-provider.py --role coding --check
fi

exec python3 scripts/run-agent-provider.py --role coding
