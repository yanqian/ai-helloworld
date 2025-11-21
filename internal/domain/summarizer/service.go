package summarizer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode"

	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Service exposes summarization capabilities.
type Service interface {
	Summarize(ctx context.Context, req Request) (Response, error)
	StreamSummary(ctx context.Context, req Request) (<-chan StreamChunk, error)
}

type ChatClient interface {
	CreateChatCompletion(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.Stream, error)
}

type service struct {
	cfg    Config
	client ChatClient
	logger *slog.Logger
}

// NewService is a wire provider for the summarizer domain.
func NewService(cfg Config, client ChatClient, logger *slog.Logger) Service {
	return &service{cfg: cfg, client: client, logger: logger.With("component", "summarizer.service")}
}

func (s *service) Summarize(ctx context.Context, req Request) (Response, error) {
	text := normalize(req.Text)
	if text == "" {
		return Response{}, apperrors.Wrap("invalid_input", "text cannot be empty", nil)
	}

	messages := s.buildMessages(req, text)
	resp, err := s.client.CreateChatCompletion(ctx, chatgpt.ChatCompletionRequest{
		Model:       s.cfg.Model,
		Messages:    messages,
		Temperature: s.cfg.Temperature,
	})
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt request failed", err)
	}
	if len(resp.Choices) == 0 {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt returned no choices", nil)
	}

	content := resp.Choices[0].Message.Content
	s.logger.Debug("chatgpt response received", "content", content)

	summary, keywords, err := parseStructuredResponse(content, s.cfg.MaxKeywords)
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt response malformed", err)
	}
	summary = truncate(summary, s.cfg.MaxSummaryLen)

	return Response{
		Summary:  summary,
		Keywords: keywords,
	}, nil
}

func (s *service) StreamSummary(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	text := normalize(req.Text)
	if text == "" {
		return nil, apperrors.Wrap("invalid_input", "text cannot be empty", nil)
	}

	messages := s.buildMessages(req, text)
	stream, err := s.client.CreateChatCompletionStream(ctx, chatgpt.ChatCompletionRequest{
		Model:       s.cfg.Model,
		Messages:    messages,
		Temperature: s.cfg.Temperature,
	})
	if err != nil {
		return nil, apperrors.Wrap("llm_error", "chatgpt stream request failed", err)
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		defer stream.Close()

		var (
			builder     strings.Builder
			lastSummary string
		)

		for {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) {
					s.logger.Error("chatgpt stream recv failed", "error", recvErr)
				}
				break
			}
			for _, choice := range chunk.Choices {
				builder.WriteString(choice.Delta.Content)
			}

			partial := truncate(extractSummary(builder.String()), s.cfg.MaxSummaryLen)
			if partial == "" || partial == lastSummary {
				continue
			}
			lastSummary = partial
			out <- StreamChunk{PartialSummary: partial}
		}

		content := builder.String()
		if content == "" {
			return
		}

		s.logger.Debug("chatgpt stream response collected", "content", content)

		summary, keywords, parseErr := parseStructuredResponse(content, s.cfg.MaxKeywords)
		if parseErr != nil {
			s.logger.Error("chatgpt stream parse failed", "error", parseErr)
			return
		}

		out <- StreamChunk{
			PartialSummary: truncate(summary, s.cfg.MaxSummaryLen),
			Completed:      true,
			Keywords:       keywords,
		}
	}()

	return out, nil
}

func (s *service) buildMessages(req Request, text string) []chatgpt.Message {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = s.cfg.DefaultPrompt
	}
	userContent := fmt.Sprintf("Text:\n%s\n\nConstraints:\n- Summary must be at most %d characters.\n- Return up to %d keywords.", text, s.cfg.MaxSummaryLen, s.cfg.MaxKeywords)
	return []chatgpt.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: userContent},
	}
}

func parseStructuredResponse(content string, keywordLimit int) (string, []string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil, errors.New("empty llm response")
	}

	summaryIdx := findMarker(content, "SUMMARY:")
	if summaryIdx == -1 {
		return "", nil, errors.New("missing SUMMARY section")
	}

	body := content[summaryIdx+len("SUMMARY:"):]
	keywordsIdx := findMarker(body, "KEYWORDS:")
	var keywordsRaw string
	if keywordsIdx != -1 {
		keywordsRaw = body[keywordsIdx+len("KEYWORDS:"):]
		body = body[:keywordsIdx]
	}

	summary := strings.TrimSpace(body)
	if summary == "" {
		return "", nil, errors.New("summary section empty")
	}

	keywords := splitKeywords(keywordsRaw, keywordLimit)
	return summary, keywords, nil
}

func splitKeywords(raw string, limit int) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	raw = strings.ReplaceAll(raw, ";", ",")
	tokens := strings.Split(raw, ",")
	keywords := make([]string, 0, limit)
	for _, token := range tokens {
		clean := strings.TrimSpace(strings.TrimPrefix(token, "-"))
		if clean == "" {
			continue
		}
		keywords = append(keywords, clean)
		if limit > 0 && len(keywords) >= limit {
			break
		}
	}
	return keywords
}

func extractSummary(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	summaryIdx := findMarker(content, "SUMMARY:")
	if summaryIdx == -1 {
		return content
	}
	body := content[summaryIdx+len("SUMMARY:"):]
	if keywordsIdx := findMarker(body, "KEYWORDS:"); keywordsIdx != -1 {
		body = body[:keywordsIdx]
	}
	return strings.TrimSpace(body)
}

func findMarker(content, marker string) int {
	lowerContent := strings.ToLower(content)
	lowerMarker := strings.ToLower(marker)
	return strings.Index(lowerContent, lowerMarker)
}

func normalize(text string) string {
	text = strings.TrimSpace(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
	return text
}

func truncate(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}
