#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "== Required files =="
for path in \
  AGENTS.md \
  .agent-harness-template.json \
  agent-provider.example.json \
  SPEC.md \
  feature_list.json \
  progress.md \
  test_plan.md \
  README.md \
  LICENSE \
  CONTRIBUTING.md \
  SECURITY.md \
  CHANGELOG.md \
  Makefile \
  .github/workflows/ci.yml \
  .github/ISSUE_TEMPLATE/bug_report.md \
  .github/ISSUE_TEMPLATE/feature_request.md \
  .github/ISSUE_TEMPLATE/config.yml \
  QUALITY.md \
  orchestrator.py \
  docs/README.md \
  docs/architecture.md \
  docs/new-project-flow.md \
  docs/testing.md \
  docs/external-behavior.md \
  docs/agent-provider-configuration.md \
  docs/spec-normalization.md \
  docs/feature-decomposition.md \
  docs/project-recovery-init.md \
  docs/evaluator-evidence.md \
  docs/commit-messages.md \
  docs/capability-gaps.md \
  docs/example-boundaries.md \
  docs/agent-workflow.md \
  docs/failure-domains.md \
  docs/real-world-usage.md \
  docs/decisions/README.md \
  runs/RUN_TEMPLATE.md \
  schemas/feature_list.schema.json \
  prompts/plan.md \
  prompts/work.md \
  prompts/continue.md \
  prompts/evaluate.md \
  scripts/clean-state.py \
  scripts/validate-state.py \
  scripts/validate-feature.sh \
  scripts/summarize-progress.sh \
  scripts/summarize-runs.sh \
  scripts/check-failure-domains.sh \
  scripts/run-agent-provider.py \
  scripts/run-coding-agent.sh \
  scripts/run-evaluator-agent.sh \
  skills/ai-agent-harness/SKILL.md \
  skills/ai-agent-harness/agents/openai.yaml \
  skills/ai-agent-harness/references/workflows.md \
  skills/ai-agent-harness/scripts/init_harness.py \
  examples/go-server/README.md \
  examples/go-server/go.mod \
  examples/go-server/main.go \
  examples/go-server/server.go \
  examples/go-server/server_test.go
do
  test -f "$path"
done

echo "== Harness state =="
python3 scripts/validate-state.py

echo "== Failure domains =="
scripts/check-failure-domains.sh

echo "== Evaluator evidence =="
scripts/check-evaluator-evidence.sh

echo "== Orchestrator syntax =="
python3 - <<'PY'
from pathlib import Path

compile(Path("orchestrator.py").read_text(), "orchestrator.py", "exec")
compile(Path("scripts/run-agent-provider.py").read_text(), "scripts/run-agent-provider.py", "exec")
PY

echo "== Tiny example =="
python3 - <<'PY'
from pathlib import Path

for path in [Path("examples/tiny-cli/tiny_cli.py"), Path("examples/tiny-cli/test_tiny_cli.py")]:
    compile(path.read_text(), str(path), "exec")
PY
python3 examples/tiny-cli/test_tiny_cli.py

echo "== Go server example =="
if command -v go >/dev/null 2>&1; then
  HARNESS_GOCACHE="${HARNESS_GOCACHE:-${TMPDIR:-/tmp}/ai-agent-harness-go-build}"
  mkdir -p "$HARNESS_GOCACHE"
  (cd examples/go-server && GOCACHE="$HARNESS_GOCACHE" go test ./...)
else
  echo "skip Go server example: go not found"
fi

if [[ "${HARNESS_SKIP_TEST_LAYERS:-}" == "1" ]]; then
  echo "skip layered tests: HARNESS_SKIP_TEST_LAYERS=1"
  echo "init verification passed"
  exit 0
fi

run_unittest_layer() {
  local name="$1"
  local path="$2"
  if [[ -d "$path" ]]; then
    echo "== ${name} tests =="
    python3 -m unittest discover -s "$path" -p 'test_*.py'
  else
    echo "skip ${name} tests: ${path} not present"
  fi
}

run_unittest_layer "Unit" "test/unit"
run_unittest_layer "Contract" "test/contract"
run_unittest_layer "Harness" "test/harness"
run_unittest_layer "Smoke" "test/smoke"

echo "init verification passed"
