package auth

import "context"

// Repository abstracts user persistence.
type Repository interface {
	Create(ctx context.Context, email, nickname, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, bool, error)
	GetByID(ctx context.Context, id int64) (User, bool, error)
	GetIdentity(ctx context.Context, provider, providerSubject string) (Identity, bool, error)
	GetIdentityByUser(ctx context.Context, userID int64, provider string) (Identity, bool, error)
	UpsertIdentity(ctx context.Context, identity Identity) (Identity, error)
}
