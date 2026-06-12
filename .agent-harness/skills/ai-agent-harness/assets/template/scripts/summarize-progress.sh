#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

python3 - <<'PY'
import json
from pathlib import Path

data = json.loads(Path("feature_list.json").read_text())
features = data.get("features", [])
done = [f for f in features if f.get("passes") is True and f.get("status") == "done"]
unfinished = [f for f in features if not (f.get("passes") is True and f.get("status") == "done")]

print(f"features_total={len(features)}")
print(f"features_done={len(done)}")
print(f"features_unfinished={len(unfinished)}")
if done:
    last = done[-1]
    print(f"last_completed={last.get('id')} {last.get('title')}")
if unfinished:
    next_feature = unfinished[0]
    print(f"next_feature={next_feature.get('id')} {next_feature.get('title')}")
else:
    print("next_feature=none")
PY

