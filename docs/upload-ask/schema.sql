-- Upload & Ask schema (PostgreSQL + pgvector)
-- Adjust VECTOR DIMENSION to match uploadAsk.vectorDim (default 1536).

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS upload_documents (
    id             UUID PRIMARY KEY,
    user_id        BIGINT NOT NULL,
    title          TEXT NOT NULL,
    source         TEXT NOT NULL,
    status         TEXT NOT NULL,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upload_documents_user_status
    ON upload_documents (user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS upload_file_objects (
    id          UUID PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES upload_documents(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    size_bytes  BIGINT NOT NULL,
    mime_type   TEXT NOT NULL,
    etag        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upload_file_objects_doc
    ON upload_file_objects (document_id);

CREATE TABLE IF NOT EXISTS upload_document_chunks (
    id          UUID PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES upload_documents(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content     TEXT NOT NULL,
    token_count INT NOT NULL,
    embedding   VECTOR(1536) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upload_document_chunks_doc
    ON upload_document_chunks (document_id, chunk_index);

-- Optional IVF_FLAT index for pgvector similarity (set lists > 0 for performance)
-- Adjust lists based on data size: CREATE INDEX ... USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
-- CREATE INDEX IF NOT EXISTS idx_upload_document_chunks_embedding
--     ON upload_document_chunks USING ivfflat (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS upload_qa_sessions (
    id         UUID PRIMARY KEY,
    user_id    BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upload_qa_sessions_user
    ON upload_qa_sessions (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS upload_query_logs (
    id            UUID PRIMARY KEY,
    session_id    UUID NOT NULL REFERENCES upload_qa_sessions(id) ON DELETE CASCADE,
    query_text    TEXT NOT NULL,
    response_text TEXT NOT NULL,
    latency_ms    BIGINT NOT NULL,
    sources       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upload_query_logs_session
    ON upload_query_logs (session_id, created_at DESC);
