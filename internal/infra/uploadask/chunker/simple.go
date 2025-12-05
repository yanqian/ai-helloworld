package chunker

import (
	"strings"
	"unicode/utf8"

	"github.com/pkoukk/tiktoken-go"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// SimpleChunker splits text into roughly even sized segments.
type SimpleChunker struct {
	MaxTokens int
	Overlap   int
	encoder   *tiktoken.Tiktoken
}

// NewSimpleChunker constructs a chunker with defaults.
func NewSimpleChunker(maxTokens, overlap int) *SimpleChunker {
	if maxTokens <= 0 {
		maxTokens = 800
	}
	if overlap < 0 {
		overlap = 0
	}
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		enc = nil
	}
	return &SimpleChunker{MaxTokens: maxTokens, Overlap: overlap, encoder: enc}
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
		tokenCount := c.countTokens(content)
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

			if currentRunes+wordRunes > maxRunes || c.countTokens(current.String()+word) >= c.MaxTokens {
				flush()
				if c.Overlap > 0 && len(out) > 0 {
					overlap := c.tailTokens(out[len(out)-1].Content, c.Overlap)
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

func (c *SimpleChunker) countTokens(text string) int {
	if text == "" {
		return 0
	}
	if c.encoder != nil {
		ids := c.encoder.Encode(text, nil, nil)
		return len(ids)
	}
	return len(strings.Fields(text))
}

func (c *SimpleChunker) tailTokens(text string, limit int) string {
	if limit <= 0 || text == "" {
		return ""
	}
	if c.encoder != nil {
		ids := c.encoder.Encode(text, nil, nil)
		if len(ids) <= limit {
			return text + " "
		}
		tail := ids[len(ids)-limit:]
		decoded := c.encoder.Decode(tail)
		return decoded + " "
	}
	words := strings.Fields(text)
	if len(words) <= limit {
		return text + " "
	}
	words = words[len(words)-limit:]
	return strings.Join(words, " ") + " "
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
