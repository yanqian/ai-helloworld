package chunker

import (
	"strings"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// SimpleChunker splits text into roughly even sized segments.
type SimpleChunker struct {
	MaxTokens int
	Overlap   int
}

// NewSimpleChunker constructs a chunker with defaults.
func NewSimpleChunker(maxTokens, overlap int) *SimpleChunker {
	if maxTokens <= 0 {
		maxTokens = 800
	}
	if overlap < 0 {
		overlap = 0
	}
	return &SimpleChunker{MaxTokens: maxTokens, Overlap: overlap}
}

// Chunk splits by paragraphs and then by token budget.
func (c *SimpleChunker) Chunk(text string) []domain.ChunkCandidate {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	var (
		current strings.Builder
		index   int
		out     []domain.ChunkCandidate
	)

	flush := func() {
		content := strings.TrimSpace(current.String())
		if content == "" {
			current.Reset()
			return
		}
		tokenCount := countTokens(content)
		out = append(out, domain.ChunkCandidate{
			Index:      index,
			Content:    content,
			TokenCount: tokenCount,
		})
		index++
		current.Reset()
	}

	for _, part := range parts {
		words := strings.Fields(part)
		for _, word := range words {
			if countTokens(current.String()+word) >= c.MaxTokens {
				flush()
				if c.Overlap > 0 && len(out) > 0 {
					overlap := tailTokens(out[len(out)-1].Content, c.Overlap)
					current.WriteString(overlap)
				}
			}
			current.WriteString(word)
			current.WriteString(" ")
		}
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		flush()
	}
	return out
}

func countTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Fields(text))
}

func tailTokens(text string, limit int) string {
	tokens := strings.Fields(text)
	if len(tokens) <= limit {
		return text
	}
	tokens = tokens[len(tokens)-limit:]
	return strings.Join(tokens, " ") + " "
}

var _ domain.Chunker = (*SimpleChunker)(nil)
