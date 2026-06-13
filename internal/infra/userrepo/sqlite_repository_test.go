package userrepo

import (
	"context"
	"path/filepath"
	"testing"

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
