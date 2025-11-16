package faqstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

type answerRecord struct {
	payload   faq.AnswerRecord
	expiresAt time.Time
}

// MemoryStore is an in-memory implementation of the FAQ store for tests/dev.
type MemoryStore struct {
	mu       sync.RWMutex
	answers  map[int64]answerRecord
	trending map[string]int64
	displays map[string]string
}

// NewMemoryStore constructs a store backed by process memory.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		answers:  make(map[int64]answerRecord),
		trending: make(map[string]int64),
		displays: make(map[string]string),
	}
}

// GetAnswer implements faq.Store.
func (s *MemoryStore) GetAnswer(_ context.Context, questionID int64) (faq.AnswerRecord, bool, error) {
	if questionID <= 0 {
		return faq.AnswerRecord{}, false, nil
	}
	s.mu.RLock()
	record, ok := s.answers[questionID]
	s.mu.RUnlock()
	if !ok {
		return faq.AnswerRecord{}, false, nil
	}
	if hasExpired(record.expiresAt) {
		s.mu.Lock()
		delete(s.answers, questionID)
		s.mu.Unlock()
		return faq.AnswerRecord{}, false, nil
	}
	answer := record.payload
	return answer, true, nil
}

// SaveAnswer caches the answer with optional TTL.
func (s *MemoryStore) SaveAnswer(_ context.Context, record faq.AnswerRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	s.answers[record.QuestionID] = answerRecord{
		payload:   record,
		expiresAt: exp,
	}
	return nil
}

// IncrementQuery bumps the counter for a canonical query and records a display string.
func (s *MemoryStore) IncrementQuery(_ context.Context, canonical, display string) error {
	if canonical == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trending[canonical]++
	if _, exists := s.displays[canonical]; !exists {
		s.displays[canonical] = display
	}
	return nil
}

// TopQueries returns the most frequent canonical questions.
func (s *MemoryStore) TopQueries(_ context.Context, limit int) ([]faq.TrendingQuery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = len(s.trending)
	}
	items := make([]faq.TrendingQuery, 0, len(s.trending))
	for canonical, count := range s.trending {
		display := s.displays[canonical]
		if display == "" {
			display = canonical
		}
		items = append(items, faq.TrendingQuery{Query: display, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Query < items[j].Query
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func hasExpired(ts time.Time) bool {
	if ts.IsZero() {
		return false
	}
	return ts.Before(time.Now())
}

var _ faq.Store = (*MemoryStore)(nil)
