package faq

import "context"

// SimilarityMatch contains the best pgvector match and its distance.
type SimilarityMatch struct {
	Question QuestionRecord
	Distance float64
}

// QuestionRepository encapsulates Postgres operations for questions.
type QuestionRepository interface {
	FindExact(ctx context.Context, question string) (QuestionRecord, bool, error)
	FindBySemanticHash(ctx context.Context, hash uint64) (QuestionRecord, bool, error)
	FindNearest(ctx context.Context, embedding []float32) (SimilarityMatch, bool, error)
	InsertQuestion(ctx context.Context, question string, embedding []float32, hash *uint64) (QuestionRecord, error)
}
