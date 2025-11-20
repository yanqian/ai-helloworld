-- Enable pgvector for similarity search on the questions table.
CREATE EXTENSION IF NOT EXISTS vector;

-- Store registered users with hashed passwords.
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    nickname      TEXT        NOT NULL CHECK (char_length(nickname) <= 10),
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Persist FAQ questions and their embeddings.
CREATE TABLE IF NOT EXISTS questions (
    id             BIGSERIAL PRIMARY KEY,
    question_text  TEXT        NOT NULL UNIQUE,
    embedding      VECTOR(1536) NOT NULL,
    semantic_hash  BIGINT UNIQUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Speed up nearest-neighbor searches on embeddings.
CREATE INDEX IF NOT EXISTS questions_embedding_idx
    ON questions
    USING ivfflat (embedding vector_l2_ops)
    WITH (lists = 100);

-- Allow fast lookups by question text and semantic hash.
CREATE INDEX IF NOT EXISTS questions_question_text_idx ON questions (question_text);
CREATE INDEX IF NOT EXISTS questions_semantic_hash_idx ON questions (semantic_hash);
