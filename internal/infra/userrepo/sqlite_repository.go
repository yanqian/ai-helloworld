package userrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
)

// SQLiteRepository persists users and external identities in SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository constructs a SQLite-backed auth repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Create stores the user record.
func (r *SQLiteRepository) Create(ctx context.Context, email, nickname, passwordHash string) (auth.User, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO users (email, nickname, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, email, nickname, passwordHash, now.Format(time.RFC3339Nano))
	if err != nil {
		if isSQLiteUniqueViolation(err) {
			return auth.User{}, auth.ErrEmailExists
		}
		return auth.User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{
		ID:           id,
		Email:        email,
		Nickname:     nickname,
		PasswordHash: passwordHash,
		CreatedAt:    now,
	}, nil
}

// GetByEmail returns a user by email.
func (r *SQLiteRepository) GetByEmail(ctx context.Context, email string) (auth.User, bool, error) {
	return sqliteScanUser(r.db.QueryRowContext(ctx, `
		SELECT id, email, nickname, password_hash, created_at
		FROM users
		WHERE email = ?
	`, email))
}

// GetByID fetches by ID.
func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (auth.User, bool, error) {
	return sqliteScanUser(r.db.QueryRowContext(ctx, `
		SELECT id, email, nickname, password_hash, created_at
		FROM users
		WHERE id = ?
	`, id))
}

// GetIdentity returns an identity by provider and subject.
func (r *SQLiteRepository) GetIdentity(ctx context.Context, provider, providerSubject string) (auth.Identity, bool, error) {
	return sqliteScanIdentity(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at
		FROM user_identities
		WHERE provider = ? AND provider_subject = ?
	`, provider, providerSubject))
}

// GetIdentityByUser returns an identity by user and provider.
func (r *SQLiteRepository) GetIdentityByUser(ctx context.Context, userID int64, provider string) (auth.Identity, bool, error) {
	return sqliteScanIdentity(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at
		FROM user_identities
		WHERE user_id = ? AND provider = ?
	`, userID, provider))
}

// UpsertIdentity stores or updates the identity mapping.
func (r *SQLiteRepository) UpsertIdentity(ctx context.Context, identity auth.Identity) (auth.Identity, error) {
	if identity.UserID == 0 {
		return auth.Identity{}, errors.New("userID is required")
	}
	now := time.Now().UTC()
	existing, found, err := r.GetIdentity(ctx, identity.Provider, identity.ProviderSubject)
	if err != nil {
		return auth.Identity{}, err
	}
	if found {
		if identity.RefreshToken != "" {
			existing.RefreshToken = identity.RefreshToken
		}
		if identity.ProviderEmail != "" {
			existing.ProviderEmail = identity.ProviderEmail
		}
		existing.UpdatedAt = now
		_, err := r.db.ExecContext(ctx, `
			UPDATE user_identities
			SET provider_email = ?, refresh_token = ?, updated_at = ?
			WHERE id = ?
		`, existing.ProviderEmail, existing.RefreshToken, existing.UpdatedAt.Format(time.RFC3339Nano), existing.ID)
		return existing, err
	}

	identity.CreatedAt = now
	identity.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_subject, provider_email, refresh_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, identity.UserID, identity.Provider, identity.ProviderSubject, identity.ProviderEmail, identity.RefreshToken, identity.CreatedAt.Format(time.RFC3339Nano), identity.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return auth.Identity{}, err
	}
	identity.ID, err = res.LastInsertId()
	if err != nil {
		return auth.Identity{}, err
	}
	return identity, nil
}

func sqliteScanUser(row *sql.Row) (auth.User, bool, error) {
	var user auth.User
	var created string
	if err := row.Scan(&user.ID, &user.Email, &user.Nickname, &user.PasswordHash, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, false, nil
		}
		return auth.User{}, false, err
	}
	parsed, err := parseSQLiteAuthTime(created)
	if err != nil {
		return auth.User{}, false, fmt.Errorf("parse sqlite auth user created_at %q: %w", created, err)
	}
	user.CreatedAt = parsed
	return user, true, nil
}

func sqliteScanIdentity(row *sql.Row) (auth.Identity, bool, error) {
	var identity auth.Identity
	var created string
	var updated string
	if err := row.Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.ProviderSubject, &identity.ProviderEmail, &identity.RefreshToken, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Identity{}, false, nil
		}
		return auth.Identity{}, false, err
	}
	var err error
	identity.CreatedAt, err = parseSQLiteAuthTime(created)
	if err != nil {
		return auth.Identity{}, false, fmt.Errorf("parse sqlite auth identity created_at %q: %w", created, err)
	}
	identity.UpdatedAt, err = parseSQLiteAuthTime(updated)
	if err != nil {
		return auth.Identity{}, false, fmt.Errorf("parse sqlite auth identity updated_at %q: %w", updated, err)
	}
	return identity, true, nil
}

func parseSQLiteAuthTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999-0700",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}

func isSQLiteUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint failed")
}

var _ auth.Repository = (*SQLiteRepository)(nil)
