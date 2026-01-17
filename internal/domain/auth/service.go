package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/yanqian/ai-helloworld/pkg/errors"
)

// Service exposes authentication workflows.
type Service interface {
	Register(ctx context.Context, req RegisterRequest) (UserView, error)
	Login(ctx context.Context, req LoginRequest) (LoginResponse, error)
	GoogleAuthURL(ctx context.Context, state, codeChallenge string) (string, error)
	GoogleCallback(ctx context.Context, code, codeVerifier string) (LoginResponse, error)
	ValidateToken(ctx context.Context, token string) (Claims, error)
	Refresh(ctx context.Context, refreshToken string) (LoginResponse, error)
	Profile(ctx context.Context, userID int64) (UserView, error)
	Logout(ctx context.Context, userID int64) error
}

type service struct {
	cfg    Config
	repo   Repository
	logger *slog.Logger
}

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// NewService constructs a Service instance.
func NewService(cfg Config, repo Repository, logger *slog.Logger) Service {
	return &service{
		cfg:    cfg,
		repo:   repo,
		logger: logger.With("component", "auth.service"),
	}
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (UserView, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return UserView{}, apperrors.Wrap("invalid_input", "invalid email address", err)
	}
	nickname, err := normalizeNickname(req.Nickname)
	if err != nil {
		return UserView{}, apperrors.Wrap("invalid_input", err.Error(), nil)
	}
	if err := validatePassword(req.Password); err != nil {
		return UserView{}, apperrors.Wrap("invalid_input", err.Error(), nil)
	}
	_, exists, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return UserView{}, apperrors.Wrap("auth_error", "failed to check user", err)
	}
	if exists {
		return UserView{}, apperrors.Wrap("email_exists", "email already registered", nil)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserView{}, apperrors.Wrap("auth_error", "failed to hash password", err)
	}
	user, err := s.repo.Create(ctx, email, nickname, string(hashed))
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			return UserView{}, apperrors.Wrap("email_exists", "email already registered", err)
		}
		return UserView{}, apperrors.Wrap("auth_error", "failed to create user", err)
	}
	return toView(user), nil
}

func (s *service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("invalid_input", "invalid email address", err)
	}
	if strings.TrimSpace(req.Password) == "" {
		return LoginResponse{}, apperrors.Wrap("invalid_input", "password cannot be empty", nil)
	}
	user, found, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to fetch user", err)
	}
	if !found {
		return LoginResponse{}, apperrors.Wrap("invalid_credentials", "invalid email or password", nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return LoginResponse{}, apperrors.Wrap("invalid_credentials", "invalid email or password", nil)
	}
	return s.buildLoginResponse(user)
}

func (s *service) ValidateToken(ctx context.Context, token string) (Claims, error) {
	if strings.TrimSpace(token) == "" {
		return Claims{}, apperrors.Wrap("invalid_token", "token missing", nil)
	}
	claims, err := s.parseToken(token)
	if err != nil {
		return Claims{}, err
	}
	if claims.TokenType != tokenTypeAccess {
		return Claims{}, apperrors.Wrap("invalid_token", "token type mismatch", nil)
	}
	return claims, nil
}

func (s *service) Profile(ctx context.Context, userID int64) (UserView, error) {
	user, found, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return UserView{}, apperrors.Wrap("auth_error", "failed to load profile", err)
	}
	if !found {
		return UserView{}, apperrors.Wrap("user_not_found", "user not found", nil)
	}
	return toView(user), nil
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (LoginResponse, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return LoginResponse{}, err
	}
	if claims.TokenType != tokenTypeRefresh {
		return LoginResponse{}, apperrors.Wrap("invalid_token", "token type mismatch", nil)
	}
	user, found, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return LoginResponse{}, apperrors.Wrap("auth_error", "failed to load user", err)
	}
	if !found {
		return LoginResponse{}, apperrors.Wrap("user_not_found", "user not found", nil)
	}
	return s.buildLoginResponse(user)
}

func (s *service) buildLoginResponse(user User) (LoginResponse, error) {
	access, err := s.generateToken(user, tokenTypeAccess, s.cfg.TokenTTL)
	if err != nil {
		return LoginResponse{}, err
	}
	refresh, err := s.generateToken(user, tokenTypeRefresh, s.cfg.RefreshTokenTTL)
	if err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{
		Token:        access,
		RefreshToken: refresh,
		User:         toView(user),
	}, nil
}

func (s *service) generateToken(user User, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		UserID:    user.ID,
		Email:     user.Email,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			ID:        newTokenID(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", apperrors.Wrap("auth_error", "failed to sign token", err)
	}
	return signed, nil
}

func (s *service) parseToken(token string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &tokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return []byte(s.cfg.Secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return Claims{}, apperrors.Wrap("invalid_token", "token validation failed", err)
	}
	claims, ok := parsed.Claims.(*tokenClaims)
	if !ok || !parsed.Valid {
		return Claims{}, apperrors.Wrap("invalid_token", "token invalid", nil)
	}
	if claims.ExpiresAt == nil {
		return Claims{}, apperrors.Wrap("invalid_token", "token missing expiry", nil)
	}
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return Claims{}, apperrors.Wrap("invalid_token", "token expired", nil)
	}
	return Claims{
		UserID:    claims.UserID,
		Email:     claims.Email,
		TokenType: claims.TokenType,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

func toView(user User) UserView {
	return UserView{
		ID:        user.ID,
		Email:     user.Email,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	}
}

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(raw))
	if email == "" {
		return "", errors.New("email cannot be empty")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", err
	}
	return email, nil
}

func normalizeNickname(raw string) (string, error) {
	nickname := strings.TrimSpace(raw)
	if nickname == "" {
		return "", errors.New("nickname cannot be empty")
	}
	if len([]rune(nickname)) > 10 {
		return "", errors.New("nickname cannot exceed 10 letters")
	}
	for _, r := range nickname {
		if !unicode.IsLetter(r) {
			return "", errors.New("nickname must contain only letters")
		}
	}
	return nickname, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

type tokenClaims struct {
	jwt.RegisteredClaims
	UserID    int64  `json:"userId"`
	Email     string `json:"email"`
	TokenType string `json:"type"`
}

func newTokenID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}
