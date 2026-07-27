package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mig(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("sqlite_master query: %v", err)
	}
	return n == 1
}

func TestMigrateAppliesInOrder(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{
		"0002_second.sql": `CREATE TABLE b (id INTEGER PRIMARY KEY, a_id INTEGER REFERENCES a(id));`,
		"0001_first.sql":  `CREATE TABLE a (id INTEGER PRIMARY KEY);`,
	})
	ran, err := Migrate(context.Background(), db, fsys)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if ran != 2 {
		t.Errorf("ran = %d, want 2", ran)
	}
	if !tableExists(t, db, "a") || !tableExists(t, db, "b") {
		t.Error("expected tables a and b to exist")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{"0001_first.sql": `CREATE TABLE a (id INTEGER PRIMARY KEY);`})
	if _, err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	ran, err := Migrate(context.Background(), db, fsys)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if ran != 0 {
		t.Errorf("second run applied %d migrations, want 0", ran)
	}
}

func TestMigrateAppliesOnlyNew(t *testing.T) {
	db := testDB(t)
	first := mig(map[string]string{"0001_first.sql": `CREATE TABLE a (id INTEGER PRIMARY KEY);`})
	if _, err := Migrate(context.Background(), db, first); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	both := mig(map[string]string{
		"0001_first.sql":  `CREATE TABLE a (id INTEGER PRIMARY KEY);`,
		"0002_second.sql": `CREATE TABLE b (id INTEGER PRIMARY KEY);`,
	})
	ran, err := Migrate(context.Background(), db, both)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if ran != 1 {
		t.Errorf("ran = %d, want 1", ran)
	}
	if !tableExists(t, db, "b") {
		t.Error("expected table b to exist")
	}
}

func TestMigrateFailureRollsBack(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{
		"0001_bad.sql": `CREATE TABLE a (id INTEGER PRIMARY KEY); CREATE TABLE a (id INTEGER PRIMARY KEY);`,
	})
	if _, err := Migrate(context.Background(), db, fsys); err == nil {
		t.Fatal("Migrate succeeded, want error")
	}
	if tableExists(t, db, "a") {
		t.Error("table a exists after failed migration, want rollback")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 0 {
		t.Errorf("schema_migrations has %d rows after failure, want 0", n)
	}
}

func TestMigrateRejectsUnknownAppliedVersion(t *testing.T) {
	db := testDB(t)
	newer := mig(map[string]string{
		"0001_first.sql":  `CREATE TABLE a (id INTEGER PRIMARY KEY);`,
		"0002_second.sql": `CREATE TABLE b (id INTEGER PRIMARY KEY);`,
	})
	if _, err := Migrate(context.Background(), db, newer); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	older := mig(map[string]string{"0001_first.sql": `CREATE TABLE a (id INTEGER PRIMARY KEY);`})
	_, err := Migrate(context.Background(), db, older)
	if err == nil {
		t.Fatal("Migrate succeeded against a newer database, want error")
	}
	if !strings.Contains(err.Error(), "newer schema") {
		t.Errorf("error %q does not mention newer schema", err)
	}
}

func TestMigrateRejectsBadNames(t *testing.T) {
	for _, filename := range []string{"init.sql", "0000_zero.sql", "abc_x.sql", "0001_.sql"} {
		t.Run(filename, func(t *testing.T) {
			db := testDB(t)
			fsys := mig(map[string]string{filename: `SELECT 1;`})
			if _, err := Migrate(context.Background(), db, fsys); err == nil {
				t.Fatalf("Migrate accepted %s, want error", filename)
			}
		})
	}
}

func TestMigrateFkOffMarker(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{
		"0001_parent.sql": `CREATE TABLE p (id INTEGER PRIMARY KEY); CREATE TABLE c (id INTEGER PRIMARY KEY, p_id INTEGER REFERENCES p(id)); INSERT INTO p (id) VALUES (1); INSERT INTO c (id, p_id) VALUES (1, 1);`,
		"0002_rebuild.fkoff.sql": `CREATE TABLE p_new (id INTEGER PRIMARY KEY);
INSERT INTO p_new SELECT id FROM p;
DROP TABLE p;
ALTER TABLE p_new RENAME TO p;`,
	})
	if _, err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys after migration = %d, %v; want 1", fk, err)
	}
}

func TestPopulatedRebuildRequiresFkOffMarker(t *testing.T) {
	rebuild := `CREATE TABLE p_new (id INTEGER PRIMARY KEY);
INSERT INTO p_new SELECT id FROM p;
DROP TABLE p;
ALTER TABLE p_new RENAME TO p;`
	seed := `CREATE TABLE p (id INTEGER PRIMARY KEY); CREATE TABLE c (id INTEGER PRIMARY KEY, p_id INTEGER REFERENCES p(id)); INSERT INTO p (id) VALUES (1); INSERT INTO c (id, p_id) VALUES (1, 1);`

	db := testDB(t)
	fsys := mig(map[string]string{"0001_seed.sql": seed, "0002_rebuild.sql": rebuild})
	if _, err := Migrate(context.Background(), db, fsys); err == nil {
		t.Error("populated rebuild without the fkoff suffix succeeded, want FK failure")
	}

	db2 := testDB(t)
	fsys2 := mig(map[string]string{"0001_seed.sql": seed, "0002_rebuild.fkoff.sql": rebuild})
	if _, err := Migrate(context.Background(), db2, fsys2); err != nil {
		t.Errorf("populated rebuild with the fkoff suffix failed: %v", err)
	}
}

func TestMigrateFkOffDetectsDanglingReferences(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{
		"0001_parent.sql":      `CREATE TABLE p (id INTEGER PRIMARY KEY); CREATE TABLE c (id INTEGER PRIMARY KEY, p_id INTEGER REFERENCES p(id)); INSERT INTO p (id) VALUES (1); INSERT INTO c (id, p_id) VALUES (1, 1);`,
		"0002_break.fkoff.sql": `DELETE FROM p;`,
	})
	_, err := Migrate(context.Background(), db, fsys)
	if err == nil || !strings.Contains(err.Error(), "dangling") {
		t.Errorf("err = %v, want dangling reference failure", err)
	}
}

func TestMigrateRejectsDuplicateVersions(t *testing.T) {
	db := testDB(t)
	fsys := mig(map[string]string{
		"0001_a.sql": `SELECT 1;`,
		"0001_b.sql": `SELECT 1;`,
	})
	if _, err := Migrate(context.Background(), db, fsys); err == nil {
		t.Fatal("Migrate accepted duplicate versions, want error")
	}
}
