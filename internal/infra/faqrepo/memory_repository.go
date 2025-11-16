package faqrepo

import (
	"context"
	"math"
	"sync"

	"github.com/yanqian/ai-helloworld/internal/domain/faq"
)

type memoryQuestion struct {
	record    faq.QuestionRecord
	embedding []float32
}

// MemoryRepository is an in-memory QuestionRepository used for tests/dev.
type MemoryRepository struct {
	mu     sync.RWMutex
	nextID int64

	records map[int64]memoryQuestion
	byText  map[string]int64
	byHash  map[uint64]int64
}

// NewMemoryRepository constructs a repo backed by memory.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID:  1,
		records: make(map[int64]memoryQuestion),
		byText:  make(map[string]int64),
		byHash:  make(map[uint64]int64),
	}
}

// FindExact implements faq.QuestionRepository.
func (r *MemoryRepository) FindExact(_ context.Context, question string) (faq.QuestionRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byText[question]
	if !ok {
		return faq.QuestionRecord{}, false, nil
	}
	rec := r.records[id]
	return rec.record, true, nil
}

// FindBySemanticHash implements faq.QuestionRepository.
func (r *MemoryRepository) FindBySemanticHash(_ context.Context, hash uint64) (faq.QuestionRecord, bool, error) {
	if hash == 0 {
		return faq.QuestionRecord{}, false, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byHash[hash]
	if !ok {
		return faq.QuestionRecord{}, false, nil
	}
	rec := r.records[id]
	return rec.record, true, nil
}

// FindNearest implements faq.QuestionRepository.
func (r *MemoryRepository) FindNearest(_ context.Context, embedding []float32) (faq.SimilarityMatch, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var (
		best   faq.SimilarityMatch
		hasAny bool
	)
	for _, candidate := range r.records {
		dist := euclideanDistance(embedding, candidate.embedding)
		if !hasAny || dist < best.Distance {
			hasAny = true
			best = faq.SimilarityMatch{
				Question: candidate.record,
				Distance: dist,
			}
		}
	}
	if !hasAny {
		return faq.SimilarityMatch{}, false, nil
	}
	return best, true, nil
}

// InsertQuestion implements faq.QuestionRepository.
func (r *MemoryRepository) InsertQuestion(_ context.Context, question string, embedding []float32, hash *uint64) (faq.QuestionRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID
	r.nextID++

	record := faq.QuestionRecord{
		ID:           id,
		QuestionText: question,
	}
	if hash != nil {
		clone := *hash
		record.SemanticHash = &clone
		r.byHash[clone] = id
	}

	r.records[id] = memoryQuestion{
		record:    record,
		embedding: append([]float32(nil), embedding...),
	}
	r.byText[question] = id

	return record, nil
}

func euclideanDistance(a, b []float32) float64 {
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	var sum float64
	for i := 0; i < length; i++ {
		diff := float64(a[i] - b[i])
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

var _ faq.QuestionRepository = (*MemoryRepository)(nil)
