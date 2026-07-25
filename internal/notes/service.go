package notes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/ahmedmissouri/noted/internal/storage/db"
)

var (
	ErrNotFound        = errors.New("note not found")
	ErrVaultNotFound   = errors.New("vault not found")
	ErrVersionConflict = errors.New("note changed since the version you read")
	ErrNameTaken       = errors.New("a note with that name already exists here")
	ErrInvalidName     = errors.New("invalid note name")
)

const (
	KindUser   = "user"
	KindAgent  = "agent"
	KindSystem = "system"
	KindImport = "import"
)

type Actor struct {
	Kind    string
	UserID  *string
	TokenID *string
}

var System = Actor{Kind: KindSystem}

type Service struct {
	sqldb *sql.DB
	q     *db.Queries
}

func NewService(sqldb *sql.DB) *Service {
	return &Service{sqldb: sqldb, q: db.New(sqldb)}
}

// EnsureDefaultVault returns the first vault, creating one with a root folder if none exist.
func (s *Service) EnsureDefaultVault(ctx context.Context) (db.Vault, error) {
	vaults, err := s.q.ListVaults(ctx)
	if err != nil {
		return db.Vault{}, fmt.Errorf("list vaults: %w", err)
	}
	if len(vaults) > 0 {
		return vaults[0], nil
	}

	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return db.Vault{}, fmt.Errorf("create default vault: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	now := timestamp()
	vaultID, err := newID()
	if err != nil {
		return db.Vault{}, err
	}
	seq, err := qtx.NextChangeSeq(ctx)
	if err != nil {
		return db.Vault{}, fmt.Errorf("create default vault: %w", err)
	}
	if err := qtx.CreateVault(ctx, db.CreateVaultParams{
		ID: vaultID, Name: "Notes", CreatedAt: now, UpdatedAt: now, ChangeSeq: seq,
	}); err != nil {
		return db.Vault{}, fmt.Errorf("create default vault: %w", err)
	}

	folderID, err := newID()
	if err != nil {
		return db.Vault{}, err
	}
	seq, err = qtx.NextChangeSeq(ctx)
	if err != nil {
		return db.Vault{}, fmt.Errorf("create root folder: %w", err)
	}
	if err := qtx.CreateFolder(ctx, db.CreateFolderParams{
		ID: folderID, VaultID: vaultID, Name: "", Path: "", CreatedAt: now, UpdatedAt: now, ChangeSeq: seq,
	}); err != nil {
		return db.Vault{}, fmt.Errorf("create root folder: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return db.Vault{}, fmt.Errorf("create default vault: %w", err)
	}
	return db.Vault{ID: vaultID, Name: "Notes", CreatedAt: now, UpdatedAt: now, ChangeSeq: seq}, nil
}

func (s *Service) Vaults(ctx context.Context) ([]db.Vault, error) {
	vaults, err := s.q.ListVaults(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vaults: %w", err)
	}
	return vaults, nil
}

func (s *Service) List(ctx context.Context, vaultID string) ([]db.ListNotesRow, error) {
	rows, err := s.q.ListNotes(ctx, vaultID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	return rows, nil
}

func (s *Service) Get(ctx context.Context, id string) (db.Note, error) {
	note, err := s.q.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, ErrNotFound
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("get note: %w", err)
	}
	return note, nil
}

func (s *Service) Create(ctx context.Context, vaultID, name, body string, actor Actor) (db.Note, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return db.Note{}, err
	}

	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	root, err := qtx.GetFolderByPath(ctx, db.GetFolderByPathParams{VaultID: vaultID, Path: ""})
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, ErrVaultNotFound
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}

	id, err := newID()
	if err != nil {
		return db.Note{}, err
	}
	seq, err := qtx.NextChangeSeq(ctx)
	if err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}
	now := timestamp()
	err = qtx.CreateNote(ctx, db.CreateNoteParams{
		ID: id, VaultID: vaultID, FolderID: root.ID, Name: name, Body: body,
		CreatedAt: now, UpdatedAt: now,
		UpdatedByKind: actor.Kind, UpdatedByUser: actor.UserID, UpdatedByToken: actor.TokenID,
		ChangeSeq: seq,
	})
	if isUniqueViolation(err) {
		return db.Note{}, ErrNameTaken
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}
	if err := qtx.SnapshotNoteVersion(ctx, db.SnapshotNoteVersionParams{
		SavedAt: now, ActorKind: actor.Kind, ActorUser: actor.UserID, ActorToken: actor.TokenID, ID: id,
	}); err != nil {
		return db.Note{}, fmt.Errorf("snapshot note version: %w", err)
	}

	note, err := qtx.GetNote(ctx, id)
	if err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return db.Note{}, fmt.Errorf("create note: %w", err)
	}
	return note, nil
}

// Update writes a new body if baseVersion matches the stored version.
func (s *Service) Update(ctx context.Context, id string, baseVersion int64, body string, actor Actor) (db.Note, error) {
	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	seq, err := qtx.NextChangeSeq(ctx)
	if err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	now := timestamp()
	affected, err := qtx.UpdateNoteBody(ctx, db.UpdateNoteBodyParams{
		Body: body, UpdatedAt: now,
		UpdatedByKind: actor.Kind, UpdatedByUser: actor.UserID, UpdatedByToken: actor.TokenID,
		ChangeSeq: seq, ID: id, Version: baseVersion,
	})
	if err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	if affected == 0 {
		if _, err := qtx.GetNote(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return db.Note{}, ErrNotFound
		} else if err != nil {
			return db.Note{}, fmt.Errorf("update note: %w", err)
		}
		return db.Note{}, ErrVersionConflict
	}
	if err := qtx.SnapshotNoteVersion(ctx, db.SnapshotNoteVersionParams{
		SavedAt: now, ActorKind: actor.Kind, ActorUser: actor.UserID, ActorToken: actor.TokenID, ID: id,
	}); err != nil {
		return db.Note{}, fmt.Errorf("snapshot note version: %w", err)
	}

	note, err := qtx.GetNote(ctx, id)
	if err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	return note, nil
}

func ValidateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	case len(name) > 200:
		return fmt.Errorf("%w: name longer than 200 characters", ErrInvalidName)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("%w: name starts with a dot", ErrInvalidName)
	case strings.HasSuffix(name, "."), strings.HasSuffix(name, " "):
		return fmt.Errorf("%w: name ends with a dot or space", ErrInvalidName)
	}
	if i := strings.IndexAny(name, "/\\:#^[]|"); i >= 0 {
		return fmt.Errorf("%w: name contains %q", ErrInvalidName, name[i])
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: name contains a control character", ErrInvalidName)
		}
	}
	return nil
}

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return id.String(), nil
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
