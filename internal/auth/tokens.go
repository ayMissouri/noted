package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ayMissouri/noted/internal/storage/db"
)

const (
	TokenWeb = "web"
	TokenCLI = "cli"
	TokenPAT = "pat"
	TokenMCP = "mcp"
)

var (
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrInvalidToken       = errors.New("invalid or revoked token")
	ErrTokenNotFound      = errors.New("token not found")
)

const tokenPrefix = "noted_"

var dummyHash = func() string {
	h, err := HashPassword("timing equalizer")
	if err != nil {
		panic(err)
	}
	return h
}()

// Authenticate verifies credentials and creates a device token.
func (s *Service) Authenticate(ctx context.Context, username, password, deviceName, kind string) (string, db.Token, db.User, error) {
	switch kind {
	case TokenWeb, TokenCLI, TokenPAT, TokenMCP:
	default:
		return "", db.Token{}, db.User{}, fmt.Errorf("unknown token kind %q", kind)
	}
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		deviceName = "unnamed device"
	}

	user, err := s.UserByUsername(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		_, _ = VerifyPassword(dummyHash, password)
		return "", db.Token{}, db.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", db.Token{}, db.User{}, err
	}
	ok, err := VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return "", db.Token{}, db.User{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return "", db.Token{}, db.User{}, ErrInvalidCredentials
	}

	secret, hash, err := mintSecret()
	if err != nil {
		return "", db.Token{}, db.User{}, err
	}
	id, err := newID()
	if err != nil {
		return "", db.Token{}, db.User{}, err
	}
	now := timestamp()
	if err := s.q.CreateToken(ctx, db.CreateTokenParams{
		ID: id, UserID: user.ID, Kind: kind, Name: deviceName,
		TokenHash: hash, Scopes: "", CreatedAt: now, LastSeenAt: &now,
	}); err != nil {
		return "", db.Token{}, db.User{}, fmt.Errorf("create token: %w", err)
	}
	token, err := s.q.GetTokenByHash(ctx, hash)
	if err != nil {
		return "", db.Token{}, db.User{}, fmt.Errorf("create token: %w", err)
	}
	return secret, token, user, nil
}

// ValidateToken resolves a given secret to its token and user.
func (s *Service) ValidateToken(ctx context.Context, secret string) (db.Token, db.User, error) {
	if !strings.HasPrefix(secret, tokenPrefix) {
		return db.Token{}, db.User{}, ErrInvalidToken
	}
	token, err := s.q.GetTokenByHash(ctx, hashSecret(secret))
	if errors.Is(err, sql.ErrNoRows) {
		return db.Token{}, db.User{}, ErrInvalidToken
	}
	if err != nil {
		return db.Token{}, db.User{}, fmt.Errorf("look up token: %w", err)
	}
	if token.ExpiresAt != nil {
		exp, err := time.Parse(time.RFC3339, *token.ExpiresAt)
		if err != nil || !time.Now().UTC().Before(exp) {
			return db.Token{}, db.User{}, ErrInvalidToken
		}
	}
	user, err := s.userByID(ctx, token.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return db.Token{}, db.User{}, ErrInvalidToken
	}
	if err != nil {
		return db.Token{}, db.User{}, err
	}
	s.touchToken(ctx, token)
	return token, user, nil
}

func (s *Service) touchToken(ctx context.Context, token db.Token) {
	now := time.Now().UTC()
	if token.LastSeenAt != nil {
		if seen, err := time.Parse(time.RFC3339, *token.LastSeenAt); err == nil && now.Sub(seen) < time.Minute {
			return
		}
	}
	_ = s.q.TouchToken(ctx, db.TouchTokenParams{LastSeenAt: ptr(now.Format(time.RFC3339)), ID: token.ID})
}

func (s *Service) Tokens(ctx context.Context, userID string) ([]db.Token, error) {
	tokens, err := s.q.ListUserTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	return tokens, nil
}

func (s *Service) RevokeToken(ctx context.Context, userID, tokenID string) error {
	affected, err := s.q.RevokeToken(ctx, db.RevokeTokenParams{
		RevokedAt: ptr(timestamp()), ID: tokenID, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	if affected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func mintSecret() (secret, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	secret = tokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return secret, hashSecret(secret), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func ptr[T any](v T) *T { return &v }
