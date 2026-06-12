#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

python3 - <<'PY'
from pathlib import Path

runs_dir = Path("runs")
records = sorted(
    path for path in runs_dir.glob("*.md")
    if path.name != "RUN_TEMPLATE.md"
)

print(f"run_records={len(records)}")
if records:
    latest = records[-1]
    print(f"latest_run={latest}")
else:
    print("latest_run=none")
PY

