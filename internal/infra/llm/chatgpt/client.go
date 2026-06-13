package chatgpt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Message mirrors the OpenAI chat message structure.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatCompletionRequest is the payload sent to the ChatGPT API.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// ChatCompletionResponse captures the response for non streaming calls.
type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage TokenUsage `json:"usage"`
}

// EmbeddingRequest represents an embedding call payload.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

// EmbeddingResponse captures the embedding vectors returned by OpenAI.
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage TokenUsage `json:"usage"`
}

// Tool represents a callable function exposed to ChatGPT.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines the shape of a callable tool.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCall is returned when ChatGPT wants to call a function.
type ToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function ToolCallDefinition `json:"function"`
}

// ToolCallDefinition contains the function payload.
type ToolCallDefinition struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionStreamChunk captures a streaming frame from ChatGPT.
type ChatCompletionStreamChunk struct {
	Choices []struct {
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
}

// TokenUsage captures prompt/completion token counts returned by the API.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens"`
}

// Client performs HTTP requests to the ChatGPT API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	offline    bool
}

// NewClient constructs a ChatGPT client.
func NewClient(apiKey, baseURL string) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	apiKey = strings.TrimSpace(apiKey)
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // embeddings on large batches can take longer; cap at 3 minutes
		},
		offline: apiKey == "",
	}, nil
}

// CreateChatCompletion triggers a sync ChatGPT call.
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	if c.offline {
		return offlineChatCompletion(req), nil
	}
	var out ChatCompletionResponse
	body, err := c.doRequest(ctx, req)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode chat completion: %w", err)
	}
	return out, nil
}

// CreateEmbedding requests an embedding vector from the OpenAI embeddings API.
func (c *Client) CreateEmbedding(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	if c.offline {
		return offlineEmbedding(req), nil
	}
	var out EmbeddingResponse
	body, err := c.doEmbeddingRequest(ctx, req)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode embedding response: %w", err)
	}
	return out, nil
}

// CreateChatCompletionStream starts a streaming ChatGPT call.
func (c *Client) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
	if c.offline {
		return &offlineStream{chunks: []ChatCompletionStreamChunk{streamChunk(offlineChatContent(req))}}, nil
	}
	req.Stream = true

	httpReq, err := c.newHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request chat completion stream: %w", err)
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("chatgpt stream failed: status=%d body=%s", resp.StatusCode, string(payload))
	}

	reader := bufio.NewScanner(resp.Body)
	reader.Buffer(make([]byte, 0, 1024), 1<<20)

	return &ChatCompletionStream{
		scanner: reader,
		closer:  resp.Body,
	}, nil
}

func (c *Client) doRequest(ctx context.Context, req ChatCompletionRequest) ([]byte, error) {
	httpReq, err := c.newHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request chat completion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("chatgpt request failed: status=%d body=%s", resp.StatusCode, string(payload))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) newHTTPRequest(ctx context.Context, req ChatCompletionRequest) (*http.Request, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode chat completion request: %w", err)
	}
	endpoint := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build chat completion request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	return httpReq, nil
}

func (c *Client) doEmbeddingRequest(ctx context.Context, req EmbeddingRequest) ([]byte, error) {
	httpReq, err := c.newEmbeddingRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request embedding: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("embedding request failed: status=%d body=%s", resp.StatusCode, string(payload))
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) newEmbeddingRequest(ctx context.Context, req EmbeddingRequest) (*http.Request, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}
	endpoint := c.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

// Stream defines the interface for streaming chat completions.
type Stream interface {
	Recv() (ChatCompletionStreamChunk, error)
	Close() error
}

// ChatCompletionStream wraps a streaming HTTP response.
type ChatCompletionStream struct {
	scanner *bufio.Scanner
	closer  io.Closer
}

// Recv reads the next streaming chunk.
func (s *ChatCompletionStream) Recv() (ChatCompletionStreamChunk, error) {
	for {
		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				s.Close()
				return ChatCompletionStreamChunk{}, err
			}
			s.Close()
			return ChatCompletionStreamChunk{}, io.EOF
		}
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			s.Close()
			return ChatCompletionStreamChunk{}, io.EOF
		}
		var chunk ChatCompletionStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			s.Close()
			return ChatCompletionStreamChunk{}, fmt.Errorf("decode stream chunk: %w", err)
		}
		return chunk, nil
	}
}

// Close closes the underlying stream.
func (s *ChatCompletionStream) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

type offlineStream struct {
	chunks []ChatCompletionStreamChunk
	index  int
}

func (s *offlineStream) Recv() (ChatCompletionStreamChunk, error) {
	if s.index >= len(s.chunks) {
		return ChatCompletionStreamChunk{}, io.EOF
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

func (s *offlineStream) Close() error {
	s.index = len(s.chunks)
	return nil
}

func streamChunk(content string) ChatCompletionStreamChunk {
	var chunk ChatCompletionStreamChunk
	chunk.Choices = append(chunk.Choices, struct {
		Delta        Message `json:"delta"`
		FinishReason string  `json:"finish_reason"`
	}{
		Delta: Message{Role: "assistant", Content: content},
	})
	return chunk
}

func offlineChatCompletion(req ChatCompletionRequest) ChatCompletionResponse {
	content := offlineChatContent(req)
	promptTokens := estimateTokens(joinMessageContent(req.Messages))
	completionTokens := estimateTokens(content)
	var resp ChatCompletionResponse
	resp.Choices = append(resp.Choices, struct {
		Message Message `json:"message"`
	}{
		Message: Message{Role: "assistant", Content: content},
	})
	resp.Usage = TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
	return resp
}

func offlineChatContent(req ChatCompletionRequest) string {
	content := joinMessageContent(req.Messages)
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "uv protection stylist") || strings.Contains(lower, "clothing") && strings.Contains(lower, "protection"):
		return `{"summary":"Offline local UV advice generated without a live LLM key.","clothing":["Light breathable clothing"],"protection":["Use sunscreen"],"tips":["Check live UV data before going outside"]}`
	case strings.Contains(lower, "summary:") || strings.Contains(lower, "keywords:"):
		return "SUMMARY:\nOffline local summary generated without a live LLM key.\n\nKEYWORDS:\nlocal, offline, summary"
	default:
		return "Offline local answer generated without a live LLM key."
	}
}

func joinMessageContent(messages []Message) string {
	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		parts = append(parts, msg.Content)
	}
	return strings.Join(parts, "\n")
}

func offlineEmbedding(req EmbeddingRequest) EmbeddingResponse {
	inputs := embeddingInputs(req.Input)
	var resp EmbeddingResponse
	tokens := 0
	for _, input := range inputs {
		resp.Data = append(resp.Data, struct {
			Embedding []float32 `json:"embedding"`
		}{Embedding: deterministicVector(input, 1536)})
		tokens += estimateTokens(input)
	}
	resp.Usage = TokenUsage{PromptTokens: tokens, TotalTokens: tokens}
	return resp
}

func embeddingInputs(input any) []string {
	switch val := input.(type) {
	case string:
		return []string{val}
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(input)}
	}
}

func deterministicVector(text string, dim int) []float32 {
	vector := make([]float32, dim)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(text))
	seed := hash.Sum64()
	for i := range vector {
		seed = seed*1099511628211 + 1469598103934665603
		vector[i] = float32(seed%997) / 997.0
	}
	return vector
}

func estimateTokens(text string) int {
	tokens := len(strings.Fields(text))
	if tokens == 0 && strings.TrimSpace(text) != "" {
		return 1
	}
	return tokens
}
