package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"
)

type jobEnvelope struct {
	Name    string         `json:"name"`
	Payload map[string]any `json:"payload"`
}

// ValkeyQueue persists jobs in Valkey and delivers them to a handler.
type ValkeyQueue struct {
	client      valkey.Client
	queueKey    string
	handler     Handler
	logger      *slog.Logger
	stop        chan struct{}
	pollTimeout time.Duration
}

// NewValkeyQueue constructs a Valkey-backed queue.
func NewValkeyQueue(client valkey.Client, queueKey string, logger *slog.Logger) *ValkeyQueue {
	if queueKey == "" {
		queueKey = "uploadask:jobs"
	}
	return &ValkeyQueue{
		client:      client,
		queueKey:    queueKey,
		logger:      logger,
		stop:        make(chan struct{}),
		pollTimeout: 5 * time.Second,
	}
}

// SetHandler starts the worker loop that pops jobs and invokes the handler.
func (q *ValkeyQueue) SetHandler(handler Handler) {
	q.handler = handler
	if handler == nil {
		return
	}
	go q.consume()
}

// Enqueue pushes a job onto the queue.
func (q *ValkeyQueue) Enqueue(ctx context.Context, name string, payload any) error {
	typed, ok := payload.(map[string]any)
	if !ok {
		typed = map[string]any{}
	}
	encoded, err := json.Marshal(jobEnvelope{Name: name, Payload: typed})
	if err != nil {
		return err
	}
	cmd := q.client.B().Lpush().Key(q.queueKey).Element(string(encoded)).Build()
	return q.client.Do(ctx, cmd).Error()
}

func (q *ValkeyQueue) consume() {
	ctx := context.Background()
	for {
		select {
		case <-q.stop:
			return
		default:
		}
		resp := q.client.Do(ctx, q.client.B().Brpop().Key(q.queueKey).Timeout(q.pollTimeout.Seconds()).Build())
		values, err := resp.ToArray()
		if err != nil {
			if !valkey.IsValkeyNil(err) {
				q.logger.Warn("valkey queue pop failed", "error", err)
			}
			continue
		}
		if len(values) < 2 || q.handler == nil {
			continue
		}
		raw, err := values[1].ToString()
		if err != nil {
			q.logger.Warn("valkey queue payload decode failed", "error", err)
			continue
		}
		var job jobEnvelope
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			q.logger.Warn("valkey queue unmarshal failed", "error", err)
			continue
		}
		q.handler(ctx, job.Name, job.Payload)
	}
}

var _ HandlerQueue = (*ValkeyQueue)(nil)
