package userrepo

import (
	"context"
	"sync"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
)

// MemoryRepository provides an in-memory user store for tests/dev.
type MemoryRepository struct {
	mu         sync.RWMutex
	users      map[int64]auth.User
	emailIndex map[string]int64
	seq        int64
}

// NewMemoryRepository constructs a new in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:      make(map[int64]auth.User),
		emailIndex: make(map[string]int64),
	}
}

// Create stores the user record.
func (r *MemoryRepository) Create(_ context.Context, email, nickname, passwordHash string) (auth.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.emailIndex[email]; exists {
		return auth.User{}, auth.ErrEmailExists
	}
	r.seq++
	user := auth.User{
		ID:           r.seq,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	r.users[user.ID] = user
	r.emailIndex[email] = user.ID
	return user, nil
}

// GetByEmail returns a user by email.
func (r *MemoryRepository) GetByEmail(_ context.Context, email string) (auth.User, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.emailIndex[email]; ok {
		return r.users[id], true, nil
	}
	return auth.User{}, false, nil
}

// GetByID fetches by ID.
func (r *MemoryRepository) GetByID(_ context.Context, id int64) (auth.User, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	return user, ok, nil
}

var _ auth.Repository = (*MemoryRepository)(nil)
