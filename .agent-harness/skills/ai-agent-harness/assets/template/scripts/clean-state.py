#!/usr/bin/env python3
import argparse
import json
from pathlib import Path


PROGRESS_TEMPLATE = """# Progress

## Current System Status

Harness state has been reset for a new project.

## Last Completed Feature

None.

## Next Feature

Add the first feature to `feature_list.json`.

## Known Issues

- Project-specific requirements have not been added yet.
"""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Reset project-specific harness state for a fresh project.")
    parser.add_argument("--root", default=".", help="repository root to clean")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()

    feature_list = root / "feature_list.json"
    progress = root / "progress.md"
    runs_dir = root / "runs"

    feature_list.write_text(json.dumps({"features": []}, indent=2) + "\n")
    progress.write_text(PROGRESS_TEMPLATE)

    if runs_dir.exists():
        for path in runs_dir.iterdir():
            if path.name in {"RUN_TEMPLATE.md", ".gitkeep"}:
                continue
            if path.is_file():
                path.unlink()

    print(f"cleaned harness state under {root}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
