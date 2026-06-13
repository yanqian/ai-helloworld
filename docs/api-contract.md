# Shared API Contract

The sibling frontend repository is `/Users/armstrong/Project/ai-helloworld-fe`.

This document records backend JSON fields that are consumed by the React frontend and should not be renamed without updating both repositories in the same change.

## Shared Surfaces

- Auth: `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/me`, `/api/v1/auth/logout`, `/api/v1/auth/google/login`, `/api/v1/auth/google/callback`.
- Summarizer: `/api/v1/summaries`, `/api/v1/summaries/stream`.
- UV advisor: `/api/v1/uv-advice`.
- Smart FAQ: `/api/v1/faq/search`, `/api/v1/faq/trending`.
- Upload & Ask: `/api/v1/upload-ask/documents`, `/api/v1/upload-ask/documents/:id`, `/api/v1/upload-ask/qa/query`, `/api/v1/upload-ask/qa/sessions`, `/api/v1/upload-ask/qa/sessions/:id/logs`.

## Contract Fields

- `refreshToken`: returned by auth login/refresh and accepted by auth refresh. The frontend stores it as `authRefreshToken` and sends `{ "refreshToken": "..." }`.
- `durationMs`: optional request duration used by summarizer, UV advisor, Smart FAQ, and Upload & Ask request stats.
- `tokenUsage`: optional LLM usage object with `promptTokens`, optional `completionTokens`, and `totalTokens`.
- `sessionId`: Upload & Ask ask responses and query logs use this field to preserve selected chat sessions.
- `partial_summary`: summarizer SSE frames use snake case. The frontend parser accepts this backend field and normalizes it to `partialSummary` in TypeScript.
- `documentId`, `chunkIndex`, `score`, `preview`: Upload & Ask citation fields rendered by the frontend.
- `failureReason`: optional Upload & Ask document status detail shown when processing fails.

## Drift Guard

Backend `TestFrontendContractJSONFields` reflects over the Go response structs and fails if the contract-sensitive JSON tags above are renamed.

Router-level smoke coverage also verifies protected route registration, structured errors, and Upload & Ask local response shapes without live LLM, OAuth, R2, Postgres, pgvector, Valkey, Redis, or GCP credentials. Run `./init.sh` from the backend repository to execute the drift guard and the full recovery check.

The frontend should keep a matching contract note in `/Users/armstrong/Project/ai-helloworld-fe/docs/api-contract.md` and type or fixture coverage for these fields.
