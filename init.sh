#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "== Harness verification =="
"$ROOT_DIR/.agent-harness/scripts/init.sh" "$@"

if [[ "${HARNESS_SKIP_TEST_LAYERS:-}" == "1" ]]; then
  echo "skip backend project recovery: HARNESS_SKIP_TEST_LAYERS=1"
  exit 0
fi

echo "== Backend toolchain =="
if ! command -v go >/dev/null 2>&1; then
  echo "go is required to verify the backend project" >&2
  exit 1
fi
go version

GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/ai-helloworld-go-build}"
GOMODCACHE="${GOMODCACHE:-${TMPDIR:-/tmp}/ai-helloworld-go-mod}"
mkdir -p "$GOCACHE" "$GOMODCACHE"

echo "== Backend tests =="
(
  cd "$ROOT_DIR"
  GOCACHE="$GOCACHE" GOMODCACHE="$GOMODCACHE" go test ./...
)

echo "backend project recovery passed"
