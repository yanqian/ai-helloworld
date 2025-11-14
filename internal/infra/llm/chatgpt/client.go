package chatgpt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Client performs HTTP requests to the ChatGPT API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient constructs a ChatGPT client.
func NewClient(apiKey, baseURL string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("chatgpt api key cannot be empty")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// CreateChatCompletion triggers a sync ChatGPT call.
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
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

// CreateChatCompletionStream starts a streaming ChatGPT call.
func (c *Client) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error) {
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
