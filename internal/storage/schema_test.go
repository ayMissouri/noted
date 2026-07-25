package storage

import (
	"context"
	"testing"
)

func TestEmbeddedMigrationsApply(t *testing.T) {
	db := testDB(t)
	ran, err := Migrate(context.Background(), db, Migrations())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if ran < 1 {
		t.Fatalf("ran = %d, want at least 1", ran)
	}
	for _, table := range []string{"users", "tokens", "vaults", "folders", "notes", "note_versions", "change_counter", "server_config"} {
		if !tableExists(t, db, table) {
			t.Errorf("table %s missing", table)
		}
	}
	var seq int
	if err := db.QueryRow(`SELECT seq FROM change_counter WHERE id = 1`).Scan(&seq); err != nil {
		t.Fatalf("change_counter seed row: %v", err)
	}
	if seq != 0 {
		t.Errorf("change_counter seq = %d, want 0", seq)
	}
}
