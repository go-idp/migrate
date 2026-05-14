package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrations_SortsBySequence(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "10_user_add_age.v2026.05.06.sql", "SELECT 10;")
	writeMigrationFile(t, dir, "2_user_seed_data.v2026.05.05.sql", "SELECT 2;")

	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	if migrations[0].Sequence != 2 || migrations[1].Sequence != 10 {
		t.Fatalf("unexpected migration order: %+v", migrations)
	}

	if migrations[0].VersionTag != "v2026.05.05" || migrations[1].VersionTag != "v2026.05.06" {
		t.Fatalf("unexpected version tags: %+v", migrations)
	}
}

func TestLoadMigrations_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "bad.sql", "SELECT 1;")

	if _, err := LoadMigrations(dir); err == nil {
		t.Fatal("expected invalid migration name error")
	}
}

func TestLoadMigrations_RejectsDuplicateSequence(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "7_user_add_col.v2026.05.06.sql", "SELECT 1;")
	writeMigrationFile(t, dir, "7_order_seed.v2026.05.07.sql", "SELECT 2;")

	if _, err := LoadMigrations(dir); err == nil {
		t.Fatal("expected duplicate migration sequence error")
	}
}

func TestLoadMigrationFile(t *testing.T) {
	dir := t.TempDir()
	name := "5_svc_setup.v2026.05.07.sql"
	content := "SELECT 5;"
	writeMigrationFile(t, dir, name, content)
	path := filepath.Join(dir, name)

	m, err := LoadMigrationFile(path)
	if err != nil {
		t.Fatalf("LoadMigrationFile: %v", err)
	}
	if m.Sequence != 5 || m.Name != name || m.VersionTag != "v2026.05.07" {
		t.Fatalf("unexpected metadata: %+v", m)
	}
	wantSum := checksum([]byte(content))
	if m.Checksum != wantSum {
		t.Fatalf("checksum: got %s want %s", m.Checksum, wantSum)
	}
}

func writeMigrationFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", name, err)
	}
}
