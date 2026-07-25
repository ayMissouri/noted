package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayMissouri/noted/internal/storage"
)

func testService(t *testing.T) *Service {
	t.Helper()
	sqldb, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if _, err := storage.Migrate(context.Background(), sqldb, storage.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewService(sqldb)
}

func TestCreateUserAndLookup(t *testing.T) {
	s := testService(t)
	ctx := context.Background()

	n, err := s.UserCount(ctx)
	if err != nil || n != 0 {
		t.Fatalf("UserCount on fresh db = %d, %v; want 0, nil", n, err)
	}

	u, err := s.CreateUser(ctx, "ahmed", "a@example.com", "long enough password", true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.IsAdmin != 1 || u.Username != "ahmed" || u.Email == nil || *u.Email != "a@example.com" {
		t.Errorf("user = %+v, want admin ahmed a@example.com", u)
	}
	if u.PasswordHash == "long enough password" || !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
		t.Errorf("password stored badly: %q", u.PasswordHash)
	}

	got, err := s.UserByUsername(ctx, "ahmed")
	if err != nil || got.ID != u.ID {
		t.Errorf("UserByUsername = %+v, %v; want the created user", got, err)
	}

	if n, _ := s.UserCount(ctx); n != 1 {
		t.Errorf("UserCount = %d, want 1", n)
	}
}

func TestCreateUserEmailOptional(t *testing.T) {
	s := testService(t)
	u, err := s.CreateUser(context.Background(), "no-email", "", "long enough password", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Email != nil {
		t.Errorf("Email = %v, want nil", *u.Email)
	}
}

func TestCreateUserRejectsDuplicates(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "alice", "alice@example.com", "long enough password", false); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, err := s.CreateUser(ctx, "Alice", "other@example.com", "long enough password", false)
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("case-insensitive username dup err = %v, want ErrUsernameTaken", err)
	}
	_, err = s.CreateUser(ctx, "bob", "Alice@Example.com", "long enough password", false)
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("case-insensitive email dup err = %v, want ErrEmailTaken", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	s := testService(t)
	ctx := context.Background()

	for _, name := range []string{"", "ab", "has space", "über", "trailing/", strings.Repeat("x", 33), ".leading"} {
		if _, err := s.CreateUser(ctx, name, "", "long enough password", false); !errors.Is(err, ErrInvalidUsername) {
			t.Errorf("username %q err = %v, want ErrInvalidUsername", name, err)
		}
	}
	if _, err := s.CreateUser(ctx, "valid", "not-an-email", "long enough password", false); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("bad email err = %v, want ErrInvalidEmail", err)
	}
	if _, err := s.CreateUser(ctx, "valid", "", "short", false); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("short password err = %v, want ErrWeakPassword", err)
	}
	if _, err := s.CreateUser(ctx, "valid", "", strings.Repeat("x", 513), false); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("huge password err = %v, want ErrWeakPassword", err)
	}
}

func TestCreateFirstAdmin(t *testing.T) {
	s := testService(t)
	ctx := context.Background()

	u, err := s.CreateFirstAdmin(ctx, "admin", "", "long enough password")
	if err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	if u.IsAdmin != 1 {
		t.Error("first admin is not an admin")
	}

	_, err = s.CreateFirstAdmin(ctx, "second", "", "long enough password")
	if !errors.Is(err, ErrSetupComplete) {
		t.Errorf("second CreateFirstAdmin err = %v, want ErrSetupComplete", err)
	}
	if n, _ := s.UserCount(ctx); n != 1 {
		t.Errorf("UserCount = %d, want 1", n)
	}
}

func TestCreateFirstAdminValidatesBeforeCounting(t *testing.T) {
	s := testService(t)
	if _, err := s.CreateFirstAdmin(context.Background(), "x", "", "short"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("err = %v, want ErrInvalidUsername", err)
	}
	if n, _ := s.UserCount(context.Background()); n != 0 {
		t.Errorf("UserCount = %d after failed setup, want 0", n)
	}
}

func TestUserByUsernameMissing(t *testing.T) {
	s := testService(t)
	if _, err := s.UserByUsername(context.Background(), "ghost"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("err = %v, want ErrUserNotFound", err)
	}
}
