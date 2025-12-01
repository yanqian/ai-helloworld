package queue

import (
	"context"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// HandlerQueue supports setting a handler for job delivery.
type HandlerQueue interface {
	domain.JobQueue
	SetHandler(handler Handler)
}

// Handler executes jobs synchronously or in the background.
type Handler func(ctx context.Context, name string, payload map[string]any)

// ImmediateQueue calls the handler immediately on enqueue.
type ImmediateQueue struct {
	handler Handler
}

// NewImmediateQueue constructs the queue.
func NewImmediateQueue(handler Handler) *ImmediateQueue {
	return &ImmediateQueue{handler: handler}
}

// SetHandler replaces the handler used for queued jobs.
func (q *ImmediateQueue) SetHandler(handler Handler) {
	q.handler = handler
}

// Enqueue invokes the handler asynchronously.
func (q *ImmediateQueue) Enqueue(ctx context.Context, name string, payload any) error {
	typed, ok := payload.(map[string]any)
	if !ok {
		typed = map[string]any{}
	}
	if q.handler == nil {
		return nil
	}
	go q.handler(ctx, name, typed)
	return nil
}

var _ domain.JobQueue = (*ImmediateQueue)(nil)
var _ HandlerQueue = (*ImmediateQueue)(nil)
