package faq

import (
	"context"
	"time"
)

// Store defines the persistence contract for FAQ cache data.
type Store interface {
	GetAnswer(ctx context.Context, questionID int64) (AnswerRecord, bool, error)
	SaveAnswer(ctx context.Context, record AnswerRecord, ttl time.Duration) error
	IncrementQuery(ctx context.Context, canonical, display string) error
	TopQueries(ctx context.Context, limit int) ([]TrendingQuery, error)
}
