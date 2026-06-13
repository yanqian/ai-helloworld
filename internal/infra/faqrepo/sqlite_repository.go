package faqrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

// SQLiteRepository persists FAQ questions and embeddings in SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository constructs a SQLite-backed FAQ question repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// FindExact fetches by literal question text.
func (r *SQLiteRepository) FindExact(ctx context.Context, question string) (faq.QuestionRecord, bool, error) {
	return scanSQLiteQuestion(r.db.QueryRowContext(ctx, `
		SELECT id, question_text, semantic_hash
		FROM faq_questions
		WHERE question_text = ?
		LIMIT 1
	`, question))
}

// FindBySemanticHash fetches by deterministic semantic hash.
func (r *SQLiteRepository) FindBySemanticHash(ctx context.Context, hash uint64) (faq.QuestionRecord, bool, error) {
	if hash == 0 {
		return faq.QuestionRecord{}, false, nil
	}
	return scanSQLiteQuestion(r.db.QueryRowContext(ctx, `
		SELECT id, question_text, semantic_hash
		FROM faq_questions
		WHERE semantic_hash = ?
		ORDER BY id
		LIMIT 1
	`, strconv.FormatUint(hash, 10)))
}

// FindNearest scans stored embeddings and returns the closest local match.
func (r *SQLiteRepository) FindNearest(ctx context.Context, embedding []float32) (faq.SimilarityMatch, bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, question_text, semantic_hash, embedding
		FROM faq_questions
		ORDER BY id
	`)
	if err != nil {
		return faq.SimilarityMatch{}, false, err
	}
	defer rows.Close()

	var (
		best   faq.SimilarityMatch
		hasAny bool
	)
	for rows.Next() {
		record, stored, err := scanSQLiteQuestionWithEmbedding(rows)
		if err != nil {
			return faq.SimilarityMatch{}, false, err
		}
		dist := sqliteEuclideanDistance(embedding, stored)
		if !hasAny || dist < best.Distance {
			hasAny = true
			best = faq.SimilarityMatch{
				Question: record,
				Distance: dist,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return faq.SimilarityMatch{}, false, err
	}
	if !hasAny {
		return faq.SimilarityMatch{}, false, nil
	}
	return best, true, nil
}

// InsertQuestion inserts a new FAQ question and embedding row.
func (r *SQLiteRepository) InsertQuestion(ctx context.Context, question string, embedding []float32, hash *uint64) (faq.QuestionRecord, error) {
	payload, err := json.Marshal(embedding)
	if err != nil {
		return faq.QuestionRecord{}, err
	}
	var hashValue any
	if hash != nil {
		hashValue = strconv.FormatUint(*hash, 10)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO faq_questions (question_text, embedding, semantic_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, question, string(payload), hashValue, now)
	if err != nil {
		return faq.QuestionRecord{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return faq.QuestionRecord{}, err
	}
	record := faq.QuestionRecord{
		ID:           id,
		QuestionText: question,
	}
	if hash != nil {
		clone := *hash
		record.SemanticHash = &clone
	}
	return record, nil
}

type sqliteQuestionScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteQuestion(row sqliteQuestionScanner) (faq.QuestionRecord, bool, error) {
	var (
		record faq.QuestionRecord
		hash   sql.NullString
	)
	if err := row.Scan(&record.ID, &record.QuestionText, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return faq.QuestionRecord{}, false, nil
		}
		return faq.QuestionRecord{}, false, err
	}
	if hash.Valid && hash.String != "" {
		parsed, err := strconv.ParseUint(hash.String, 10, 64)
		if err != nil {
			return faq.QuestionRecord{}, false, err
		}
		record.SemanticHash = &parsed
	}
	return record, true, nil
}

func scanSQLiteQuestionWithEmbedding(row sqliteQuestionScanner) (faq.QuestionRecord, []float32, error) {
	var (
		record   faq.QuestionRecord
		hash     sql.NullString
		rawEmbed string
	)
	if err := row.Scan(&record.ID, &record.QuestionText, &hash, &rawEmbed); err != nil {
		return faq.QuestionRecord{}, nil, err
	}
	if hash.Valid && hash.String != "" {
		parsed, err := strconv.ParseUint(hash.String, 10, 64)
		if err != nil {
			return faq.QuestionRecord{}, nil, err
		}
		record.SemanticHash = &parsed
	}
	var embedding []float32
	if err := json.Unmarshal([]byte(rawEmbed), &embedding); err != nil {
		return faq.QuestionRecord{}, nil, err
	}
	return record, embedding, nil
}

func sqliteEuclideanDistance(a, b []float32) float64 {
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	var sum float64
	for i := 0; i < length; i++ {
		diff := float64(a[i] - b[i])
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

var _ faq.QuestionRepository = (*SQLiteRepository)(nil)
