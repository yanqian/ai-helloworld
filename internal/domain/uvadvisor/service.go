package uvadvisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Service exposes UV based recommendation capabilities.
type Service interface {
	Recommend(ctx context.Context, req Request) (Response, error)
}

type chatClient interface {
	CreateChatCompletion(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.ChatCompletionResponse, error)
}

type uvClient interface {
	Fetch(ctx context.Context, date string) (UVSeries, error)
}

type service struct {
	cfg      Config
	client   chatClient
	uvClient uvClient
	logger   *slog.Logger
	timezone *time.Location
	now      func() time.Time
}

// NewService wires up the UV advisor domain.
func NewService(cfg Config, uvClient uvClient, client chatClient, logger *slog.Logger) Service {
	return &service{
		cfg:      cfg,
		client:   client,
		uvClient: uvClient,
		logger:   logger.With("component", "uvadvisor.service"),
		timezone: time.FixedZone("Asia/Singapore", 8*60*60),
		now:      time.Now,
	}
}

func (s *service) Recommend(ctx context.Context, req Request) (Response, error) {
	date, err := s.resolveDate(req.Date)
	if err != nil {
		return Response{}, apperrors.Wrap("invalid_input", "date must be formatted as YYYY-MM-DD", err)
	}

	messages := []chatgpt.Message{
		{Role: "system", Content: s.buildSystemPrompt()},
		{Role: "user", Content: fmt.Sprintf("Create outfit and protection advice for outdoor plans roughly on %s Singapore time. Always call the get_sg_uv tool first before replying.", date)},
	}

	toolDef := s.toolDefinition()

	first, err := s.client.CreateChatCompletion(ctx, chatgpt.ChatCompletionRequest{
		Model:       s.cfg.Model,
		Messages:    messages,
		Temperature: s.cfg.Temperature,
		Tools:       []chatgpt.Tool{toolDef},
	})
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt request failed", err)
	}
	if len(first.Choices) == 0 {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt returned no choices", nil)
	}

	initialMsg := first.Choices[0].Message
	if len(initialMsg.ToolCalls) == 0 {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt did not call the UV tool", nil)
	}

	call := initialMsg.ToolCalls[0]
	toolArgs, err := s.parseToolArgs(call.Function.Arguments)
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "invalid tool arguments", err)
	}
	s.logger.Info("uv advisor tool call issued", "function", call.Function.Name, "arguments", call.Function.Arguments)

	fetchDate := date
	if toolArgs.Time != "" {
		if parsed, parseErr := s.parseSingaporeDate(toolArgs.Time); parseErr == nil {
			fetchDate = parsed
		}
	}

	series, err := s.uvClient.Fetch(ctx, fetchDate)
	if err != nil {
		return Response{}, apperrors.Wrap("uv_data_error", "failed to fetch UV data", err)
	}
	if len(series.Readings) == 0 {
		return Response{}, apperrors.Wrap("uv_data_error", "no UV readings available for the selected date", nil)
	}
	s.logger.Info("uv advisor uv data fetched", "date", fetchDate, "readings", len(series.Readings))

	messages = append(messages, initialMsg)
	messages = append(messages, chatgpt.Message{
		Role:       "tool",
		Content:    string(series.RawJSON),
		ToolCallID: call.ID,
	})

	second, err := s.client.CreateChatCompletion(ctx, chatgpt.ChatCompletionRequest{
		Model:       s.cfg.Model,
		Messages:    messages,
		Temperature: s.cfg.Temperature,
		Tools:       []chatgpt.Tool{toolDef},
	})
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt follow-up request failed", err)
	}
	if len(second.Choices) == 0 {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt follow-up returned no choices", nil)
	}
	finalMsg := second.Choices[0].Message
	if len(finalMsg.ToolCalls) > 0 {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt requested unexpected additional tool calls", nil)
	}
	s.logger.Info("uv advisor final response", "content", finalMsg.Content)

	stats := summarize(series.Readings)
	readings := toResponseReadings(series.Readings)
	advice, err := parseAIAdvice(finalMsg.Content)
	if err != nil {
		return Response{}, apperrors.Wrap("llm_error", "chatgpt response malformed", err)
	}

	dataTimestamp := ""
	if !series.UpdatedAt.IsZero() {
		dataTimestamp = series.UpdatedAt.Format(time.RFC3339)
	}
	res := Response{
		Date:          firstNonEmpty(series.Date, fetchDate),
		Category:      stats.Category,
		MaxUV:         stats.Max,
		PeakHour:      stats.PeakHour.Format(time.RFC3339),
		Source:        firstNonEmpty(series.Source, s.cfg.SourceURL),
		Summary:       advice.Summary,
		Clothing:      normalizeList(advice.Clothing),
		Protection:    normalizeList(advice.Protection),
		Tips:          normalizeList(advice.Tips),
		Readings:      readings,
		DataTimestamp: dataTimestamp,
	}
	s.logger.Info("uv advisor final returns", "content", res)

	return res, nil
}

func (s *service) resolveDate(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return s.now().In(s.timezone).Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

type seriesStats struct {
	Max      float64
	Category string
	PeakHour time.Time
}

func summarize(points []UVSample) seriesStats {
	if len(points) == 0 {
		return seriesStats{}
	}

	maxVal := -1.0
	var peak time.Time
	for _, pt := range points {
		if pt.Value > maxVal || (pt.Value == maxVal && pt.Hour.Before(peak)) {
			maxVal = pt.Value
			peak = pt.Hour
		}
	}
	return seriesStats{
		Max:      maxVal,
		Category: categoryFor(maxVal),
		PeakHour: peak,
	}
}

func categoryFor(uv float64) string {
	switch {
	case uv < 3:
		return "low"
	case uv < 6:
		return "moderate"
	case uv < 8:
		return "high"
	case uv < 11:
		return "very_high"
	default:
		return "extreme"
	}
}

func toResponseReadings(points []UVSample) []Reading {
	readings := make([]Reading, 0, len(points))
	for _, pt := range points {
		readings = append(readings, Reading{
			Hour:  pt.Hour.Format(time.RFC3339),
			Value: math.Round(pt.Value*10) / 10,
		})
	}
	return readings
}

type aiAdvice struct {
	Summary    string
	Clothing   []string
	Protection []string
	Tips       []string
}

func parseAIAdvice(raw string) (aiAdvice, error) {
	sanitized := strings.TrimSpace(raw)
	sanitized = strings.TrimPrefix(sanitized, "```json")
	sanitized = strings.TrimSuffix(sanitized, "```")
	sanitized = strings.Trim(sanitized, "`")
	sanitized = strings.TrimSpace(strings.TrimPrefix(sanitized, "json"))

	wire, err := decodeAdviceWire([]byte(sanitized))
	if err != nil {
		return aiAdvice{}, err
	}
	advice := aiAdvice{
		Summary:    wire.Summary,
		Clothing:   normalizeList(wire.Clothing),
		Protection: normalizeList(wire.Protection),
		Tips:       normalizeList(wire.Tips),
	}
	if advice.Summary == "" {
		return aiAdvice{}, errors.New("summary missing")
	}
	if len(advice.Clothing) == 0 && len(advice.Protection) == 0 {
		return aiAdvice{}, errors.New("recommendations missing")
	}

	return advice, nil
}

func normalizeList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *service) buildSystemPrompt() string {
	base := strings.TrimSpace(s.cfg.Prompt)
	if base == "" {
		base = "You are a UV protection stylist for Singapore."
	}
	enforcer := " Respond ONLY with valid minified JSON using this shape: {\"summary\":string,\"clothing\":string[],\"protection\":string[],\"tips\":string[]}. Arrays must contain short actionable strings; if none apply, respond with an empty array. Never return plain text or other fields."
	return base + enforcer
}

func (s *service) toolDefinition() chatgpt.Tool {
	return chatgpt.Tool{
		Type: "function",
		Function: chatgpt.ToolFunction{
			Name:        "get_sg_uv",
			Description: "Get current ultraviolet (UV) index in Singapore from the official data.gov.sg API and return raw JSON.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"time": map[string]any{
						"type":        "string",
						"description": "ISO datetime in Singapore time; if omitted, use now.",
					},
				},
			},
		},
	}
}

type toolArgs struct {
	Time string `json:"time"`
}

func (s *service) parseToolArgs(raw string) (toolArgs, error) {
	var args toolArgs
	if strings.TrimSpace(raw) == "" {
		return args, nil
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return toolArgs{}, err
	}
	return args, nil
}

func (s *service) parseSingaporeDate(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("empty time")
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", err
	}
	return ts.In(s.timezone).Format("2006-01-02"), nil
}

type adviceWire struct {
	Summary    string
	Clothing   []string
	Protection []string
	Tips       []string
}

func decodeAdviceWire(data []byte) (adviceWire, error) {
	var raw struct {
		Summary    string          `json:"summary"`
		Clothing   json.RawMessage `json:"clothing"`
		Protection json.RawMessage `json:"protection"`
		Tips       json.RawMessage `json:"tips"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return adviceWire{}, err
	}

	clothing, err := coerceStringArray(raw.Clothing)
	if err != nil {
		return adviceWire{}, err
	}
	protection, err := coerceStringArray(raw.Protection)
	if err != nil {
		return adviceWire{}, err
	}
	tips, err := coerceStringArray(raw.Tips)
	if err != nil {
		return adviceWire{}, err
	}

	return adviceWire{
		Summary:    raw.Summary,
		Clothing:   clothing,
		Protection: protection,
		Tips:       tips,
	}, nil
}

func coerceStringArray(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	switch raw[0] {
	case '"':
		var single string
		if err := json.Unmarshal(raw, &single); err != nil {
			return nil, err
		}
		if strings.TrimSpace(single) == "" {
			return nil, nil
		}
		return []string{single}, nil
	case '[':
		var many []string
		if err := json.Unmarshal(raw, &many); err != nil {
			return nil, err
		}
		return many, nil
	default:
		return nil, errors.New("unsupported advice array format")
	}
}
