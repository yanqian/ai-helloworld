package summarizer

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
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

// StreamChunk represents a streaming update.
type StreamChunk struct {
	PartialSummary string   `json:"partial_summary"`
	Completed      bool     `json:"completed"`
	Keywords       []string `json:"keywords,omitempty"`
}
