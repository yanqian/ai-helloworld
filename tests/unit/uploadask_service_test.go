package unit

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	uploadmemory "github.com/yanqian/ai-helloworld/internal/infra/uploadask/memory"
	uploadrepo "github.com/yanqian/ai-helloworld/internal/infra/uploadask/repo"
)

func TestAskSkipsMemoryWhenDisabled(t *testing.T) {
	chunkRepo := &stubChunkRepo{results: []uploadask.RetrievedChunk{{Chunk: uploadask.DocumentChunk{DocumentID: uuid.New()}}}}
	memStore := &stubMemoryStore{}
	msgLog := uploadmemory.NewMemoryMessageLog()
	cfg := baseUploadConfig()
	cfg.Memory.Enabled = false

	svc := newUploadService(cfg, chunkRepo, memStore, msgLog, &stubEmbedder{}, &stubLLM{})
	resp, err := svc.Ask(context.Background(), 1, uploadask.AskRequest{Query: "Hi"})
	require.NoError(t, err)
	require.NotZero(t, resp.SessionID)
	require.Empty(t, resp.Memories)
	require.Equal(t, 0, memStore.searchCalled)
	require.Empty(t, memStore.upserts)
}

func TestAskUsesMemoriesWhenEnabled(t *testing.T) {
	chunkRepo := &stubChunkRepo{results: []uploadask.RetrievedChunk{{Chunk: uploadask.DocumentChunk{DocumentID: uuid.New(), Content: "chunk"}}}}
	memStore := &stubMemoryStore{
		records: []uploadask.RetrievedMemory{
			{Memory: uploadask.MemoryRecord{Content: "remember this", Source: uploadask.MemorySourceQATurn}},
		},
	}
	msgLog := uploadmemory.NewMemoryMessageLog()
	cfg := baseUploadConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.PruneLimit = 1
	cfg.Memory.TopKMems = 1

	svc := newUploadService(cfg, chunkRepo, memStore, msgLog, &stubEmbedder{}, &stubLLM{response: "final"})
	resp, err := svc.Ask(context.Background(), 42, uploadask.AskRequest{Query: "Question?"})
	require.NoError(t, err)
	require.Len(t, resp.Memories, 1)
	require.Equal(t, 1, memStore.searchCalled)
	require.NotEmpty(t, memStore.upserts)
	require.Equal(t, chunkRepo.lastEmbedding, memStore.lastEmbedding)
}

func TestAskTrimsHistoryTokens(t *testing.T) {
	chunkRepo := &stubChunkRepo{}
	memStore := &stubMemoryStore{}
	msgLog := uploadmemory.NewMemoryMessageLog()
	sessions := uploadrepo.NewMemoryQASessionRepository()
	sessionID := uuid.New()
	_ = sessions.Create(context.Background(), uploadask.QASession{ID: sessionID, UserID: 7, CreatedAt: time.Now()})
	_ = msgLog.Append(context.Background(), uploadask.ConversationMessage{SessionID: sessionID, UserID: 7, Role: uploadask.MessageRoleUser, Content: "older", TokenCount: 10})
	_ = msgLog.Append(context.Background(), uploadask.ConversationMessage{SessionID: sessionID, UserID: 7, Role: uploadask.MessageRoleAssistant, Content: "newer", TokenCount: 5})

	cfg := baseUploadConfig()
	cfg.Memory.Enabled = true
	cfg.Memory.MaxHistoryTokens = 100
	llm := &stubLLM{response: "ok"}
	svc := uploadask.NewService(cfg, uploadrepo.NewMemoryDocumentRepository(), uploadrepo.NewMemoryFileRepository(), chunkRepo, sessions, uploadrepo.NewMemoryQueryLogRepository(), msgLog, memStore, nil, &stubEmbedder{}, llm, nil, nil, uploadaskTestLogger())

	maxTokens := 6
	resp, err := svc.Ask(context.Background(), 7, uploadask.AskRequest{
		Query:            "latest question",
		SessionID:        &sessionID,
		MaxHistoryTokens: &maxTokens,
	})
	require.NoError(t, err)
	require.Equal(t, 5, resp.UsedHistoryTokens)
	require.GreaterOrEqual(t, len(llm.lastMessages), 2)
	require.Equal(t, "assistant", llm.lastMessages[len(llm.lastMessages)-2].Role)
}

func baseUploadConfig() uploadask.Config {
	return uploadask.Config{
		VectorDim:       3,
		MaxFileBytes:    0,
		MaxRetrieved:    4,
		MaxPreviewChars: 120,
		Memory: uploadask.MemoryConfig{
			Enabled:            false,
			TopKMems:           2,
			MaxHistoryTokens:   8,
			MemoryVectorDim:    3,
			SummaryEveryNTurns: 0,
			PruneLimit:         2,
		},
	}
}

type stubChunkRepo struct {
	results       []uploadask.RetrievedChunk
	lastEmbedding []float32
}

func (s *stubChunkRepo) InsertBatch(ctx context.Context, chunks []uploadask.DocumentChunk) error {
	return nil
}
func (s *stubChunkRepo) SearchSimilar(ctx context.Context, userID int64, embedding []float32, filter uploadask.DocumentFilter) ([]uploadask.RetrievedChunk, error) {
	s.lastEmbedding = append([]float32(nil), embedding...)
	return s.results, nil
}

type stubMemoryStore struct {
	records       []uploadask.RetrievedMemory
	searchCalled  int
	lastEmbedding []float32
	upserts       []uploadask.MemoryRecord
	prunes        int
}

func (s *stubMemoryStore) Upsert(ctx context.Context, mem uploadask.MemoryRecord) error {
	s.upserts = append(s.upserts, mem)
	return nil
}

func (s *stubMemoryStore) Search(ctx context.Context, userID int64, sessionID uuid.UUID, embedding []float32, k int) ([]uploadask.RetrievedMemory, error) {
	s.searchCalled++
	s.lastEmbedding = append([]float32(nil), embedding...)
	if k > 0 && len(s.records) > k {
		return s.records[:k], nil
	}
	return s.records, nil
}

func (s *stubMemoryStore) Prune(ctx context.Context, userID int64, sessionID *uuid.UUID, limit int) error {
	s.prunes++
	return nil
}

type stubEmbedder struct{}

func (stubEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{1, 0, float32(i)}
	}
	return result, nil
}

type stubLLM struct {
	response     string
	lastMessages []uploadask.LLMMessage
}

func (s *stubLLM) Chat(ctx context.Context, messages []uploadask.LLMMessage) (string, error) {
	s.lastMessages = messages
	if s.response != "" {
		return s.response, nil
	}
	return "stub-answer", nil
}

func newUploadService(cfg uploadask.Config, chunkRepo uploadask.ChunkRepository, memStore uploadask.MemoryStore, msgLog uploadask.MessageLog, embedder uploadask.Embedder, llm uploadask.LLM) *uploadask.Service {
	return uploadask.NewService(
		cfg,
		uploadrepo.NewMemoryDocumentRepository(),
		uploadrepo.NewMemoryFileRepository(),
		chunkRepo,
		uploadrepo.NewMemoryQASessionRepository(),
		uploadrepo.NewMemoryQueryLogRepository(),
		msgLog,
		memStore,
		nil,
		embedder,
		llm,
		nil,
		nil,
		uploadaskTestLogger(),
	)
}

func uploadaskTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
