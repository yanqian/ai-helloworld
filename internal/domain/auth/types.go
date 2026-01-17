package auth

import "time"

// Config drives authentication behavior.
type Config struct {
	Secret          string
	TokenTTL        time.Duration
	RefreshTokenTTL time.Duration
	Google          GoogleConfig
}

// GoogleConfig holds OAuth settings for Google sign-in.
type GoogleConfig struct {
	ClientID             string
	ClientSecret         string
	RedirectURL          string
	TokenEncryptionKey   string
	PostLoginRedirectURL string
}

// User represents a persisted account.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Nickname     string    `json:"nickname"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Identity represents an external auth provider linkage.
type Identity struct {
	ID              int64
	UserID          int64
	Provider        string
	ProviderSubject string
	ProviderEmail   string
	RefreshToken    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// RegisterRequest captures the registration payload.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

// LoginRequest captures login details.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse returns the signed token.
type LoginResponse struct {
	Token        string   `json:"token"`
	RefreshToken string   `json:"refreshToken"`
	User         UserView `json:"user"`
}

// UserView trims sensitive fields.
type UserView struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"createdAt"`
}

// Claims are extracted from the JWT token.
type Claims struct {
	UserID    int64
	Email     string
	TokenType string
	ExpiresAt time.Time
}

// RefreshRequest encapsulates refresh token payload.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}
