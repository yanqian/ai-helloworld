package userrepo

import (
	"context"
	"time"

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
	return scanUser(row)
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

var _ auth.Repository = (*PostgresRepository)(nil)
