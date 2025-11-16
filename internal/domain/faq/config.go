package faq

import "time"

// Config holds runtime knobs for the FAQ service.
type Config struct {
	Model               string
	EmbeddingModel      string
	Temperature         float32
	Prompt              string
	CacheTTL            time.Duration
	TopRecommendations  int
	SimilarityThreshold float64
}
