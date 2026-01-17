package userrepo

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
)

// MemoryRepository provides an in-memory user store for tests/dev.
type MemoryRepository struct {
	mu         sync.RWMutex
	users      map[int64]auth.User
	emailIndex map[string]int64
	identities map[string]auth.Identity
	userIndex  map[string]auth.Identity
	seq        int64
	identityID int64
}

// NewMemoryRepository constructs a new in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:      make(map[int64]auth.User),
		emailIndex: make(map[string]int64),
		identities: make(map[string]auth.Identity),
		userIndex:  make(map[string]auth.Identity),
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

// GetIdentity returns an identity by provider and subject.
func (r *MemoryRepository) GetIdentity(_ context.Context, provider, providerSubject string) (auth.Identity, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := identityKey(provider, providerSubject)
	identity, ok := r.identities[key]
	return identity, ok, nil
}

// GetIdentityByUser returns an identity by user and provider.
func (r *MemoryRepository) GetIdentityByUser(_ context.Context, userID int64, provider string) (auth.Identity, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := userIdentityKey(provider, userID)
	identity, ok := r.userIndex[key]
	return identity, ok, nil
}

// UpsertIdentity stores or updates the identity mapping.
func (r *MemoryRepository) UpsertIdentity(_ context.Context, identity auth.Identity) (auth.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if identity.UserID == 0 {
		return auth.Identity{}, errors.New("userID is required")
	}
	key := identityKey(identity.Provider, identity.ProviderSubject)
	existing, ok := r.identities[key]
	if ok {
		if identity.RefreshToken != "" {
			existing.RefreshToken = identity.RefreshToken
		}
		if identity.ProviderEmail != "" {
			existing.ProviderEmail = identity.ProviderEmail
		}
		existing.UpdatedAt = time.Now().UTC()
		r.identities[key] = existing
		r.userIndex[userIdentityKey(existing.Provider, existing.UserID)] = existing
		return existing, nil
	}
	r.identityID++
	identity.ID = r.identityID
	now := time.Now().UTC()
	identity.CreatedAt = now
	identity.UpdatedAt = now
	r.identities[key] = identity
	r.userIndex[userIdentityKey(identity.Provider, identity.UserID)] = identity
	return identity, nil
}

var _ auth.Repository = (*MemoryRepository)(nil)

func identityKey(provider, subject string) string {
	return provider + ":" + subject
}

func userIdentityKey(provider string, userID int64) string {
	return provider + ":" + strconv.FormatInt(userID, 10)
}
