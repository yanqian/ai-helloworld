#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: scripts/validate-feature.sh F001" >&2
  exit 2
fi

FEATURE_ID="$1"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

python3 scripts/validate-state.py

python3 - "$FEATURE_ID" <<'PY'
import json
import sys
from pathlib import Path

feature_id = sys.argv[1]
data = json.loads(Path("feature_list.json").read_text())
for feature in data["features"]:
    if feature.get("id") == feature_id:
        print(f"validating {feature_id}: {feature.get('title')}")
        print(f"status={feature.get('status')} passes={feature.get('passes')}")
        break
else:
    print(f"unknown feature id: {feature_id}", file=sys.stderr)
    raise SystemExit(1)
PY

./init.sh

echo "feature validation passed: $FEATURE_ID"

