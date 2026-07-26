package notes

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayMissouri/noted/internal/storage"
)

func testService(t *testing.T) (*Service, string) {
	t.Helper()
	sqldb, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if _, err := storage.Migrate(context.Background(), sqldb, storage.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := NewService(sqldb)
	vault, err := s.EnsureDefaultVault(context.Background())
	if err != nil {
		t.Fatalf("EnsureDefaultVault: %v", err)
	}
	return s, vault.ID
}

func versionCount(t *testing.T, s *Service, noteID string) int {
	t.Helper()
	var n int
	err := s.sqldb.QueryRow(`SELECT count(*) FROM note_versions WHERE note_id = ?`, noteID).Scan(&n)
	if err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return n
}

func TestEnsureDefaultVaultIsIdempotent(t *testing.T) {
	s, vaultID := testService(t)
	again, err := s.EnsureDefaultVault(context.Background())
	if err != nil {
		t.Fatalf("second EnsureDefaultVault: %v", err)
	}
	if again.ID != vaultID {
		t.Errorf("second call returned vault %s, want %s", again.ID, vaultID)
	}
}

func TestCreateAndGet(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	created, err := s.Create(ctx, vaultID, "Hello", "# Hello\n", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Version != 1 {
		t.Errorf("Version = %d, want 1", created.Version)
	}
	got, err := s.Get(ctx, created.ID, System)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Body != "# Hello\n" || got.Name != "Hello" {
		t.Errorf("Get = %q %q, want Hello / # Hello", got.Name, got.Body)
	}
	if versionCount(t, s, created.ID) != 1 {
		t.Errorf("version rows = %d, want 1", versionCount(t, s, created.ID))
	}
}

func TestCreateTrimsName(t *testing.T) {
	s, vaultID := testService(t)
	created, err := s.Create(context.Background(), vaultID, "  Padded  ", "", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "Padded" {
		t.Errorf("Name = %q, want Padded", created.Name)
	}
}

func TestCreateDuplicateName(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	if _, err := s.Create(ctx, vaultID, "Twice", "", System); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := s.Create(ctx, vaultID, "Twice", "", System)
	if !errors.Is(err, ErrNameTaken) {
		t.Errorf("err = %v, want ErrNameTaken", err)
	}
}

func TestCreateUnknownVault(t *testing.T) {
	s, _ := testService(t)
	_, err := s.Create(context.Background(), "no-such-vault", "Note", "", System)
	if !errors.Is(err, ErrVaultNotFound) {
		t.Errorf("err = %v, want ErrVaultNotFound", err)
	}
}

func TestValidateName(t *testing.T) {
	bad := []string{
		"", ".", "..", ".hidden", "a/b", `a\b`, "a:b", "a#b", "a^b", "a[b", "a]b", "a|b",
		"ends with dot.", "ends with space ", "ctrl\x01char", strings.Repeat("x", 201),
	}
	for _, name := range bad {
		if err := ValidateName(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("ValidateName(%q) = %v, want ErrInvalidName", name, err)
		}
	}
	good := []string{"Hello", "Meeting notes 2026-07-25", "über cool", "questions?", "50% done"}
	for _, name := range good {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
}

func TestUpdateBumpsVersionAndSnapshots(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	created, err := s.Create(ctx, vaultID, "Draft", "v1", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := s.Update(ctx, created.ID, created.Version, "v2", System)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Version != 2 || updated.Body != "v2" {
		t.Errorf("after update: version %d body %q, want 2 / v2", updated.Version, updated.Body)
	}
	if updated.ChangeSeq <= created.ChangeSeq {
		t.Errorf("change_seq did not advance: %d -> %d", created.ChangeSeq, updated.ChangeSeq)
	}
	if versionCount(t, s, created.ID) != 2 {
		t.Errorf("version rows = %d, want 2", versionCount(t, s, created.ID))
	}
}

func TestUpdateStaleVersionConflicts(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	created, err := s.Create(ctx, vaultID, "Contested", "v1", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Update(ctx, created.ID, created.Version, "v2", System); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	_, err = s.Update(ctx, created.ID, created.Version, "lost write", System)
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("err = %v, want ErrVersionConflict", err)
	}
	got, err := s.Get(ctx, created.ID, System)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Body != "v2" {
		t.Errorf("body = %q after rejected stale write, want v2", got.Body)
	}
}

func TestUpdateMissingNote(t *testing.T) {
	s, _ := testService(t)
	_, err := s.Update(context.Background(), "no-such-note", 1, "", System)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func seedUserAndVault(t *testing.T, s *Service, userID string) (vaultID string) {
	t.Helper()
	now := "2026-07-26T00:00:00Z"
	if _, err := s.sqldb.Exec(`INSERT INTO users (id, username, password_hash, created_at, updated_at) VALUES (?, ?, 'x', ?, ?)`,
		userID, userID, now, now); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	vaultID = "vault-" + userID
	if _, err := s.sqldb.Exec(`INSERT INTO vaults (id, owner_id, name, created_at, updated_at, change_seq) VALUES (?, ?, ?, ?, ?, 0)`,
		vaultID, userID, "vault of "+userID, now, now); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	if _, err := s.sqldb.Exec(`INSERT INTO folders (id, vault_id, name, path, created_at, updated_at, change_seq) VALUES (?, ?, '', '', ?, ?, 0)`,
		"root-"+userID, vaultID, now, now); err != nil {
		t.Fatalf("seed root folder: %v", err)
	}
	return vaultID
}

func actorFor(userID string) Actor {
	return Actor{Kind: KindUser, UserID: &userID}
}

func TestVaultOwnership(t *testing.T) {
	s, defaultVault := testService(t)
	ctx := context.Background()
	vaultA := seedUserAndVault(t, s, "user-a")
	seedUserAndVault(t, s, "user-b")
	alice, bob := actorFor("user-a"), actorFor("user-b")

	note, err := s.Create(ctx, vaultA, "Private", "secret", alice)
	if err != nil {
		t.Fatalf("owner Create: %v", err)
	}

	if _, err := s.List(ctx, vaultA, bob); !errors.Is(err, ErrVaultNotFound) {
		t.Errorf("stranger List err = %v, want ErrVaultNotFound", err)
	}
	if _, err := s.Get(ctx, note.ID, bob); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger Get err = %v, want ErrNotFound", err)
	}
	if _, err := s.Update(ctx, note.ID, note.Version, "defaced", bob); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger Update err = %v, want ErrNotFound", err)
	}
	if _, err := s.Create(ctx, vaultA, "Intruder note", "", bob); !errors.Is(err, ErrVaultNotFound) {
		t.Errorf("stranger Create err = %v, want ErrVaultNotFound", err)
	}
	if got, err := s.Get(ctx, note.ID, alice); err != nil || got.Body != "secret" {
		t.Errorf("owner Get after stranger attempts = %q, %v", got.Body, err)
	}

	admin := Actor{Kind: KindUser, UserID: ptrTo("user-b"), Admin: true}
	if _, err := s.Get(ctx, note.ID, admin); err != nil {
		t.Errorf("admin Get err = %v, want nil", err)
	}

	if _, err := s.List(ctx, defaultVault, bob); err != nil {
		t.Errorf("unowned vault List err = %v, want nil", err)
	}

	vaults, err := s.Vaults(ctx, bob)
	if err != nil {
		t.Fatalf("Vaults: %v", err)
	}
	for _, v := range vaults {
		if v.ID == vaultA {
			t.Errorf("stranger sees vault %s in listing", vaultA)
		}
	}
}

func ptrTo[T any](v T) *T { return &v }

func TestListSortsByName(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	for _, name := range []string{"banana", "Apple", "cherry"} {
		if _, err := s.Create(ctx, vaultID, name, "", System); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}
	rows, err := s.List(ctx, vaultID, System)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var names []string
	for _, r := range rows {
		names = append(names, r.Name)
	}
	want := []string{"Apple", "banana", "cherry"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("List order = %v, want %v", names, want)
	}
}
