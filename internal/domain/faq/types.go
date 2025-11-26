package faq

import (
	"time"

	"github.com/yanqian/ai-helloworld/pkg/metrics"
)

// SearchMode identifies the lookup strategy.
type SearchMode string

const (
	// SearchModeExact only considers literal text equality.
	SearchModeExact SearchMode = "exact"
	// SearchModeSemanticHash maps questions to a deterministic LSH bucket.
	SearchModeSemanticHash SearchMode = "semantic_hash"
	// SearchModeSimilarity uses pgvector nearest neighbour lookups.
	SearchModeSimilarity SearchMode = "similarity"
	// SearchModeHybrid tries exact before falling back to similarity.
	SearchModeHybrid SearchMode = "hybrid"
)

// Request encapsulates a FAQ search query.
type Request struct {
	Question string     `json:"question"`
	Mode     SearchMode `json:"mode"`
}

// Response is returned to the HTTP transport.
type Response struct {
	Question        string              `json:"question"`
	Answer          string              `json:"answer"`
	Source          string              `json:"source"`
	MatchedQuestion string              `json:"matchedQuestion"`
	Mode            SearchMode          `json:"mode"`
	Recommendations []TrendingQuery     `json:"recommendations"`
	DurationMs      int64               `json:"durationMs,omitempty"`
	TokenUsage      *metrics.TokenUsage `json:"tokenUsage,omitempty"`
}

// TrendingQuery represents a frequently asked question.
type TrendingQuery struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// QuestionRecord represents the Postgres question row.
type QuestionRecord struct {
	ID           int64
	QuestionText string
	SemanticHash *uint64
}

// AnswerRecord captures the payload persisted in the KV cache.
type AnswerRecord struct {
	QuestionID int64     `json:"questionId"`
	Question   string    `json:"question"`
	Answer     string    `json:"answer"`
	CreatedAt  time.Time `json:"createdAt"`
}
