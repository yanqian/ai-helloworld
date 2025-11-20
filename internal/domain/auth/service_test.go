package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_RegisterLoginAndRefresh(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(Config{
		Secret:          "test-secret",
		TokenTTL:        time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}, repo, newTestLogger())

	view, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "User@Example.com",
		Password: "pass1234",
		Nickname: "CodeStar",
	})
	require.NoError(t, err)
	require.Equal(t, "user@example.com", view.Email)
	require.Equal(t, "CodeStar", view.Nickname)
	require.NotZero(t, view.ID)

	resp, err := svc.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "pass1234",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Token)
	require.NotEmpty(t, resp.RefreshToken)
	require.Equal(t, view.Email, resp.User.Email)

	claims, err := svc.ValidateToken(context.Background(), resp.Token)
	require.NoError(t, err)
	require.Equal(t, view.ID, claims.UserID)
	require.Equal(t, view.Email, claims.Email)
	require.WithinDuration(t, time.Now().Add(time.Hour), claims.ExpiresAt, time.Minute)

	refreshed, err := svc.Refresh(context.Background(), resp.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, resp.Token, refreshed.Token)
	require.Equal(t, resp.User.Email, refreshed.User.Email)
	require.Equal(t, "CodeStar", refreshed.User.Nickname)
}

func TestService_DuplicateEmail(t *testing.T) {
	repo := newMemoryRepo()
	svc := NewService(Config{
		Secret:          "test-secret",
		TokenTTL:        time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}, repo, newTestLogger())

	_, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "user@example.com",
		Password: "pass1234",
		Nickname: "NickOne",
	})
	require.NoError(t, err)

	_, err = svc.Register(context.Background(), RegisterRequest{
		Email:    "user@example.com",
		Password: "pass12345",
		Nickname: "NickTwo",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func newTestLogger() *slog.Logger {
	handler := slog.NewTextHandler(io.Discard, nil)
	return slog.New(handler)
}

type memoryRepo struct {
	users map[int64]User
	seq   int64
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{users: make(map[int64]User)}
}

func (m *memoryRepo) Create(_ context.Context, email, nickname, passwordHash string) (User, error) {
	m.seq++
	user := User{
		ID:           m.seq,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	m.users[user.ID] = user
	return user, nil
}

func (m *memoryRepo) GetByEmail(_ context.Context, email string) (User, bool, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, true, nil
		}
	}
	return User{}, false, nil
}

func (m *memoryRepo) GetByID(_ context.Context, id int64) (User, bool, error) {
	user, ok := m.users[id]
	return user, ok, nil
}
