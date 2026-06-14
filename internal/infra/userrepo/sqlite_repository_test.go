package userrepo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	sqliteinfra "github.com/yanqian/ai-helloworld/internal/infra/sqlite"
)

func TestSQLiteRepositoryPersistsUsersAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	repo := NewSQLiteRepository(db)
	created, err := repo.Create(ctx, "local@example.com", "Local", "hash")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopened := NewSQLiteRepository(db)

	byEmail, found, err := reopened.GetByEmail(ctx, "local@example.com")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, created.ID, byEmail.ID)
	require.Equal(t, "Local", byEmail.Nickname)

	byID, found, err := reopened.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "local@example.com", byID.Email)
}

func TestSQLiteRepositoryPersistsIdentitiesAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	repo := NewSQLiteRepository(db)
	user, err := repo.Create(ctx, "oauth@example.com", "Oauth", "hash")
	require.NoError(t, err)
	identity, err := repo.UpsertIdentity(ctx, auth.Identity{
		UserID:          user.ID,
		Provider:        "google",
		ProviderSubject: "subject-1",
		ProviderEmail:   "oauth@example.com",
		RefreshToken:    "refresh-1",
	})
	require.NoError(t, err)
	require.NotZero(t, identity.ID)
	require.NoError(t, db.Close())

	db, err = sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()
	reopened := NewSQLiteRepository(db)

	byProvider, found, err := reopened.GetIdentity(ctx, "google", "subject-1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "refresh-1", byProvider.RefreshToken)

	byUser, found, err := reopened.GetIdentityByUser(ctx, user.ID, "google")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "subject-1", byUser.ProviderSubject)
}

func TestSQLiteRepositoryParsesDatabaseStyleUserTimestamp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (email, nickname, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, "legacy-time@example.com", "Legacy", "hash", "2025-11-21 14:10:45.570822+00")
	require.NoError(t, err)

	repo := NewSQLiteRepository(db)
	user, found, err := repo.GetByEmail(ctx, "legacy-time@example.com")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "Legacy", user.Nickname)
	require.Equal(t, time.Date(2025, 11, 21, 14, 10, 45, 570822000, time.UTC), user.CreatedAt)
}

func TestSQLiteRepositoryParsesDatabaseStyleIdentityTimestamps(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, nickname, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, 1, "legacy-identity@example.com", "Legacy", "hash", "2025-11-21 14:10:45.570822+00")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, 1, "google", "legacy-subject", "legacy-identity@example.com", "refresh", "2025-11-21 14:10:45.570822+00", "2025-11-21 15:10:45.570822+00")
	require.NoError(t, err)

	repo := NewSQLiteRepository(db)
	identity, found, err := repo.GetIdentity(ctx, "google", "legacy-subject")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "refresh", identity.RefreshToken)
	require.Equal(t, time.Date(2025, 11, 21, 14, 10, 45, 570822000, time.UTC), identity.CreatedAt)
	require.Equal(t, time.Date(2025, 11, 21, 15, 10, 45, 570822000, time.UTC), identity.UpdatedAt)
}

func TestSQLiteRepositoryRejectsInvalidUserTimestamp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")

	db, err := sqliteinfra.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (email, nickname, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, "bad-time@example.com", "BadTime", "hash", "not-a-time")
	require.NoError(t, err)

	repo := NewSQLiteRepository(db)
	_, found, err := repo.GetByEmail(ctx, "bad-time@example.com")
	require.Error(t, err)
	require.False(t, found)
	require.Contains(t, err.Error(), "parse sqlite auth user created_at")
}
