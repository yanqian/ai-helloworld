#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

python3 - <<'PY'
import json
import os
import re
import sys
from pathlib import Path

features_path = Path(os.environ.get("HARNESS_FEATURE_LIST", "feature_list.json"))
runs_dir = Path(os.environ.get("HARNESS_RUNS_DIR", "runs"))
baseline = os.environ.get("HARNESS_EVALUATOR_EVIDENCE_BASELINE", "F027")


def feature_number(feature_id: str) -> int:
    match = re.fullmatch(r"F([0-9]{3,})", feature_id)
    if not match:
        raise ValueError(f"invalid feature id: {feature_id}")
    return int(match.group(1))


try:
    baseline_number = feature_number(baseline)
except ValueError as exc:
    print(f"evaluator evidence check failed: {exc}", file=sys.stderr)
    raise SystemExit(1)

try:
    data = json.loads(features_path.read_text())
except FileNotFoundError:
    print(f"evaluator evidence check failed: missing {features_path}", file=sys.stderr)
    raise SystemExit(1)
except json.JSONDecodeError as exc:
    print(f"evaluator evidence check failed: invalid JSON in {features_path}: {exc}", file=sys.stderr)
    raise SystemExit(1)

records = []
if runs_dir.exists():
    records = [
        path for path in runs_dir.glob("*.md")
        if path.name != "RUN_TEMPLATE.md"
    ]

run_text = "\n".join(path.read_text() for path in records)
checked = []
missing = []
for feature in data.get("features", []):
    feature_id = str(feature.get("id", ""))
    try:
        number = feature_number(feature_id)
    except ValueError:
        continue
    if number < baseline_number:
        continue
    if feature.get("passes") is True and feature.get("status") == "done":
        checked.append(feature_id)
        pass_line = f"EVAL_PASS: {feature_id}"
        if pass_line not in run_text:
            missing.append(feature_id)

print(f"evaluator_evidence_baseline={baseline}")
print(f"evaluator_evidence_checked={len(checked)}")
print(f"missing_evaluator_evidence={len(missing)}")

if missing:
    for feature_id in missing:
        print(f"missing evaluator evidence: {feature_id}", file=sys.stderr)
    raise SystemExit(1)
PY
