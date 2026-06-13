#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HTTP_ADDRESS="${HTTP_ADDRESS:-127.0.0.1:18080}"
JWT_SECRET="${JWT_SECRET:-local-smoke-secret-change-me}"
TMP_DIR="$(mktemp -d)"
SQLITE_PATH="${SQLITE_PATH:-$TMP_DIR/ai-helloworld-smoke.db}"
BIN="$TMP_DIR/ai-helloworld"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for local smoke verification" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for local smoke verification" >&2
  exit 1
fi

case "$HTTP_ADDRESS" in
  :*) BASE_URL="http://127.0.0.1${HTTP_ADDRESS}" ;;
  *) BASE_URL="http://${HTTP_ADDRESS}" ;;
esac

export GOCACHE="${GOCACHE:-/tmp/ai-helloworld-go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/ai-helloworld-go-mod}"

echo "== Build backend =="
(cd "$ROOT_DIR" && go build -o "$BIN" ./cmd/app)

echo "== Start backend =="
(
  cd "$ROOT_DIR"
  HTTP_ADDRESS="$HTTP_ADDRESS" \
    JWT_SECRET="$JWT_SECRET" \
    SQLITE_ENABLED=true \
    SQLITE_PATH="$SQLITE_PATH" \
    "$BIN"
) &
SERVER_PID="$!"

echo "== Wait for backend =="
for _ in $(seq 1 50); do
  if curl -s -o /dev/null "$BASE_URL/api/v1/auth/me"; then
    break
  fi
  sleep 0.1
done
if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
  echo "backend exited before smoke checks completed" >&2
  wait "$SERVER_PID" || true
  exit 1
fi

email="local-smoke-$(date +%s)-$$@example.com"

echo "== Register user =="
register_payload=$(jq -n --arg email "$email" '{email:$email,password:"password123",nickname:"Smoke"}')
curl -fsS -X POST "$BASE_URL/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "$register_payload" >/dev/null

echo "== Login user =="
login_payload=$(jq -n --arg email "$email" '{email:$email,password:"password123"}')
login_response=$(curl -fsS -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "$login_payload")
token=$(printf "%s" "$login_response" | jq -r '.token')
if [[ -z "$token" || "$token" == "null" ]]; then
  echo "login did not return token" >&2
  exit 1
fi

echo "== Protected profile =="
curl -fsS "$BASE_URL/api/v1/auth/me" \
  -H "Authorization: Bearer $token" \
  | jq -e '.user.email == "'"$email"'"' >/dev/null

echo "== Offline summarizer =="
summary_response=$(curl -fsS -X POST "$BASE_URL/api/v1/summaries" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $token" \
  -d '{"text":"Local smoke verifies backend startup without live LLM credentials."}')
printf "%s" "$summary_response" | jq -e '(.summary | length > 0) and (.keywords | type == "array")' >/dev/null

echo "local backend smoke passed"
