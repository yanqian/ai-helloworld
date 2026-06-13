package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database and applies shared local schema migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("sqlite path cannot be empty")
	}
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureDir(path string) error {
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			nickname TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS auth_identities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			provider_subject TEXT NOT NULL,
			provider_email TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(provider, provider_subject),
			UNIQUE(user_id, provider),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS faq_questions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_text TEXT NOT NULL UNIQUE,
			embedding TEXT NOT NULL,
			semantic_hash TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_faq_questions_semantic_hash
			ON faq_questions(semantic_hash)`,
		`CREATE TABLE IF NOT EXISTS faq_answer_cache (
			question_id INTEGER PRIMARY KEY,
			question_text TEXT NOT NULL,
			answer TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			FOREIGN KEY(question_id) REFERENCES faq_questions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS faq_trending_queries (
			canonical TEXT PRIMARY KEY,
			display TEXT NOT NULL,
			count INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS upload_documents (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			failure_reason TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_documents_user_created
			ON upload_documents(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS upload_file_objects (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			storage_key TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			etag TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(document_id) REFERENCES upload_documents(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS upload_document_chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			token_count INTEGER NOT NULL,
			embedding TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(document_id, chunk_index),
			FOREIGN KEY(document_id) REFERENCES upload_documents(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_document_chunks_document
			ON upload_document_chunks(document_id, chunk_index)`,
		`CREATE TABLE IF NOT EXISTS upload_qa_sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_qa_sessions_user_created
			ON upload_qa_sessions(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS upload_query_logs (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			query_text TEXT NOT NULL,
			response_text TEXT NOT NULL,
			latency_ms INTEGER NOT NULL,
			sources TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES upload_qa_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_query_logs_session_created
			ON upload_query_logs(session_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS upload_qa_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			token_count INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(session_id) REFERENCES upload_qa_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_qa_messages_session_created
			ON upload_qa_messages(session_id, user_id, created_at DESC, id DESC)`,
		`CREATE TABLE IF NOT EXISTS upload_qa_memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			source TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding TEXT,
			importance INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(user_id, session_id, source, content),
			FOREIGN KEY(session_id) REFERENCES upload_qa_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_qa_memories_session_user
			ON upload_qa_memories(user_id, session_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}
	return nil
}
