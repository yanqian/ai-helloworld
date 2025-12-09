package memory

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// MemoryMessageLog stores conversation turns in-memory.
type MemoryMessageLog struct {
	mu       sync.RWMutex
	nextID   int64
	messages map[uuid.UUID][]domain.ConversationMessage
}

// NewMemoryMessageLog constructs the in-memory message log.
func NewMemoryMessageLog() *MemoryMessageLog {
	return &MemoryMessageLog{
		messages: make(map[uuid.UUID][]domain.ConversationMessage),
	}
}

// Append stores a conversation message.
func (l *MemoryMessageLog) Append(_ context.Context, msg domain.ConversationMessage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if msg.ID == 0 {
		l.nextID++
		msg.ID = l.nextID
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	l.messages[msg.SessionID] = append(l.messages[msg.SessionID], msg)
	return nil
}

// ListRecent returns recent messages capped by tokens and count.
func (l *MemoryMessageLog) ListRecent(_ context.Context, userID int64, sessionID uuid.UUID, maxTokens int, maxMessages int) ([]domain.ConversationMessage, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	msgs := l.messages[sessionID]
	selected := make([]domain.ConversationMessage, 0, len(msgs))
	totalTokens := 0

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.UserID != userID {
			continue
		}
		if maxMessages > 0 && len(selected) >= maxMessages {
			break
		}
		tokens := msg.TokenCount
		if tokens < 0 {
			tokens = 0
		}
		if maxTokens > 0 && totalTokens+tokens > maxTokens {
			break
		}
		totalTokens += tokens
		selected = append(selected, msg)
	}

	// reverse to chronological order
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected, nil
}

var _ domain.MessageLog = (*MemoryMessageLog)(nil)

// MemoryStore keeps memory records in-memory.
type MemoryStore struct {
	mu       sync.RWMutex
	nextID   int64
	memories map[uuid.UUID][]domain.MemoryRecord
}

// NewMemoryStore constructs the in-memory memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		memories: make(map[uuid.UUID][]domain.MemoryRecord),
	}
}

// Upsert inserts or replaces a memory by unique key (user + session + source + content).
func (s *MemoryStore) Upsert(_ context.Context, mem domain.MemoryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mem.ID == 0 {
		s.nextID++
		mem.ID = s.nextID
	}
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	list := s.memories[mem.SessionID]
	updated := false
	for i, existing := range list {
		if existing.UserID == mem.UserID && existing.Source == mem.Source && existing.Content == mem.Content {
			mem.ID = existing.ID
			list[i] = mem
			updated = true
			break
		}
	}
	if !updated {
		list = append(list, mem)
	}
	s.memories[mem.SessionID] = list
	return nil
}

// Search returns top-k memories by cosine similarity.
func (s *MemoryStore) Search(_ context.Context, userID int64, sessionID uuid.UUID, embedding []float32, k int) ([]domain.RetrievedMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates := s.memories[sessionID]

	results := make([]domain.RetrievedMemory, 0)
	for _, mem := range candidates {
		if mem.UserID != userID {
			continue
		}
		if len(mem.Embedding) == 0 {
			continue
		}
		score := cosineSimilarity(embedding, mem.Embedding)
		results = append(results, domain.RetrievedMemory{
			Memory:    mem,
			Score:     score,
			CreatedAt: mem.CreatedAt,
		})
	}

	// sort by score desc
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if k > 0 && len(results) > k {
		results = results[:k]
	}
	return results, nil
}

// Prune drops older memories beyond the limit (per session if provided).
func (s *MemoryStore) Prune(_ context.Context, userID int64, sessionID *uuid.UUID, limit int) error {
	if limit <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions := make([]uuid.UUID, 0, len(s.memories))
	if sessionID != nil {
		sessions = append(sessions, *sessionID)
	} else {
		for id := range s.memories {
			sessions = append(sessions, id)
		}
	}

	for _, sid := range sessions {
		list := s.memories[sid]
		filtered := make([]domain.MemoryRecord, 0, len(list))
		for _, mem := range list {
			if mem.UserID == userID {
				filtered = append(filtered, mem)
			}
		}
		if len(filtered) <= limit {
			s.memories[sid] = filtered
			continue
		}
		// keep most recent and highest importance first
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].Importance == filtered[j].Importance {
				return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
			}
			return filtered[i].Importance > filtered[j].Importance
		})
		s.memories[sid] = append([]domain.MemoryRecord(nil), filtered[:limit]...)
	}
	return nil
}

var _ domain.MemoryStore = (*MemoryStore)(nil)

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	var magA float64
	var magB float64
	for i := 0; i < len(a); i++ {
		dot += float64(a[i] * b[i])
		magA += float64(a[i] * a[i])
		magB += float64(b[i] * b[i])
	}
	den := math.Sqrt(magA) * math.Sqrt(magB)
	if den == 0 {
		return 0
	}
	return dot / den
}
