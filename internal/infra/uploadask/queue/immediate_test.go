package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type contextKey string

func TestImmediateQueueHandlerContextSurvivesEnqueueContextCancellation(t *testing.T) {
	handlerCtx := make(chan context.Context, 1)
	q := NewImmediateQueue(func(ctx context.Context, name string, payload map[string]any) {
		handlerCtx <- ctx
	})

	parent := context.WithValue(context.Background(), contextKey("trace"), "upload-request")
	ctx, cancel := context.WithCancel(parent)
	require.NoError(t, q.Enqueue(ctx, "process_document", map[string]any{"document_id": "doc"}))
	cancel()

	var got context.Context
	select {
	case got = <-handlerCtx:
	case <-time.After(time.Second):
		t.Fatal("handler was not invoked")
	}

	require.Equal(t, "upload-request", got.Value(contextKey("trace")))
	require.NoError(t, got.Err())
	select {
	case <-got.Done():
		t.Fatal("handler context was canceled by enqueue context cancellation")
	default:
	}
}
