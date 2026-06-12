#!/usr/bin/env python3
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Optional

SKILL_DIR = Path(__file__).resolve().parents[1]
BUNDLED_TEMPLATE = SKILL_DIR / "assets" / "template"
TEMPLATE_MANIFEST = ".agent-harness-template.json"
INSTALL_MANIFEST = ".agent-harness/manifest.json"
TEMPLATE_VERSION = "0.3.3"
MODE_CHOICES = {"new", "adopt", "repair", "check"}
LAYOUT_CHOICES = {"hidden", "visible"}
DEFAULT_LAYOUT = "hidden"
EXECUTABLE_TEMPLATE_PATHS = {
    "init.sh",
    "scripts/check-evaluator-evidence.sh",
    "scripts/check-failure-domains.sh",
    "scripts/init.sh",
    "scripts/run-agent-provider.py",
    "scripts/run-coding-agent.sh",
    "scripts/run-evaluator-agent.sh",
    "scripts/summarize-progress.sh",
    "scripts/summarize-runs.sh",
    "scripts/validate-feature.sh",
}

REQUIRED_TEMPLATE_FILES = [
    TEMPLATE_MANIFEST,
    "AGENTS.md",
    "agent-provider.example.json",
    "SPEC.md",
    "feature_list.json",
    "progress.md",
    "init.sh",
    "Makefile",
    "QUALITY.md",
    "orchestrator.py",
    "scripts/init.sh",
    "scripts/run-agent-provider.py",
    "scripts/validate-state.py",
    "scripts/validate-feature.sh",
    "prompts/work.md",
    "prompts/evaluate.md",
    "docs/README.md",
    "docs/new-project-flow.md",
    "docs/agent-provider-configuration.md",
    "docs/feature-decomposition.md",
    "docs/commit-messages.md",
    "docs/capability-gaps.md",
    "docs/example-boundaries.md",
    "runs/RUN_TEMPLATE.md",
]

PROJECT_OWNED_STATE = {"SPEC.md", "feature_list.json", "progress.md"}
MERGE_SENSITIVE = {"AGENTS.md", "README.md", "Makefile"}
OPTIONAL_PREFIXES = (".github/", "examples/")
SKIP_DIRS = {".git", "__pycache__", ".pytest_cache"}

FRESH_FEATURE_LIST = {"features": []}
FRESH_PROGRESS = """# Progress

## Current System Status

Harness state has been reset for a new project.

## Last Completed Feature

None.

## Next Feature

Add the first feature to `feature_list.json`.

## Known Issues

- Project-specific requirements and verification are not defined yet.
"""


class HarnessInitError(Exception):
    pass


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Initialize or repair an AI Agent Harness project.")
    parser.add_argument("--root", default=".", help="Target project root. Defaults to the current directory.")
    parser.add_argument("--mode", choices=sorted(MODE_CHOICES), default="adopt")
    parser.add_argument(
        "--layout",
        choices=sorted(LAYOUT_CHOICES),
        help="Installation layout. Defaults to hidden for new installs, or the installed manifest layout when present.",
    )
    parser.add_argument("--template-root", help="Override the template root used for copying harness files.")
    parser.add_argument("--force", action="store_true", help="Overwrite conflicting files. Requires explicit user approval.")
    parser.add_argument("--dry-run", action="store_true", help="Report planned changes without writing files.")
    return parser.parse_args()


def find_template_root(explicit: Optional[str]) -> Path:
    candidates = []
    if explicit:
        candidates.append(Path(explicit))
    candidates.append(BUNDLED_TEMPLATE)

    # When this skill is copied inside a harness repository without bundled assets,
    # the repository root itself can serve as the template source.
    candidates.append(SKILL_DIR.parents[1])

    for candidate in candidates:
        root = candidate.resolve()
        if all((root / path).exists() for path in REQUIRED_TEMPLATE_FILES):
            return root
    checked = ", ".join(str(path.resolve()) for path in candidates)
    raise HarnessInitError(f"no usable harness template found; checked: {checked}")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def category_for(rel: Path) -> str:
    text = rel.as_posix()
    if text in PROJECT_OWNED_STATE:
        return "project-owned state"
    if text in MERGE_SENSITIVE:
        return "merge-sensitive"
    if text == TEMPLATE_MANIFEST:
        return "template manifest"
    if text.startswith(INSTALL_MANIFEST):
        return "installation manifest"
    if text.startswith(OPTIONAL_PREFIXES):
        return "optional integration"
    return "harness-owned static"


def resolve_layout(requested: Optional[str], root: Path) -> str:
    if requested:
        return requested
    installed = installed_manifest(root)
    if installed and installed.get("layout") in LAYOUT_CHOICES:
        return installed["layout"]
    if (root / "feature_list.json").exists() and (root / "scripts" / "init.sh").exists():
        return "visible"
    return DEFAULT_LAYOUT


def layout_harness_root(root: Path, layout: str) -> Path:
    return root if layout == "visible" else root / ".agent-harness"


def hidden_agents_text() -> str:
    return """# AGENTS.md

This repository uses the AI Agent Harness in hidden layout.

Agents must reconstruct context from repository files and git history, not chat history.

Before planning, coding, evaluating, or resuming work:

1. Read `.agent-harness/progress.md`.
2. Read `.agent-harness/feature_list.json`.
3. Check recent work:

   ```bash
   git log --oneline -20
   ```

4. Run:

   ```bash
   ./init.sh
   ```

Full harness rules live in `.agent-harness/AGENTS.md`.
Project-specific implementation should live in project-owned source and test paths, not in `.agent-harness/` unless the selected feature explicitly changes the harness.

Root `./init.sh` starts as harness verification only. Before a minspec exists, it proves the harness can plan and resume. After minspec acceptance, plan a runnable-skeleton feature that turns root `./init.sh` into the project recovery contract described in `.agent-harness/docs/project-recovery-init.md`.

Spec Normalization rules live in `.agent-harness/docs/spec-normalization.md`. Planning must define goal, included scope, excluded scope, core flows, constraints, ambiguities or assumptions, required capabilities, implementation paths, and verification surface before appending feature entries.
"""


def hidden_init_text() -> str:
    return """#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Pre-minspec state: verify the harness is installed and runnable.
# After minspec acceptance, project work should adapt this root entrypoint
# into the project recovery contract described in
# .agent-harness/docs/project-recovery-init.md.
exec "$ROOT_DIR/.agent-harness/scripts/init.sh" "$@"
"""


def install_items(template_root: Path, layout: str):
    for rel in iter_template_files(template_root):
        target = rel if layout == "visible" else Path(".agent-harness") / rel
        yield {
            "source": template_root / rel,
            "content": None,
            "logical": rel,
            "target": target,
            "category": category_for(rel),
        }
    if layout == "hidden":
        yield {
            "source": None,
            "content": hidden_agents_text(),
            "logical": Path("AGENTS.md"),
            "target": Path("AGENTS.md"),
            "category": "merge-sensitive",
        }
        yield {
            "source": None,
            "content": hidden_init_text(),
            "logical": Path("init.sh"),
            "target": Path("init.sh"),
            "category": "merge-sensitive",
        }


def item_hash(item: dict) -> str:
    if item["source"] is not None:
        return sha256(item["source"])
    return hashlib.sha256(item["content"].encode()).hexdigest()


def write_item(item: dict, target_root: Path, dry_run: bool) -> None:
    dst = target_root / item["target"]
    ensure_parent(dst, dry_run)
    if dry_run:
        return
    if item["source"] is not None:
        shutil.copy2(item["source"], dst)
    else:
        dst.write_text(item["content"])
    ensure_executable_mode(item, dst)


def ensure_executable_mode(item: dict, path: Path) -> None:
    if item_should_be_executable(item):
        path.chmod(path.stat().st_mode | 0o755)


def item_should_be_executable(item: dict) -> bool:
    return item["logical"].as_posix() in EXECUTABLE_TEMPLATE_PATHS


def iter_template_files(template_root: Path):
    for path in sorted(template_root.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(template_root)
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        if rel.parts and rel.parts[0] == "runs" and rel.name not in {"RUN_TEMPLATE.md", ".gitkeep"}:
            continue
        if rel.as_posix() == "manifest.json":
            continue
        if rel.parts[:3] == ("skills", "ai-agent-harness", "assets"):
            continue
        if rel.as_posix().startswith(".agent-harness/"):
            continue
        yield rel


def build_template_manifest(template_root: Path) -> dict:
    files = {}
    for rel in iter_template_files(template_root):
        files[rel.as_posix()] = {
            "category": category_for(rel),
            "sha256": sha256(template_root / rel),
        }
    return {
        "schema_version": 1,
        "template_version": TEMPLATE_VERSION,
        "default_layout": DEFAULT_LAYOUT,
        "file_categories": {
            "harness-owned static": "Copied and drift-checked by content hash.",
            "project-owned state": "Copied or reset during initialization, then validated semantically instead of byte-compared.",
            "merge-sensitive": "Never overwritten by default because the target project may already own this file.",
            "optional integration": "Copied when missing and reported when changed, but not required for core harness validity.",
            "template manifest": "Template metadata used for future drift checks.",
        },
        "files": files,
    }


def read_json(path: Path) -> Optional[dict]:
    try:
        return json.loads(path.read_text())
    except (OSError, json.JSONDecodeError):
        return None


def installed_manifest(root: Path) -> Optional[dict]:
    return read_json(root / INSTALL_MANIFEST)


def ensure_parent(path: Path, dry_run: bool) -> None:
    if dry_run:
        return
    path.parent.mkdir(parents=True, exist_ok=True)


def copy_file(src: Path, dst: Path, dry_run: bool) -> None:
    ensure_parent(dst, dry_run)
    if not dry_run:
        shutil.copy2(src, dst)


def reset_project_state(root: Path, layout: str, dry_run: bool) -> list[str]:
    changed = []
    harness_root = layout_harness_root(root, layout)
    feature_path = harness_root / "feature_list.json"
    progress_path = harness_root / "progress.md"
    if not dry_run:
        harness_root.mkdir(parents=True, exist_ok=True)
        feature_path.write_text(json.dumps(FRESH_FEATURE_LIST, indent=2) + "\n")
        progress_path.write_text(FRESH_PROGRESS)
    prefix = "" if layout == "visible" else ".agent-harness/"
    changed.extend([f"{prefix}feature_list.json", f"{prefix}progress.md"])
    return changed


def validate_feature_list(path: Path) -> tuple[bool, str]:
    data = read_json(path)
    if data is None:
        return False, "feature_list.json is missing or invalid JSON"
    features = data.get("features")
    if not isinstance(features, list):
        return False, "feature_list.json must contain a features array"
    seen = set()
    for feature in features:
        if not isinstance(feature, dict):
            return False, "each feature must be an object"
        feature_id = feature.get("id")
        if not isinstance(feature_id, str) or not feature_id:
            return False, "each feature must have an id"
        if feature_id in seen:
            return False, f"duplicate feature id: {feature_id}"
        seen.add(feature_id)
        for key in ["title", "description", "last_error", "status"]:
            if key not in feature:
                return False, f"feature {feature_id} is missing {key}"
        if not isinstance(feature.get("acceptance"), list):
            return False, f"feature {feature_id} must have acceptance list"
        if not isinstance(feature.get("passes"), bool):
            return False, f"feature {feature_id} must have boolean passes"
        if not isinstance(feature.get("attempts"), int):
            return False, f"feature {feature_id} must have integer attempts"
    return True, "ok"


def semantic_validation(root: Path, layout: str) -> dict:
    harness_root = layout_harness_root(root, layout)
    required = [
        "AGENTS.md",
        "SPEC.md",
        "feature_list.json",
        "progress.md",
        "init.sh",
        "QUALITY.md",
        "orchestrator.py",
        "agent-provider.example.json",
        "scripts/run-agent-provider.py",
        "scripts/validate-state.py",
        "scripts/check-evaluator-evidence.sh",
        "scripts/check-failure-domains.sh",
        "prompts/work.md",
        "prompts/evaluate.md",
        "docs/README.md",
        "docs/new-project-flow.md",
        "docs/agent-provider-configuration.md",
        "docs/spec-normalization.md",
        "docs/feature-decomposition.md",
        "docs/commit-messages.md",
        "docs/evaluator-evidence.md",
        "docs/capability-gaps.md",
        "docs/example-boundaries.md",
        "runs/RUN_TEMPLATE.md",
    ]
    missing = [path for path in required if not (harness_root / path).exists()]
    if layout == "hidden":
        for path in ["AGENTS.md", "init.sh"]:
            if not (root / path).exists():
                missing.append(path)
    checks = []
    if missing:
        checks.append("missing required semantic files: " + ",".join(missing))

    agents = harness_root / "AGENTS.md"
    if agents.exists():
        text = agents.read_text(errors="replace")
        for phrase in ["Required Startup Protocol", "State Safety Rules", "External Behavior Verification", "Capability Gap Handling"]:
            if phrase not in text:
                checks.append(f"AGENTS.md missing {phrase}")

    progress = harness_root / "progress.md"
    if progress.exists():
        text = progress.read_text(errors="replace")
        for phrase in ["Current System Status", "Next Feature", "Known Issues"]:
            if phrase not in text:
                checks.append(f"progress.md missing {phrase}")

    feature_ok, feature_reason = validate_feature_list(harness_root / "feature_list.json")
    if not feature_ok:
        checks.append(feature_reason)

    for script in sorted(EXECUTABLE_TEMPLATE_PATHS):
        if not (harness_root / script).exists():
            continue
        if not is_executable(harness_root / script):
            checks.append(f"{script} is not executable")
    if layout == "hidden" and (root / "init.sh").exists() and not is_executable(root / "init.sh"):
        checks.append("init.sh is not executable")

    init_ok = run_quick_init(root)
    if not init_ok:
        checks.append("HARNESS_SKIP_TEST_LAYERS=1 ./init.sh failed")

    return {
        "state_valid": "false" if checks else "true",
        "runnable_harness": "false" if checks else "true",
        "state_errors": checks,
    }


def is_executable(path: Path) -> bool:
    return bool(path.stat().st_mode & 0o111)


def run_quick_init(root: Path) -> bool:
    init = root / "init.sh"
    if not init.exists():
        return False
    try:
        result = subprocess.run(
            [str(init)],
            cwd=str(root),
            text=True,
            capture_output=True,
            env={**os.environ, "HARNESS_SKIP_TEST_LAYERS": "1"},
            timeout=20,
        )
    except (OSError, subprocess.TimeoutExpired):
        return False
    return result.returncode == 0


def classify_files(template_root: Path, target_root: Path, layout: str):
    missing = []
    unchanged = []
    conflicts = []
    drift = []
    project_state_changed = []
    optional_changed = []
    for item in install_items(template_root, layout):
        dst = target_root / item["target"]
        category = item["category"]
        if not dst.exists():
            missing.append(item)
            continue
        if item_hash(item) == sha256(dst):
            unchanged.append(item)
            continue
        if category == "project-owned state":
            project_state_changed.append(item)
        elif category == "merge-sensitive":
            conflicts.append(item)
        elif category == "optional integration":
            optional_changed.append(item)
        elif category == "template manifest":
            drift.append(item)
        else:
            drift.append(item)
    return {
        "missing": missing,
        "unchanged": unchanged,
        "conflicts": conflicts,
        "drift": drift,
        "project_state_changed": project_state_changed,
        "optional_changed": optional_changed,
    }


def write_install_manifest(root: Path, template_root: Path, mode: str, layout: str, dry_run: bool) -> None:
    if dry_run:
        return
    manifest = {
        "schema_version": 1,
        "template_version": TEMPLATE_VERSION,
        "layout": layout,
        "installed_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "mode": mode,
        "files": {},
    }
    for item in install_items(template_root, layout):
        dst = root / item["target"]
        if dst.exists():
            manifest["files"][item["target"].as_posix()] = {
                "category": item["category"],
                "sha256": sha256(dst),
            }
    path = root / INSTALL_MANIFEST
    ensure_parent(path, dry_run=False)
    path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")


def ensure_runs_gitkeep(root: Path, layout: str, dry_run: bool) -> None:
    gitkeep = layout_harness_root(root, layout) / "runs" / ".gitkeep"
    if gitkeep.exists() or dry_run:
        return
    gitkeep.parent.mkdir(parents=True, exist_ok=True)
    gitkeep.write_text("")


def should_copy_missing(mode: str, item: dict) -> bool:
    category = item["category"]
    if mode == "repair":
        return category in {
            "harness-owned static",
            "template manifest",
            "optional integration",
            "project-owned state",
            "merge-sensitive",
        }
    return True


def initialize(args: argparse.Namespace) -> int:
    target_root = Path(args.root).resolve()
    template_root = find_template_root(args.template_root)
    layout = resolve_layout(args.layout, target_root)
    target_root.mkdir(parents=True, exist_ok=True)

    classification = classify_files(template_root, target_root, layout)
    missing = classification["missing"]
    conflicts = classification["conflicts"]
    drift = classification["drift"]

    installed = installed_manifest(target_root)
    semantic = semantic_validation(target_root, layout)
    if args.mode == "check":
        print_summary(args.mode, layout, template_root, target_root, classification, semantic, installed, [], [], [], [])
        return 0 if check_is_clean(classification, semantic, installed) else 1

    created = []
    overwritten = []
    reset = []
    blocking_conflicts = [] if args.force else conflicts
    if blocking_conflicts:
        print_summary(args.mode, layout, template_root, target_root, classification, semantic, installed, [], blocking_conflicts, [], [])
        return 1

    for item in missing:
        if should_copy_missing(args.mode, item):
            write_item(item, target_root, args.dry_run)
            created.append(item)

    writable_conflicts = conflicts + drift if args.force else []
    for item in writable_conflicts:
        write_item(item, target_root, args.dry_run)
        overwritten.append(item)

    if args.mode in {"new", "adopt"}:
        reset = reset_project_state(target_root, layout, args.dry_run)
    elif args.mode == "repair":
        for rel in sorted(PROJECT_OWNED_STATE):
            prefix = "" if layout == "visible" else ".agent-harness/"
            if rel in {item["logical"].as_posix() for item in missing}:
                reset.append(f"{prefix}{rel}")

    ensure_runs_gitkeep(target_root, layout, args.dry_run)
    if not blocking_conflicts:
        write_install_manifest(target_root, template_root, args.mode, layout, args.dry_run)

    classification_after = classify_files(template_root, target_root, layout) if not args.dry_run else classification
    semantic_after = semantic_validation(target_root, layout) if not args.dry_run else semantic
    installed_after = installed_manifest(target_root) if not args.dry_run else installed
    print_summary(
        args.mode,
        layout,
        template_root,
        target_root,
        classification_after,
        semantic_after,
        installed_after,
        created,
        blocking_conflicts,
        overwritten,
        reset,
    )
    return 1 if blocking_conflicts else 0


def check_is_clean(classification: dict, semantic: dict, installed: Optional[dict]) -> bool:
    return (
        not classification["missing"]
        and not classification["conflicts"]
        and not classification["drift"]
        and semantic["state_valid"] == "true"
        and installed is not None
        and installed.get("template_version") == TEMPLATE_VERSION
        and installed.get("layout") in LAYOUT_CHOICES
    )


def next_action(classification: dict, semantic: dict, installed: Optional[dict]) -> str:
    if classification["conflicts"]:
        return "review merge-sensitive conflicts; rerun with --force only after explicit approval"
    if classification["missing"]:
        return "run repair to restore missing harness files"
    if classification["drift"]:
        return "review harness-owned drift; repair missing files or use explicit force/upgrade when appropriate"
    if installed is None:
        return "run repair to write an installation manifest"
    if installed.get("template_version") != TEMPLATE_VERSION:
        return "template version drift detected; review changes before upgrade"
    if semantic["state_valid"] != "true":
        return "fix semantic harness validation errors"
    return "harness is installed and runnable"


def display_path(item) -> str:
    if isinstance(item, Path):
        return item.as_posix()
    return item["target"].as_posix()


def print_list(name: str, paths: list) -> None:
    print(f"{name}={len(paths)}")
    if paths:
        print(f"{name}_files:")
        for item in paths:
            print(f"- {display_path(item)}")


def print_summary(
    mode: str,
    layout: str,
    template_root: Path,
    target_root: Path,
    classification: dict,
    semantic: dict,
    installed: Optional[dict],
    created: list[Path],
    blocking_conflicts: list[Path],
    overwritten: list[Path],
    reset: list[str],
) -> None:
    print(f"mode={mode}")
    print(f"layout={layout}")
    print(f"template_root={template_root}")
    print(f"target_root={target_root}")
    print(f"template_version={TEMPLATE_VERSION}")
    print(f"installed_version={installed.get('template_version') if installed else 'none'}")
    print(f"state_valid={semantic['state_valid']}")
    print(f"runnable_harness={semantic['runnable_harness']}")
    print_list("missing", classification["missing"])
    print(f"unchanged={len(classification['unchanged'])}")
    print_list("conflicts", classification["conflicts"])
    print_list("drift", classification["drift"])
    print_list("project_state_changed", classification["project_state_changed"])
    print_list("optional_changed", classification["optional_changed"])
    print_list("created", created)
    print_list("overwritten", overwritten)
    print(f"state_reset={','.join(reset) if reset else 'none'}")
    if semantic["state_errors"]:
        print("state_errors:")
        for error in semantic["state_errors"]:
            print(f"- {error}")
    if blocking_conflicts:
        print("blocking_conflicts:")
        for item in blocking_conflicts:
            print(f"- {display_path(item)}")
        print("Use --force only after explicit approval to overwrite conflicts.")
    print(f"next_action={next_action(classification, semantic, installed)}")


def main() -> int:
    try:
        return initialize(parse_args())
    except HarnessInitError as exc:
        print(f"init_harness error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
