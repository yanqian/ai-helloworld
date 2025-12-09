package uploadask

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeMessageLog struct {
	msgs []ConversationMessage
	err  error
}

func (f fakeMessageLog) Append(context.Context, ConversationMessage) error {
	return nil
}

func (f fakeMessageLog) ListRecent(context.Context, int64, uuid.UUID, int, int) ([]ConversationMessage, error) {
	return f.msgs, f.err
}

type fakeQueue struct {
	jobs []struct {
		name    string
		payload map[string]any
	}
}

func (f *fakeQueue) Enqueue(_ context.Context, name string, payload any) error {
	typed, _ := payload.(map[string]any)
	f.jobs = append(f.jobs, struct {
		name    string
		payload map[string]any
	}{name: name, payload: typed})
	return nil
}

type fakeLLM struct {
	resp string
	err  error
}

func (f fakeLLM) Chat(context.Context, []LLMMessage) (string, error) {
	return f.resp, f.err
}

type fakeEmbedder struct {
	vec []float32
	err error
}

func (f fakeEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return [][]float32{f.vec}, nil
}

type fakeMemoryStore struct {
	results   []RetrievedMemory
	err       error
	upserts   []MemoryRecord
	upsertErr error
}

func (f *fakeMemoryStore) Upsert(ctx context.Context, mem MemoryRecord) error {
	f.upserts = append(f.upserts, mem)
	return f.upsertErr
}

func (f *fakeMemoryStore) Search(context.Context, int64, uuid.UUID, []float32, int) ([]RetrievedMemory, error) {
	return f.results, f.err
}

func (f *fakeMemoryStore) Prune(context.Context, int64, *uuid.UUID, int) error {
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestLoadHistory(t *testing.T) {
	svc := &Service{
		cfg:      Config{Memory: MemoryConfig{Enabled: true}},
		messages: fakeMessageLog{msgs: []ConversationMessage{{TokenCount: 3}, {TokenCount: 4}}},
		logger:   testLogger(),
	}
	sessionID := uuid.New()

	msgs, tokens := svc.loadHistory(context.Background(), 1, sessionID, 100, true)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if tokens != 7 {
		t.Fatalf("expected 7 tokens, got %d", tokens)
	}
}

func TestLoadHistorySkipsWhenDisabled(t *testing.T) {
	svc := &Service{logger: testLogger()}
	sessionID := uuid.New()

	msgs, tokens := svc.loadHistory(context.Background(), 1, sessionID, 100, false)

	if msgs != nil || tokens != 0 {
		t.Fatalf("expected nil history and 0 tokens, got %v and %d", msgs, tokens)
	}
}

func TestBuildSemanticQuery(t *testing.T) {
	history := []ConversationMessage{
		{Role: MessageRoleUser, Content: "Hi"},
		{Role: MessageRoleAssistant, Content: "Hello, how can I help?"},
	}
	svc := &Service{logger: testLogger()}

	out := svc.buildSemanticQuery("What is the status?", history)

	if !strings.Contains(out, "What is the status?") || !strings.Contains(out, "Recent history:") {
		t.Fatalf("semantic query missing expected parts: %s", out)
	}
	if !strings.Contains(out, "Hi") || !strings.Contains(out, "Hello") {
		t.Fatalf("semantic query missing summarized history: %s", out)
	}
}

func TestSummarizeHistoryLimitsEntriesAndChars(t *testing.T) {
	history := []ConversationMessage{
		{Role: MessageRoleUser, Content: "first"},
		{Role: MessageRoleAssistant, Content: "second"},
		{Role: MessageRoleUser, Content: "third"},
	}

	summary := summarizeHistory(history, 2, 100)
	if strings.Contains(summary, "first") {
		t.Fatalf("expected oldest entry to be dropped: %s", summary)
	}
	if !strings.HasPrefix(summary, "user: third") {
		t.Fatalf("expected most recent first in summary, got %s", summary)
	}

	short := summarizeHistory(history, 3, len("user: third"))
	if strings.Contains(short, "second") {
		t.Fatalf("expected char limit to truncate older entries: %s", short)
	}
}

func TestSearchMemories(t *testing.T) {
	mem := RetrievedMemory{Memory: MemoryRecord{Content: "note"}}
	svc := &Service{
		cfg:      Config{Memory: MemoryConfig{Enabled: true}},
		memories: &fakeMemoryStore{results: []RetrievedMemory{mem}},
		logger:   testLogger(),
	}

	out := svc.searchMemories(context.Background(), 1, uuid.New(), []float32{0.1}, 1)
	if len(out) != 1 || out[0].Memory.Content != "note" {
		t.Fatalf("unexpected memory search result: %#v", out)
	}

	svc.memories = &fakeMemoryStore{err: io.ErrUnexpectedEOF}
	out = svc.searchMemories(context.Background(), 1, uuid.New(), []float32{0.1}, 1)
	if out != nil {
		t.Fatalf("expected nil on memory search error, got %#v", out)
	}
}

func TestMaybeTriggerSummaryEnqueues(t *testing.T) {
	sessionID := uuid.New()
	queue := &fakeQueue{}
	svc := &Service{
		cfg:    Config{Memory: MemoryConfig{Enabled: true, SummaryEveryNTurns: 3}},
		queue:  queue,
		logger: testLogger(),
	}

	svc.maybeTriggerSummary(context.Background(), 42, sessionID, 3)

	if len(queue.jobs) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(queue.jobs))
	}
	job := queue.jobs[0]
	if job.name != "summarize_session" {
		t.Fatalf("unexpected job name: %s", job.name)
	}
	if job.payload["session_id"] != sessionID.String() || job.payload["user_id"] != int64(42) {
		t.Fatalf("unexpected payload: %#v", job.payload)
	}
}

func TestSummarizeSessionStoresMemory(t *testing.T) {
	sessionID := uuid.New()
	memStore := &fakeMemoryStore{}
	svc := &Service{
		cfg: Config{Memory: MemoryConfig{Enabled: true, MaxHistoryTokens: 500}},
		messages: fakeMessageLog{msgs: []ConversationMessage{
			{Role: MessageRoleUser, Content: "Hi"},
			{Role: MessageRoleAssistant, Content: "Answer about docs"},
		}},
		memories: memStore,
		llm:      fakeLLM{resp: "Summary note"},
		embedder: fakeEmbedder{vec: []float32{0.1, 0.2}},
		logger:   testLogger(),
	}

	svc.SummarizeSession(context.Background(), 99, sessionID)

	if len(memStore.upserts) != 1 {
		t.Fatalf("expected one summary upsert, got %d", len(memStore.upserts))
	}
	upsert := memStore.upserts[0]
	if upsert.Source != MemorySourceSummary || upsert.Content != "Summary note" {
		t.Fatalf("unexpected memory record: %#v", upsert)
	}
	if len(upsert.Embedding) != 2 {
		t.Fatalf("expected embedding to be set")
	}
}
