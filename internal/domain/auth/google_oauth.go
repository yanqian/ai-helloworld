package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

const (
	googleProviderName = "google"
	googleIssuerURL    = "https://accounts.google.com"
)

type googleClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

func (s *service) GoogleAuthURL(ctx context.Context, state, codeChallenge string) (string, error) {
	cfg, err := s.googleOAuthConfig()
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("access_type", "offline"),
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	return cfg.AuthCodeURL(state, opts...), nil
}

func (s *service) GoogleCallback(ctx context.Context, code, codeVerifier string) (LoginResponse, error) {
	cfg, err := s.googleOAuthConfig()
	if err != nil {
		return LoginResponse{}, err
	}
	if strings.TrimSpace(code) == "" || strings.TrimSpace(codeVerifier) == "" {
		return LoginResponse{}, apperrors.Wrap("invalid_request", "missing oauth code or verifier", nil)
	}
	token, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("oauth_exchange_failed", "failed to exchange oauth code", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return LoginResponse{}, apperrors.Wrap("oauth_exchange_failed", "missing id_token in oauth response", nil)
	}
	claims, err := s.verifyGoogleIDToken(ctx, rawIDToken)
	if err != nil {
		return LoginResponse{}, err
	}
	if !claims.EmailVerified {
		return LoginResponse{}, apperrors.Wrap("invalid_credentials", "google account email not verified", nil)
	}
	email, err := normalizeEmail(claims.Email)
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("invalid_input", "invalid email address", err)
	}
	if claims.Subject == "" {
		return LoginResponse{}, apperrors.Wrap("auth_error", "missing google subject", nil)
	}

	identity, found, err := s.repo.GetIdentity(ctx, googleProviderName, claims.Subject)
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to fetch identity", err)
	}
	if found {
		user, ok, err := s.repo.GetByID(ctx, identity.UserID)
		if err != nil {
			return LoginResponse{}, apperrors.Wrap("auth_error", "failed to load user", err)
		}
		if !ok {
			return LoginResponse{}, apperrors.Wrap("user_not_found", "user not found", nil)
		}
		if token.RefreshToken != "" {
			if err := s.upsertGoogleIdentity(ctx, identity.UserID, claims, token.RefreshToken); err != nil {
				return LoginResponse{}, err
			}
		}
		return s.buildLoginResponse(user)
	}

	if _, exists, err := s.repo.GetByEmail(ctx, email); err != nil {
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to check existing user", err)
	} else if exists {
		return LoginResponse{}, apperrors.Wrap("account_linking_disabled", "account linking by email is not enabled", nil)
	}

	nickname := googleNickname(claims)
	passwordHash, err := hashRandomPassword()
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to generate password hash", err)
	}
	user, err := s.repo.Create(ctx, email, nickname, passwordHash)
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			return LoginResponse{}, apperrors.Wrap("email_exists", "email already registered", err)
		}
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to create user", err)
	}

	if err := s.upsertGoogleIdentity(ctx, user.ID, claims, token.RefreshToken); err != nil {
		return LoginResponse{}, err
	}
	return s.buildLoginResponse(user)
}

func (s *service) Logout(ctx context.Context, userID int64) error {
	identity, found, err := s.repo.GetIdentityByUser(ctx, userID, googleProviderName)
	if err != nil {
		return apperrors.Wrap("auth_error", "failed to fetch identity", err)
	}
	if !found || identity.RefreshToken == "" {
		return nil
	}
	refreshToken, err := decryptToken(s.cfg.Google.TokenEncryptionKey, identity.RefreshToken)
	if err != nil {
		s.logger.Warn("failed to decrypt google refresh token", "error", err)
		return nil
	}
	if refreshToken == "" {
		return nil
	}
	if err := revokeGoogleToken(ctx, refreshToken); err != nil {
		s.logger.Warn("failed to revoke google refresh token", "error", err)
		return nil
	}
	return nil
}

func (s *service) googleOAuthConfig() (*oauth2.Config, error) {
	googleCfg := s.cfg.Google
	if strings.TrimSpace(googleCfg.ClientID) == "" || strings.TrimSpace(googleCfg.ClientSecret) == "" || strings.TrimSpace(googleCfg.RedirectURL) == "" {
		return nil, apperrors.Wrap("auth_not_configured", "google oauth is not configured", nil)
	}
	if strings.TrimSpace(googleCfg.TokenEncryptionKey) == "" {
		return nil, apperrors.Wrap("auth_not_configured", "google token encryption key is missing", nil)
	}
	return &oauth2.Config{
		ClientID:     googleCfg.ClientID,
		ClientSecret: googleCfg.ClientSecret,
		RedirectURL:  googleCfg.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}, nil
}

func (s *service) verifyGoogleIDToken(ctx context.Context, rawToken string) (googleClaims, error) {
	provider, err := oidc.NewProvider(ctx, googleIssuerURL)
	if err != nil {
		return googleClaims{}, apperrors.Wrap("auth_error", "failed to initialize oidc provider", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: s.cfg.Google.ClientID})
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return googleClaims{}, apperrors.Wrap("invalid_token", "failed to verify id token", err)
	}
	var claims googleClaims
	if err := idToken.Claims(&claims); err != nil {
		return googleClaims{}, apperrors.Wrap("invalid_token", "failed to parse id token claims", err)
	}
	if claims.Email == "" {
		return googleClaims{}, apperrors.Wrap("invalid_token", "missing email in id token", nil)
	}
	return claims, nil
}

func (s *service) upsertGoogleIdentity(ctx context.Context, userID int64, claims googleClaims, refreshToken string) error {
	encoded := ""
	if refreshToken != "" {
		ciphertext, err := encryptToken(s.cfg.Google.TokenEncryptionKey, refreshToken)
		if err != nil {
			return apperrors.Wrap("auth_error", "failed to encrypt refresh token", err)
		}
		encoded = ciphertext
	}
	_, err := s.repo.UpsertIdentity(ctx, Identity{
		UserID:          userID,
		Provider:        googleProviderName,
		ProviderSubject: claims.Subject,
		ProviderEmail:   claims.Email,
		RefreshToken:    encoded,
	})
	if err != nil {
		return apperrors.Wrap("auth_error", "failed to persist identity", err)
	}
	return nil
}

func googleNickname(claims googleClaims) string {
	candidate := strings.TrimSpace(claims.GivenName)
	if candidate == "" {
		candidate = strings.TrimSpace(claims.Name)
	}
	if candidate == "" {
		candidate = strings.Split(claims.Email, "@")[0]
	}
	builder := strings.Builder{}
	count := 0
	for _, r := range candidate {
		if count >= 10 {
			break
		}
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			builder.WriteRune(r)
			count++
		}
	}
	name := builder.String()
	if name == "" {
		name = "User"
	}
	if normalized, err := normalizeNickname(name); err == nil {
		return normalized
	}
	return "User"
}

func hashRandomPassword() (string, error) {
	raw, err := randomString(32)
	if err != nil {
		return "", err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func randomString(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// CodeChallengeFromVerifier computes the PKCE code challenge for a verifier.
func CodeChallengeFromVerifier(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func revokeGoogleToken(ctx context.Context, refreshToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/revoke", strings.NewReader("token="+refreshToken))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("google revoke returned status %d", resp.StatusCode)
}

// NewOAuthState returns a state, code verifier, and code challenge for PKCE.
func NewOAuthState() (state string, codeVerifier string, codeChallenge string, err error) {
	state, err = randomString(32)
	if err != nil {
		return "", "", "", err
	}
	codeVerifier, err = randomString(32)
	if err != nil {
		return "", "", "", err
	}
	codeChallenge = CodeChallengeFromVerifier(codeVerifier)
	return state, codeVerifier, codeChallenge, nil
}
