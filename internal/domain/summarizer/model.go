package summarizer

import "github.com/yanqian/ai-helloworld/pkg/metrics"

// Config configures the summarizer heuristics.
type Config struct {
	MaxSummaryLen int
	MaxKeywords   int
	DefaultPrompt string
	Model         string
	Temperature   float32
}

// Request represents the incoming summarization payload.
type Request struct {
	Text   string `json:"text"`
	Prompt string `json:"prompt,omitempty"`
}

// Response is returned by the sync endpoint.
type Response struct {
	Summary    string              `json:"summary"`
	Keywords   []string            `json:"keywords"`
	DurationMs int64               `json:"durationMs,omitempty"`
	TokenUsage *metrics.TokenUsage `json:"tokenUsage,omitempty"`
}

// StreamChunk represents a streaming update.
type StreamChunk struct {
	PartialSummary string   `json:"partial_summary"`
	Completed      bool     `json:"completed"`
	Keywords       []string `json:"keywords,omitempty"`
}
