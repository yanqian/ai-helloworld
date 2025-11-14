package uvadvisor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/infra/llm/chatgpt"
)

func TestServiceRecommendSuccess(t *testing.T) {
	uvSeries := UVSeries{
		Date: "2024-07-01",
		Readings: []UVSample{
			{Hour: mustParse("2024-07-01T10:00:00+08:00"), Value: 3},
			{Hour: mustParse("2024-07-01T12:00:00+08:00"), Value: 8},
		},
		UpdatedAt: mustParse("2024-07-01T19:00:00+08:00"),
		Source:    "https://example.com",
		RawJSON:   []byte(`{"code":0,"data":{"records":[]}}`),
	}

	chatStub := &stubChatClient{
		responses: []chatgpt.ChatCompletionResponse{
			{
				Choices: []struct {
					Message chatgpt.Message "json:\"message\""
				}{
					{Message: chatgpt.Message{
						Role:    "assistant",
						Content: `{"summary":"Sunny day","clothing":["Light linen"],"protection":["SPF 50"],"tips":["Stay hydrated"]}`,
					}},
				},
			},
		},
	}

	uvStub := &stubUVClient{series: uvSeries}

	svc := &service{
		cfg: Config{
			Model:       "gpt-test",
			Temperature: 0.1,
			Prompt:      "Return JSON",
		},
		client:   chatStub,
		uvClient: uvStub,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time {
			return mustParse("2024-07-01T09:00:00+08:00")
		},
		timezone: time.FixedZone("Asia/Singapore", 8*60*60),
	}

	resp, err := svc.Recommend(context.Background(), Request{})
	require.NoError(t, err)
	require.Equal(t, "2024-07-01", resp.Date)
	require.Equal(t, "very_high", resp.Category)
	require.Equal(t, 8.0, resp.MaxUV)
	require.Equal(t, uvSeries.Source, resp.Source)
	require.Equal(t, "Sunny day", resp.Summary)
	require.Equal(t, []string{"Light linen"}, resp.Clothing)
	require.Equal(t, []string{"SPF 50"}, resp.Protection)
	require.Equal(t, []string{"Stay hydrated"}, resp.Tips)
	require.Len(t, resp.Readings, 2)
	require.Equal(t, "2024-07-01", uvStub.lastDate)
	require.Equal(t, 1, chatStub.calls)
}

func TestServiceRecommendInvalidDate(t *testing.T) {
	svc := &service{
		cfg:      Config{},
		client:   &stubChatClient{},
		uvClient: &stubUVClient{},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		timezone: time.FixedZone("Asia/Singapore", 8*60*60),
		now:      time.Now,
	}

	_, err := svc.Recommend(context.Background(), Request{Date: "2024/01/01"})
	require.Error(t, err)
}

func TestParseAIAdvice(t *testing.T) {
	raw := "```json\n{\"summary\":\"Test\",\"clothing\":[\"Hat\"],\"protection\":[\"SPF\"],\"tips\":[]}\n```"
	advice, err := parseAIAdvice(raw)
	require.NoError(t, err)
	require.Equal(t, "Test", advice.Summary)
	require.Equal(t, []string{"Hat"}, advice.Clothing)
	require.Equal(t, []string{"SPF"}, advice.Protection)
	require.Empty(t, advice.Tips)
}

func TestParseAIAdviceStringFields(t *testing.T) {
	raw := "{\"summary\":\"Test\",\"clothing\":\"Long sleeves\",\"protection\":\"SPF 50\",\"tips\":\"Drink water\"}"
	advice, err := parseAIAdvice(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"Long sleeves"}, advice.Clothing)
	require.Equal(t, []string{"SPF 50"}, advice.Protection)
	require.Equal(t, []string{"Drink water"}, advice.Tips)
}

type stubChatClient struct {
	responses []chatgpt.ChatCompletionResponse
	err       error
	calls     int
}

func (s *stubChatClient) CreateChatCompletion(ctx context.Context, req chatgpt.ChatCompletionRequest) (chatgpt.ChatCompletionResponse, error) {
	if s.err != nil {
		return chatgpt.ChatCompletionResponse{}, s.err
	}
	if s.calls >= len(s.responses) {
		return chatgpt.ChatCompletionResponse{}, nil
	}
	resp := s.responses[s.calls]
	s.calls++
	return resp, nil
}

type stubUVClient struct {
	series   UVSeries
	err      error
	lastDate string
}

func (s *stubUVClient) Fetch(ctx context.Context, date string) (UVSeries, error) {
	if s.err != nil {
		return UVSeries{}, s.err
	}
	s.lastDate = date
	return s.series, nil
}

func mustParse(value string) time.Time {
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return ts
}
