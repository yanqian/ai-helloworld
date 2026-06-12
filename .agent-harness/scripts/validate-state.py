#!/usr/bin/env python3
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FEATURES_PATH = ROOT / "feature_list.json"
SCHEMA_PATH = ROOT / "schemas" / "feature_list.schema.json"
AGENTS_PATH = ROOT / "AGENTS.md"
QUALITY_PATH = ROOT / "QUALITY.md"
DOCS_INDEX_PATH = ROOT / "docs" / "README.md"
RUN_TEMPLATE_PATH = ROOT / "runs" / "RUN_TEMPLATE.md"
FAILURE_DOMAINS_PATH = ROOT / "docs" / "failure-domains.md"
FEATURE_DECOMPOSITION_PATH = ROOT / "docs" / "feature-decomposition.md"
COMMIT_MESSAGES_PATH = ROOT / "docs" / "commit-messages.md"
VALID_STATUSES = {"todo", "in_progress", "done", "blocked"}
FEATURE_ID_RE = re.compile(r"^F[0-9]{3,}$")


def fail(message: str) -> None:
    print(f"state validation failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def load_json(path: Path):
    try:
        return json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        fail(f"{path.relative_to(ROOT)} is invalid JSON: {exc}")


def require_string(feature: dict, key: str) -> str:
    value = feature.get(key)
    if not isinstance(value, str) or not value.strip():
        fail(f"feature {feature.get('id', '<unknown>')} must have non-empty string {key}")
    return value


def main() -> int:
    if not SCHEMA_PATH.exists():
        fail("schemas/feature_list.schema.json is missing")
    validate_agents_guardrails()
    validate_knowledge_files()

    data = load_json(FEATURES_PATH)
    if not isinstance(data, dict):
        fail("feature_list.json must be a JSON object")
    features = data.get("features")
    if not isinstance(features, list):
        fail("feature_list.json must contain a features array")

    seen_ids = set()
    for index, feature in enumerate(features):
        if not isinstance(feature, dict):
            fail(f"feature at index {index} must be an object")
        feature_id = require_string(feature, "id")
        if not FEATURE_ID_RE.match(feature_id):
            fail(f"feature id must match F###: {feature_id}")
        if feature_id in seen_ids:
            fail(f"duplicate feature id: {feature_id}")
        seen_ids.add(feature_id)

        require_string(feature, "title")
        require_string(feature, "description")
        acceptance = feature.get("acceptance")
        if not isinstance(acceptance, list) or not acceptance or not all(isinstance(item, str) and item.strip() for item in acceptance):
            fail(f"feature {feature_id} must have non-empty acceptance strings")
        if not isinstance(feature.get("passes"), bool):
            fail(f"feature {feature_id} must have boolean passes")
        status = feature.get("status")
        if status not in VALID_STATUSES:
            fail(f"feature {feature_id} has invalid status: {status}")
        attempts = feature.get("attempts")
        if not isinstance(attempts, int) or attempts < 0:
            fail(f"feature {feature_id} must have non-negative integer attempts")
        if not isinstance(feature.get("last_error"), str):
            fail(f"feature {feature_id} must have string last_error")
        if feature["passes"] is True and status != "done":
            fail(f"feature {feature_id} has passes=true but status is not done")
        if status == "done" and feature["passes"] is not True:
            fail(f"feature {feature_id} has status=done but passes is not true")

    print(f"validated {len(features)} features")
    return 0


def validate_agents_guardrails() -> None:
    if not AGENTS_PATH.exists():
        fail("AGENTS.md is missing")
    text = AGENTS_PATH.read_text()
    required = [
        "### Initializer",
        "## State Safety Rules",
        "## External Behavior Verification",
        "### External Tool Schema Rules",
        "## Work Rules",
        "## Anti-Patterns",
        "Do not infer unknown external behavior from intuition or local mocks.",
        "real-shaped output",
        "Recoverable at any time.",
    ]
    for phrase in required:
        if phrase not in text:
            fail(f"AGENTS.md is missing required guardrail text: {phrase}")


def validate_knowledge_files() -> None:
    for path in [QUALITY_PATH, DOCS_INDEX_PATH, RUN_TEMPLATE_PATH, FAILURE_DOMAINS_PATH, FEATURE_DECOMPOSITION_PATH, COMMIT_MESSAGES_PATH]:
        if not path.exists():
            fail(f"{path.relative_to(ROOT)} is missing")

    quality = QUALITY_PATH.read_text()
    for phrase in ["Correctness", "Completeness", "Maintainability", "Test Coverage", "Recoverability", "Safety"]:
        if phrase not in quality:
            fail(f"QUALITY.md is missing rubric criterion: {phrase}")

    docs_index = DOCS_INDEX_PATH.read_text()
    for phrase in ["architecture.md", "testing.md", "external-behavior.md", "feature-decomposition.md", "commit-messages.md", "capability-gaps.md", "example-boundaries.md", "agent-workflow.md", "failure-domains.md", "real-world-usage.md", "decisions/"]:
        if phrase not in docs_index:
            fail(f"docs/README.md is missing index entry: {phrase}")

    decomposition = FEATURE_DECOMPOSITION_PATH.read_text()
    for phrase in ["independently verifiable", "Split Triggers", "Allowed Merges", "feature_decomposition_gap"]:
        if phrase not in decomposition:
            fail(f"docs/feature-decomposition.md is missing rule: {phrase}")

    commit_messages = COMMIT_MESSAGES_PATH.read_text()
    for phrase in ["Fxxx <Action> <concise summary>", "Put the feature ID first", "Multiple Features", "No-feature:", "Verify each referenced ID exists"]:
        if phrase not in commit_messages:
            fail(f"docs/commit-messages.md is missing rule: {phrase}")

    run_template = RUN_TEMPLATE_PATH.read_text()
    for phrase in ["Commands Run", "Evidence", "Failure Analysis", "Failure domain", "Harness improvement", "Evaluator Result", "Follow-Up"]:
        if phrase not in run_template:
            fail(f"runs/RUN_TEMPLATE.md is missing section: {phrase}")

    failure_domains = FAILURE_DOMAINS_PATH.read_text()
    for phrase in ["requirement_gap", "feature_decomposition_gap", "implementation_gap", "test_gap", "contract_gap", "external_behavior_gap", "capability_gap", "example_scope_gap", "state_recovery_gap", "agent_workflow_gap", "environment_gap", "Improvement Loop"]:
        if phrase not in failure_domains:
            fail(f"docs/failure-domains.md is missing domain or rule: {phrase}")


if __name__ == "__main__":
    raise SystemExit(main())
