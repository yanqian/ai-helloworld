package chunker

import (
	"strings"
	"unicode/utf8"

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
	maxRunes := c.MaxTokens * 5 // conservative guard for token inflation (e.g., long base64 strings)
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	var (
		current      strings.Builder
		currentRunes int
		index        int
		out          []domain.ChunkCandidate
	)

	flush := func() {
		content := strings.TrimSpace(current.String())
		if content == "" {
			current.Reset()
			currentRunes = 0
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
		currentRunes = 0
	}

	for _, part := range parts {
		words := strings.Fields(part)
		for _, word := range words {
			wordRunes := utf8.RuneCountInString(word)

			// Split extremely long "words" (e.g., base64 strings) into manageable pieces.
			if wordRunes > maxRunes {
				chunks := splitLongWord(word, maxRunes)
				for i, chunk := range chunks {
					if currentRunes+utf8.RuneCountInString(chunk) > maxRunes {
						flush()
					}
					current.WriteString(chunk)
					current.WriteString(" ")
					currentRunes += utf8.RuneCountInString(chunk) + 1
					// Flush between split parts to avoid giant chunks.
					if i < len(chunks)-1 {
						flush()
					}
				}
				continue
			}

			if currentRunes+wordRunes > maxRunes || countTokens(current.String()+word) >= c.MaxTokens {
				flush()
				if c.Overlap > 0 && len(out) > 0 {
					overlap := tailTokens(out[len(out)-1].Content, c.Overlap)
					current.WriteString(overlap)
					currentRunes = utf8.RuneCountInString(overlap)
				}
			}
			current.WriteString(word)
			current.WriteString(" ")
			currentRunes += wordRunes + 1
		}
		current.WriteString("\n")
		currentRunes++
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

// splitLongWord slices a long token-free string into smaller pieces to avoid oversize chunks.
func splitLongWord(word string, maxRunes int) []string {
	if maxRunes <= 0 || utf8.RuneCountInString(word) <= maxRunes {
		return []string{word}
	}
	runes := []rune(word)
	var parts []string
	for i := 0; i < len(runes); i += maxRunes {
		end := i + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[i:end]))
	}
	return parts
}

var _ domain.Chunker = (*SimpleChunker)(nil)
