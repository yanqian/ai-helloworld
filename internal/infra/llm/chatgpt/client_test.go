package chatgpt

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientOfflineFallbackWithoutAPIKey(t *testing.T) {
	client, err := NewClient("", "")
	require.NoError(t, err)
	require.NotNil(t, client)

	resp, err := client.CreateChatCompletion(context.Background(), ChatCompletionRequest{
		Messages: []Message{{Role: "system", Content: "Respond using SUMMARY: and KEYWORDS:"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	require.Contains(t, resp.Choices[0].Message.Content, "SUMMARY:")
	require.Contains(t, resp.Choices[0].Message.Content, "KEYWORDS:")
	require.Greater(t, resp.Usage.TotalTokens, 0)

	stream, err := client.CreateChatCompletionStream(context.Background(), ChatCompletionRequest{
		Messages: []Message{{Role: "system", Content: "Respond using SUMMARY: and KEYWORDS:"}},
	})
	require.NoError(t, err)
	chunk, err := stream.Recv()
	require.NoError(t, err)
	require.Contains(t, chunk.Choices[0].Delta.Content, "SUMMARY:")
	_, err = stream.Recv()
	require.True(t, errors.Is(err, io.EOF))

	emb, err := client.CreateEmbedding(context.Background(), EmbeddingRequest{Input: []string{"alpha", "beta"}})
	require.NoError(t, err)
	require.Len(t, emb.Data, 2)
	require.Len(t, emb.Data[0].Embedding, 1536)
	require.Equal(t, deterministicVector("alpha", 1536), emb.Data[0].Embedding)
}
