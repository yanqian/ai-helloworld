package userrepo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
)

// PostgresRepository persists users in Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Create inserts a new user row.
func (r *PostgresRepository) Create(ctx context.Context, email, nickname, passwordHash string) (auth.User, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, nickname, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, nickname, password_hash, created_at
	`, email, nickname, passwordHash)
	user, err := scanUser(row)
	if err != nil {
		if isDuplicateError(err) {
			return auth.User{}, auth.ErrEmailExists
		}
		return auth.User{}, err
	}
	return user, nil
}

// GetByEmail fetches a user by email.
func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (auth.User, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, nickname, password_hash, created_at
		FROM users
		WHERE email = $1
		LIMIT 1
	`, email)
	if err != nil {
		return auth.User{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return auth.User{}, false, rows.Err()
	}
	user, err := scanUser(rows)
	if err != nil {
		return auth.User{}, false, err
	}
	return user, true, rows.Err()
}

// GetByID fetches by primary key.
func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (auth.User, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, nickname, password_hash, created_at
		FROM users
		WHERE id = $1
		LIMIT 1
	`, id)
	if err != nil {
		return auth.User{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return auth.User{}, false, rows.Err()
	}
	user, err := scanUser(rows)
	if err != nil {
		return auth.User{}, false, err
	}
	return user, true, rows.Err()
}

// GetIdentity fetches an identity by provider + subject.
func (r *PostgresRepository) GetIdentity(ctx context.Context, provider, providerSubject string) (auth.Identity, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at
		FROM user_identities
		WHERE provider = $1 AND provider_subject = $2
		LIMIT 1
	`, provider, providerSubject)
	if err != nil {
		return auth.Identity{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return auth.Identity{}, false, rows.Err()
	}
	identity, err := scanIdentity(rows)
	if err != nil {
		return auth.Identity{}, false, err
	}
	return identity, true, rows.Err()
}

// GetIdentityByUser fetches an identity for a user and provider.
func (r *PostgresRepository) GetIdentityByUser(ctx context.Context, userID int64, provider string) (auth.Identity, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at
		FROM user_identities
		WHERE user_id = $1 AND provider = $2
		LIMIT 1
	`, userID, provider)
	if err != nil {
		return auth.Identity{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return auth.Identity{}, false, rows.Err()
	}
	identity, err := scanIdentity(rows)
	if err != nil {
		return auth.Identity{}, false, err
	}
	return identity, true, rows.Err()
}

// UpsertIdentity inserts or updates an external identity mapping.
func (r *PostgresRepository) UpsertIdentity(ctx context.Context, identity auth.Identity) (auth.Identity, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_subject, provider_email, refresh_token)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT (provider, provider_subject)
		DO UPDATE SET
			provider_email = EXCLUDED.provider_email,
			refresh_token = COALESCE(EXCLUDED.refresh_token, user_identities.refresh_token),
			updated_at = NOW()
		RETURNING id, user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at
	`, identity.UserID, identity.Provider, identity.ProviderSubject, identity.ProviderEmail, identity.RefreshToken)
	updated, err := scanIdentity(row)
	if err != nil {
		return auth.Identity{}, err
	}
	return updated, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (auth.User, error) {
	var user auth.User
	var created time.Time
	if err := row.Scan(&user.ID, &user.Email, &user.Nickname, &user.PasswordHash, &created); err != nil {
		return auth.User{}, err
	}
	user.CreatedAt = created.UTC()
	return user, nil
}

func scanIdentity(row rowScanner) (auth.Identity, error) {
	var identity auth.Identity
	var created time.Time
	var updated time.Time
	if err := row.Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderSubject,
		&identity.ProviderEmail,
		&identity.RefreshToken,
		&created,
		&updated,
	); err != nil {
		return auth.Identity{}, err
	}
	identity.CreatedAt = created.UTC()
	identity.UpdatedAt = updated.UTC()
	return identity, nil
}

var _ auth.Repository = (*PostgresRepository)(nil)

func isDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
