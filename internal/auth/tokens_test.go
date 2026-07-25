package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func seedUser(t *testing.T, s *Service) {
	t.Helper()
	if _, err := s.CreateUser(context.Background(), "alice", "", "long enough password", false); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func TestAuthenticateAndValidate(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	ctx := context.Background()

	secret, token, user, err := s.Authenticate(ctx, "alice", "long enough password", "Firefox on desk", TokenWeb)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !strings.HasPrefix(secret, "noted_") {
		t.Errorf("secret = %q, want noted_ prefix", secret)
	}
	if strings.Contains(token.TokenHash, secret) || token.TokenHash == secret {
		t.Error("token stored in plaintext")
	}
	if token.Kind != "web" || token.Name != "Firefox on desk" || user.Username != "alice" {
		t.Errorf("token = %+v user = %s", token, user.Username)
	}

	gotToken, gotUser, err := s.ValidateToken(ctx, secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if gotToken.ID != token.ID || gotUser.ID != user.ID {
		t.Error("ValidateToken resolved a different token or user")
	}
}

func TestAuthenticateRejectsBadCredentials(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	ctx := context.Background()

	if _, _, _, err := s.Authenticate(ctx, "alice", "wrong password!", "d", TokenWeb); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
	if _, _, _, err := s.Authenticate(ctx, "nobody", "long enough password", "d", TokenWeb); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user err = %v, want ErrInvalidCredentials", err)
	}
	if _, _, _, err := s.Authenticate(ctx, "alice", "long enough password", "d", "starship"); err == nil {
		t.Error("unknown token kind accepted")
	}
}

func TestValidateTokenRejectsGarbage(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	for _, secret := range []string{"", "noted_", "noted_notreal", "Bearer xyz"} {
		if _, _, err := s.ValidateToken(context.Background(), secret); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("ValidateToken(%q) err = %v, want ErrInvalidToken", secret, err)
		}
	}
}

func TestRevokeToken(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	ctx := context.Background()

	secret, token, user, err := s.Authenticate(ctx, "alice", "long enough password", "laptop", TokenWeb)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if err := s.RevokeToken(ctx, user.ID, token.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if _, _, err := s.ValidateToken(ctx, secret); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("revoked token err = %v, want ErrInvalidToken", err)
	}
	if err := s.RevokeToken(ctx, user.ID, token.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("double revoke err = %v, want ErrTokenNotFound", err)
	}
	tokens, err := s.Tokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("Tokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("Tokens after revoke = %d entries, want 0", len(tokens))
	}
}

func TestRevokeTokenWrongUser(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	ctx := context.Background()

	secret, token, _, err := s.Authenticate(ctx, "alice", "long enough password", "laptop", TokenWeb)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if err := s.RevokeToken(ctx, "someone-else", token.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Errorf("cross-user revoke err = %v, want ErrTokenNotFound", err)
	}
	if _, _, err := s.ValidateToken(ctx, secret); err != nil {
		t.Errorf("token invalid after failed cross-user revoke: %v", err)
	}
}

func TestValidateTokenRejectsExpired(t *testing.T) {
	s := testService(t)
	seedUser(t, s)
	ctx := context.Background()

	secret, token, _, err := s.Authenticate(ctx, "alice", "long enough password", "laptop", TokenWeb)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if _, err := s.sqldb.Exec(`UPDATE tokens SET expires_at = '2020-01-01T00:00:00Z' WHERE id = ?`, token.ID); err != nil {
		t.Fatalf("expire token: %v", err)
	}
	if _, _, err := s.ValidateToken(ctx, secret); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired token err = %v, want ErrInvalidToken", err)
	}
}
