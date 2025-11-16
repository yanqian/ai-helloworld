package faq

import (
	"strings"
	"unicode"
)

func normalizeQuestion(q string) string {
	lowered := strings.ToLower(strings.TrimSpace(q))
	var builder strings.Builder
	builder.Grow(len(lowered))
	lastSpace := true
	for _, r := range lowered {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		// treat punctuation as space
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	normalized := strings.TrimSpace(builder.String())
	return strings.Join(strings.Fields(normalized), " ")
}
