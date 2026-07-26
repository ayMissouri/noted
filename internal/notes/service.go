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

	"github.com/ayMissouri/noted/internal/storage/db"
)

var (
	ErrNotFound         = errors.New("note not found")
	ErrVaultNotFound    = errors.New("vault not found")
	ErrVersionConflict  = errors.New("note changed since the version you read")
	ErrNameTaken        = errors.New("a note with that name already exists here")
	ErrInvalidName      = errors.New("invalid note name")
	ErrInvalidVaultName = errors.New("vault names are 1 to 100 characters")
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
	Admin   bool
}

var System = Actor{Kind: KindSystem}

type Service struct {
	sqldb *sql.DB
	q     *db.Queries
}

func NewService(sqldb *sql.DB) *Service {
	return &Service{sqldb: sqldb, q: db.New(sqldb)}
}

// EnsureDefaultVault returns the first vault, creating an unowned one if none exist.
func (s *Service) EnsureDefaultVault(ctx context.Context) (db.Vault, error) {
	vaults, err := s.q.ListVaults(ctx)
	if err != nil {
		return db.Vault{}, fmt.Errorf("list vaults: %w", err)
	}
	if len(vaults) > 0 {
		return vaults[0], nil
	}
	return s.createVault(ctx, "Notes", nil)
}

func (s *Service) CreateVault(ctx context.Context, name string, actor Actor) (db.Vault, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return db.Vault{}, ErrInvalidVaultName
	}
	return s.createVault(ctx, name, actor.UserID)
}

func (s *Service) createVault(ctx context.Context, name string, owner *string) (db.Vault, error) {
	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return db.Vault{}, fmt.Errorf("create vault: %w", err)
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
		return db.Vault{}, fmt.Errorf("create vault: %w", err)
	}
	if err := qtx.CreateVault(ctx, db.CreateVaultParams{
		ID: vaultID, OwnerID: owner, Name: name, CreatedAt: now, UpdatedAt: now, ChangeSeq: seq,
	}); err != nil {
		return db.Vault{}, fmt.Errorf("create vault: %w", err)
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

	vault, err := qtx.GetVault(ctx, vaultID)
	if err != nil {
		return db.Vault{}, fmt.Errorf("create vault: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return db.Vault{}, fmt.Errorf("create vault: %w", err)
	}
	return vault, nil
}

func (s *Service) RenameVault(ctx context.Context, id, name string, actor Actor) (db.Vault, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return db.Vault{}, ErrInvalidVaultName
	}
	if _, err := checkVault(ctx, s.q, id, actor); err != nil {
		return db.Vault{}, err
	}
	seq, err := s.q.NextChangeSeq(ctx)
	if err != nil {
		return db.Vault{}, fmt.Errorf("rename vault: %w", err)
	}
	if err := s.q.RenameVault(ctx, db.RenameVaultParams{
		Name: name, UpdatedAt: timestamp(), ChangeSeq: seq, ID: id,
	}); err != nil {
		return db.Vault{}, fmt.Errorf("rename vault: %w", err)
	}
	vault, err := s.q.GetVault(ctx, id)
	if err != nil {
		return db.Vault{}, fmt.Errorf("rename vault: %w", err)
	}
	return vault, nil
}

func (s *Service) DeleteVault(ctx context.Context, id string, actor Actor) error {
	if _, err := checkVault(ctx, s.q, id, actor); err != nil {
		return err
	}
	seq, err := s.q.NextChangeSeq(ctx)
	if err != nil {
		return fmt.Errorf("delete vault: %w", err)
	}
	now := timestamp()
	if err := s.q.SoftDeleteVault(ctx, db.SoftDeleteVaultParams{
		DeletedAt: &now, UpdatedAt: now, ChangeSeq: seq, ID: id,
	}); err != nil {
		return fmt.Errorf("delete vault: %w", err)
	}
	return nil
}

// unowned vaults are open to everyone, owned vaults to their owner and admins.
func accessible(v db.Vault, actor Actor) bool {
	if v.OwnerID == nil || actor.Admin {
		return true
	}
	return actor.UserID != nil && *actor.UserID == *v.OwnerID
}

// checkVault resolves a vault the user may access.
func checkVault(ctx context.Context, q *db.Queries, vaultID string, actor Actor) (db.Vault, error) {
	v, err := q.GetVault(ctx, vaultID)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Vault{}, ErrVaultNotFound
	}
	if err != nil {
		return db.Vault{}, fmt.Errorf("get vault: %w", err)
	}
	if !accessible(v, actor) {
		return db.Vault{}, ErrVaultNotFound
	}
	return v, nil
}

// NoteSummary is a note without its body.
type NoteSummary struct {
	ID        string
	VaultID   string
	Name      string
	Version   int64
	CreatedAt string
	UpdatedAt string
	ChangeSeq int64
	Deleted   bool
}

// Vaults lists accessible vaults.
func (s *Service) Vaults(ctx context.Context, actor Actor, since int64) ([]db.Vault, int64, error) {
	var vaults []db.Vault
	var err error
	if since > 0 {
		vaults, err = s.q.ListVaultsSince(ctx, since)
	} else {
		vaults, err = s.q.ListVaults(ctx)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list vaults: %w", err)
	}
	visible := vaults[:0]
	for _, v := range vaults {
		if accessible(v, actor) {
			visible = append(visible, v)
		}
	}
	cursor, err := s.q.GetChangeSeq(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("read change cursor: %w", err)
	}
	return visible, cursor, nil
}

// List lists a vault's notes.
func (s *Service) List(ctx context.Context, vaultID string, actor Actor, since int64) ([]NoteSummary, int64, error) {
	if _, err := checkVault(ctx, s.q, vaultID, actor); err != nil {
		return nil, 0, err
	}
	var out []NoteSummary
	if since > 0 {
		rows, err := s.q.ListNotesSince(ctx, db.ListNotesSinceParams{VaultID: vaultID, ChangeSeq: since})
		if err != nil {
			return nil, 0, fmt.Errorf("list notes: %w", err)
		}
		for _, r := range rows {
			out = append(out, NoteSummary{ID: r.ID, VaultID: r.VaultID, Name: r.Name, Version: r.Version,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ChangeSeq: r.ChangeSeq,
				Deleted: r.TrashedAt != nil || r.DeletedAt != nil})
		}
	} else {
		rows, err := s.q.ListNotes(ctx, vaultID)
		if err != nil {
			return nil, 0, fmt.Errorf("list notes: %w", err)
		}
		for _, r := range rows {
			out = append(out, NoteSummary{ID: r.ID, VaultID: r.VaultID, Name: r.Name, Version: r.Version,
				CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ChangeSeq: r.ChangeSeq})
		}
	}
	cursor, err := s.q.GetChangeSeq(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("read change cursor: %w", err)
	}
	return out, cursor, nil
}

func (s *Service) Get(ctx context.Context, id string, actor Actor) (db.Note, error) {
	note, err := s.q.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, ErrNotFound
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("get note: %w", err)
	}
	if _, err := checkVault(ctx, s.q, note.VaultID, actor); err != nil {
		return db.Note{}, ErrNotFound
	}
	if note.TrashedAt != nil {
		return db.Note{}, ErrNotFound
	}
	return note, nil
}

// Trash soft-deletes a note, it stays restorable.
func (s *Service) Trash(ctx context.Context, id string, actor Actor) error {
	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("trash note: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	existing, err := qtx.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("trash note: %w", err)
	}
	if _, err := checkVault(ctx, qtx, existing.VaultID, actor); err != nil {
		return ErrNotFound
	}
	seq, err := qtx.NextChangeSeq(ctx)
	if err != nil {
		return fmt.Errorf("trash note: %w", err)
	}
	now := timestamp()
	affected, err := qtx.TrashNote(ctx, db.TrashNoteParams{
		TrashedAt: &now, UpdatedAt: now,
		UpdatedByKind: actor.Kind, UpdatedByUser: actor.UserID, UpdatedByToken: actor.TokenID,
		ChangeSeq: seq, ID: id,
	})
	if err != nil {
		return fmt.Errorf("trash note: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// Restore brings a trashed note back.
func (s *Service) Restore(ctx context.Context, id string, actor Actor) (db.Note, error) {
	tx, err := s.sqldb.BeginTx(ctx, nil)
	if err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	existing, err := qtx.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, ErrNotFound
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
	}
	if _, err := checkVault(ctx, qtx, existing.VaultID, actor); err != nil {
		return db.Note{}, ErrNotFound
	}
	if existing.TrashedAt == nil {
		return existing, nil
	}
	seq, err := qtx.NextChangeSeq(ctx)
	if err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
	}
	affected, err := qtx.RestoreNote(ctx, db.RestoreNoteParams{
		UpdatedAt:     timestamp(),
		UpdatedByKind: actor.Kind, UpdatedByUser: actor.UserID, UpdatedByToken: actor.TokenID,
		ChangeSeq: seq, ID: id,
	})
	if err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
	}
	if affected == 0 {
		return db.Note{}, ErrNotFound
	}
	note, err := qtx.GetNote(ctx, id)
	if err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return db.Note{}, fmt.Errorf("restore note: %w", err)
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

	if _, err := checkVault(ctx, qtx, vaultID, actor); err != nil {
		return db.Note{}, err
	}
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

	existing, err := qtx.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.Note{}, ErrNotFound
	}
	if err != nil {
		return db.Note{}, fmt.Errorf("update note: %w", err)
	}
	if _, err := checkVault(ctx, qtx, existing.VaultID, actor); err != nil {
		return db.Note{}, ErrNotFound
	}
	if existing.TrashedAt != nil {
		return db.Note{}, ErrNotFound
	}

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
