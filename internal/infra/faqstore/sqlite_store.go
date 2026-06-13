package faqstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

// SQLiteStore persists FAQ answer cache entries and trending counts in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore constructs a SQLite-backed FAQ store.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// GetAnswer loads a cached answer if it exists and has not expired.
func (s *SQLiteStore) GetAnswer(ctx context.Context, questionID int64) (faq.AnswerRecord, bool, error) {
	if questionID <= 0 {
		return faq.AnswerRecord{}, false, nil
	}
	var (
		record    faq.AnswerRecord
		createdAt string
		expiresAt sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT question_id, question_text, answer, created_at, expires_at
		FROM faq_answer_cache
		WHERE question_id = ?
	`, questionID).Scan(&record.QuestionID, &record.Question, &record.Answer, &createdAt, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return faq.AnswerRecord{}, false, nil
		}
		return faq.AnswerRecord{}, false, err
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return faq.AnswerRecord{}, false, err
	}
	record.CreatedAt = parsedCreated
	if expiresAt.Valid && expiresAt.String != "" {
		expiry, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil {
			return faq.AnswerRecord{}, false, err
		}
		if expiry.Before(time.Now()) {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM faq_answer_cache WHERE question_id = ?`, questionID)
			return faq.AnswerRecord{}, false, nil
		}
	}
	return record, true, nil
}

// SaveAnswer stores or replaces a cached answer with an optional TTL.
func (s *SQLiteStore) SaveAnswer(ctx context.Context, record faq.AnswerRecord, ttl time.Duration) error {
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO faq_answer_cache (question_id, question_text, answer, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(question_id) DO UPDATE SET
			question_text = excluded.question_text,
			answer = excluded.answer,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at
	`, record.QuestionID, record.Question, record.Answer, createdAt.UTC().Format(time.RFC3339Nano), expiresAt)
	return err
}

// IncrementQuery bumps a canonical query count and preserves the first display value.
func (s *SQLiteStore) IncrementQuery(ctx context.Context, canonical, display string) error {
	if canonical == "" {
		return nil
	}
	if display == "" {
		display = canonical
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO faq_trending_queries (canonical, display, count, updated_at)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(canonical) DO UPDATE SET
			count = faq_trending_queries.count + 1,
			display = CASE
				WHEN faq_trending_queries.display = '' THEN excluded.display
				ELSE faq_trending_queries.display
			END,
			updated_at = excluded.updated_at
	`, canonical, display, now)
	return err
}

// TopQueries returns trending queries ordered by count descending.
func (s *SQLiteStore) TopQueries(ctx context.Context, limit int) ([]faq.TrendingQuery, error) {
	query := `
		SELECT display, count
		FROM faq_trending_queries
		ORDER BY count DESC, display ASC
	`
	args := []any(nil)
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []faq.TrendingQuery{}
	for rows.Next() {
		var item faq.TrendingQuery
		if err := rows.Scan(&item.Query, &item.Count); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var _ faq.Store = (*SQLiteStore)(nil)
