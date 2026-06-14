package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenFreshFAQSchemaUsesQuestionsTable(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fresh.db"))
	require.NoError(t, err)
	defer db.Close()

	require.True(t, testTableExists(t, ctx, db, "questions"))
	require.False(t, testTableExists(t, ctx, db, "faq_questions"))
	require.True(t, testColumnExists(t, ctx, db, "questions", "created_at"))
	require.Equal(t, "questions", testForeignKeyTable(t, ctx, db, "faq_answer_cache", "question_id"))
}

func TestOpenMigratesLegacyQuestionsCreatedAt(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-questions.db")
	raw := testRawDB(t, path)
	_, err := raw.ExecContext(ctx, `
		CREATE TABLE questions (
			id INTEGER PRIMARY KEY,
			question_text TEXT NOT NULL,
			embedding TEXT NOT NULL,
			semantic_hash INTEGER
		)
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		INSERT INTO questions (id, question_text, embedding, semantic_hash)
		VALUES (1, 'legacy question', '[1,0,0]', 99)
	`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	require.True(t, testColumnExists(t, ctx, db, "questions", "created_at"))
	require.False(t, testTableExists(t, ctx, db, "faq_questions"))
	var question string
	var createdAt string
	err = db.QueryRowContext(ctx, `SELECT question_text, created_at FROM questions WHERE id = 1`).Scan(&question, &createdAt)
	require.NoError(t, err)
	require.Equal(t, "legacy question", question)
	require.NotEmpty(t, createdAt)
}

func TestOpenMigratesFAQQuestionsAndCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "faq-questions.db")
	raw := testRawDB(t, path)
	_, err := raw.ExecContext(ctx, `
		CREATE TABLE faq_questions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			question_text TEXT NOT NULL UNIQUE,
			embedding TEXT NOT NULL,
			semantic_hash TEXT,
			created_at TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		CREATE TABLE faq_answer_cache (
			question_id INTEGER PRIMARY KEY,
			question_text TEXT NOT NULL,
			answer TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			FOREIGN KEY(question_id) REFERENCES faq_questions(id) ON DELETE CASCADE
		)
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		INSERT INTO faq_questions (id, question_text, embedding, semantic_hash, created_at)
		VALUES (7, 'migrated question', '[0,1,0]', '123', '2026-06-14T00:00:00Z')
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		INSERT INTO faq_answer_cache (question_id, question_text, answer, created_at, expires_at)
		VALUES (7, 'migrated question', 'migrated answer', '2026-06-14T00:00:01Z', NULL)
	`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	require.True(t, testTableExists(t, ctx, db, "questions"))
	require.False(t, testTableExists(t, ctx, db, "faq_questions"))
	require.Equal(t, "questions", testForeignKeyTable(t, ctx, db, "faq_answer_cache", "question_id"))

	var question string
	var createdAt string
	err = db.QueryRowContext(ctx, `SELECT question_text, created_at FROM questions WHERE id = 7`).Scan(&question, &createdAt)
	require.NoError(t, err)
	require.Equal(t, "migrated question", question)
	require.Equal(t, "2026-06-14T00:00:00Z", createdAt)

	var answer string
	err = db.QueryRowContext(ctx, `SELECT answer FROM faq_answer_cache WHERE question_id = 7`).Scan(&answer)
	require.NoError(t, err)
	require.Equal(t, "migrated answer", answer)
}

func TestOpenDoesNotOverwriteExistingQuestionsFromFAQQuestions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "duplicate-questions.db")
	raw := testRawDB(t, path)
	_, err := raw.ExecContext(ctx, `
		CREATE TABLE questions (
			id INTEGER PRIMARY KEY,
			question_text TEXT NOT NULL,
			embedding TEXT NOT NULL,
			semantic_hash TEXT,
			created_at TEXT
		)
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		INSERT INTO questions (id, question_text, embedding, semantic_hash, created_at)
		VALUES (1, 'same question', '[1,0,0]', 'original', '2026-06-14T00:00:00Z')
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		CREATE TABLE faq_questions (
			id INTEGER PRIMARY KEY,
			question_text TEXT NOT NULL,
			embedding TEXT NOT NULL,
			semantic_hash TEXT,
			created_at TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `
		INSERT INTO faq_questions (id, question_text, embedding, semantic_hash, created_at)
		VALUES (2, 'same question', '[0,1,0]', 'incoming', '2026-06-14T00:00:01Z')
	`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	require.False(t, testTableExists(t, ctx, db, "faq_questions"))
	var count int
	var semanticHash string
	err = db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(semantic_hash) FROM questions WHERE question_text = 'same question'`).Scan(&count, &semanticHash)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "original", semanticHash)
}

func testRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	return db
}

func testTableExists(t *testing.T, ctx context.Context, db *sql.DB, name string) bool {
	t.Helper()
	exists, err := tableExists(ctx, db, name)
	require.NoError(t, err)
	return exists
}

func testColumnExists(t *testing.T, ctx context.Context, db *sql.DB, table string, column string) bool {
	t.Helper()
	exists, err := columnExists(ctx, db, table, column)
	require.NoError(t, err)
	return exists
}

func testForeignKeyTable(t *testing.T, ctx context.Context, db *sql.DB, table string, fromColumn string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_list("+table+")")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			from     string
			to       string
			onUpdate string
			onDelete string
			match    string
		)
		require.NoError(t, rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match))
		if from == fromColumn {
			return refTable
		}
	}
	require.NoError(t, rows.Err())
	t.Fatalf("foreign key for %s.%s not found", table, fromColumn)
	return ""
}
