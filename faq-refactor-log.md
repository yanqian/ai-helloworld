## Smart FAQ Refactor Log

### 2024-09-27
- Reviewed `faq-spec.md` to capture the required behaviors for SearchModeExact, SearchModeSimilarity, SearchModeSemanticHash, and SearchModeHybrid. Identified gaps in the current service (Valkey-only cache, cosine similarity search, SHA-based IDs) versus the Postgres + KV workflow outlined in the spec.
- Refactored the FAQ domain to route lookups through a `QuestionRepository` (Postgres or in-memory) and a KV-backed answer store. `SearchModeHybrid` now performs exact → pgvector similarity per spec, and answers are cached using the `q:<question_id>` keyspace.
- Introduced Postgres + pgvector repository implementations, Valkey/memory cache updates, and configuration knobs (`faq.postgres.*`). Manually adjusted `cmd/app/wire_gen.go` after `wire` generation failed offline.
- Synced the React Smart FAQ mode descriptions with the new behaviors so the UI explains the four strategies accurately.
- Added build tags so the Postgres-backed repository and provider only compile when `-tags postgres` is set; default builds now fall back to the memory repository which avoids missing third-party modules in restricted environments.
- Reverted the build-tag gating per new requirements: Postgres support always compiles, and we now toggle by presence of `FAQ_POSTGRES_DSN` (or config YAML `faq.postgres.dsn`). When absent, we automatically fall back to the in-memory repo.
- Swapped to `github.com/pgvector/pgvector-go` plus the pgx pool utilities and reworked the repository scanner to use `sql.NullInt64`, removing the older pgvector/pgx import paths from our codebase.
