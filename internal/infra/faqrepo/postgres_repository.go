package faqrepo

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

// PostgresRepository implements faq.QuestionRepository using pgx.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs the repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// FindExact fetches by literal question text.
func (r *PostgresRepository) FindExact(ctx context.Context, question string) (faq.QuestionRecord, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, question_text, semantic_hash
		FROM questions
		WHERE question_text = $1
		LIMIT 1
	`, question)
	if err != nil {
		return faq.QuestionRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return faq.QuestionRecord{}, false, rows.Err()
	}
	record, err := scanQuestionRecord(rows)
	if err != nil {
		return faq.QuestionRecord{}, false, err
	}
	return record, true, rows.Err()
}

// FindBySemanticHash fetches by deterministic hash.
func (r *PostgresRepository) FindBySemanticHash(ctx context.Context, hash uint64) (faq.QuestionRecord, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, question_text, semantic_hash
		FROM questions
		WHERE semantic_hash = $1
		LIMIT 1
	`, int64(hash))
	if err != nil {
		return faq.QuestionRecord{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return faq.QuestionRecord{}, false, rows.Err()
	}
	record, err := scanQuestionRecord(rows)
	if err != nil {
		return faq.QuestionRecord{}, false, err
	}
	return record, true, rows.Err()
}

// FindNearest returns the closest pgvector match.
func (r *PostgresRepository) FindNearest(ctx context.Context, embedding []float32) (faq.SimilarityMatch, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, question_text, semantic_hash, embedding <-> $1 AS distance
		FROM questions
		ORDER BY embedding <-> $1
		LIMIT 1
	`, pgvector.NewVector(embedding))
	if err != nil {
		return faq.SimilarityMatch{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return faq.SimilarityMatch{}, false, rows.Err()
	}
	var distance float64
	record, err := scanQuestionRecord(rows, &distance)
	if err != nil {
		return faq.SimilarityMatch{}, false, err
	}
	match := faq.SimilarityMatch{
		Question: record,
		Distance: distance,
	}
	return match, true, rows.Err()
}

// InsertQuestion inserts a new FAQ row.
func (r *PostgresRepository) InsertQuestion(ctx context.Context, question string, embedding []float32, hash *uint64) (faq.QuestionRecord, error) {
	var hashValue any
	if hash != nil {
		hashValue = int64(*hash)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO questions (question_text, embedding, semantic_hash)
		VALUES ($1, $2, $3)
		RETURNING id, question_text, semantic_hash
	`, question, pgvector.NewVector(embedding), hashValue)
	record, err := scanQuestionRecord(row)
	if err != nil {
		return faq.QuestionRecord{}, err
	}
	return record, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuestionRecord(row rowScanner, extras ...any) (faq.QuestionRecord, error) {
	var (
		record   faq.QuestionRecord
		semantic sql.NullInt64
	)
	args := []any{&record.ID, &record.QuestionText, &semantic}
	args = append(args, extras...)
	if err := row.Scan(args...); err != nil {
		return faq.QuestionRecord{}, err
	}
	if semantic.Valid {
		hash := uint64(semantic.Int64)
		record.SemanticHash = &hash
	}
	return record, nil
}

var _ faq.QuestionRepository = (*PostgresRepository)(nil)
