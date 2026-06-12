#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

python3 - <<'PY'
from collections import Counter
from pathlib import Path
import os
import re
import sys

runs_dir = Path(os.environ.get("HARNESS_RUNS_DIR", "runs"))
records = sorted(
    path for path in runs_dir.glob("*.md")
    if path.name != "RUN_TEMPLATE.md"
)

failure_records = []
missing_domain = []
missing_improvement = []
domains = Counter()


def field(text: str, label: str) -> str:
    match = re.search(rf"(?im)^[ \t]*-[ \t]*{re.escape(label)}[ \t]*:[ \t]*(.*?)[ \t]*$", text)
    return match.group(1).strip() if match else ""


def filled(value: str) -> bool:
    normalized = value.strip().lower()
    return bool(normalized) and normalized not in {"tbd", "todo", "unknown", "n/a", "na"}


for path in records:
    text = path.read_text()
    domain = field(text, "Failure domain")
    improvement = field(text, "Harness improvement")
    failed = "EVAL_FAIL:" in text or re.search(r"(?im)^\s*-\s*Result\s*:\s*fail", text)
    if not failed and not filled(domain):
        continue

    failure_records.append(path)
    if filled(domain):
        domains[domain] += 1
    else:
        missing_domain.append(path)
    if not filled(improvement):
        missing_improvement.append(path)

print(f"failure_records={len(failure_records)}")
if domains:
    for name, count in sorted(domains.items()):
        print(f"domain[{name}]={count}")
else:
    print("domains=none")
print(f"missing_failure_domain={len(missing_domain)}")
print(f"missing_harness_improvement={len(missing_improvement)}")

if missing_domain or missing_improvement:
    for path in missing_domain:
        print(f"missing failure domain: {path}", file=sys.stderr)
    for path in missing_improvement:
        print(f"missing harness improvement assessment: {path}", file=sys.stderr)
    raise SystemExit(1)
PY
