package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

type migration struct {
	version int
	name    string
	sql     string
	fkOff   bool
}

func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) (int, error) {
	migs, err := loadMigrations(fsys)
	if err != nil {
		return 0, err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return 0, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	current := 0
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return 0, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return 0, fmt.Errorf("read schema_migrations: %w", err)
		}
		applied[v] = true
		if v > current {
			current = v
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read schema_migrations: %w", err)
	}

	known := map[int]bool{}
	for _, m := range migs {
		known[m.version] = true
	}
	for v := range applied {
		if !known[v] {
			return 0, fmt.Errorf("database has migration %d but this binary does not; refusing to run against a newer schema", v)
		}
	}

	ran := 0
	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		if m.version < current {
			return 0, fmt.Errorf("migration %04d_%s is older than already-applied version %d", m.version, m.name, current)
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return ran, err
		}
		current = m.version
		ran++
	}
	return ran, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	if m.fkOff {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return fmt.Errorf("migration %04d_%s: disable foreign keys: %w", m.version, m.name, err)
		}
		defer db.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	}
	if err := applyMigrationTx(ctx, db, m); err != nil {
		return err
	}
	if m.fkOff {
		var table, rowid, parent, fkid any
		err := db.QueryRowContext(ctx, "PRAGMA foreign_key_check").Scan(&table, &rowid, &parent, &fkid)
		if err == nil {
			return fmt.Errorf("migration %04d_%s left dangling references in %v", m.version, m.name, table)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("migration %04d_%s: foreign key check: %w", m.version, m.name, err)
		}
	}
	return nil
}

func applyMigrationTx(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.version, m.name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %04d_%s: %w", m.version, m.name, err)
	}
	return nil
}

func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var migs []migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, name, fkOff, err := parseMigrationName(e.Name())
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, prev, e.Name())
		}
		seen[version] = e.Name()
		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: version, name: name, sql: string(body), fkOff: fkOff})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

// the .fkoff.sql suffix runs the migration with foreign keys disabled.
func parseMigrationName(filename string) (version int, name string, fkOff bool, err error) {
	base := strings.TrimSuffix(filename, ".sql")
	fkOff = strings.HasSuffix(base, ".fkoff")
	base = strings.TrimSuffix(base, ".fkoff")
	num, name, ok := strings.Cut(base, "_")
	if !ok || num == "" || name == "" {
		return 0, "", false, fmt.Errorf("migration %s: want NNNN_name.sql", filename)
	}
	version, err = strconv.Atoi(num)
	if err != nil || version <= 0 {
		return 0, "", false, fmt.Errorf("migration %s: want NNNN_name.sql with a positive number", filename)
	}
	return version, name, fkOff, nil
}
