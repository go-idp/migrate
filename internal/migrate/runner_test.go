package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRun_SqliteExecutesOnceAndSkipsOnRerun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_user_create_table.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL
);
`)
	writeSQL(t, filepath.Join(dir, "2_user_seed_data.v2026.05.06.sql"), `
INSERT INTO users (name)
SELECT 'alice'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE name = 'alice');
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	if err := r.Run(dir); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'alice'").Scan(&count); err != nil {
		t.Fatalf("count seeded rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 seeded row, got %d", count)
	}

	var migrationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations"`).Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != 2 {
		t.Fatalf("expected 2 migration records, got %d", migrationCount)
	}
}

func TestRun_SqliteSupportsCustomMigrationsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migrate_custom_table.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_user_create_table.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL
);
`)

	customTable := "schema_migrations"
	r := NewRunner(db, "sqlite3", customTable)
	if err := r.Run(dir); err != nil {
		t.Fatalf("run with custom table failed: %v", err)
	}

	var migrationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "schema_migrations"`).Scan(&migrationCount); err != nil {
		t.Fatalf("count custom table migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("expected 1 migration record, got %d", migrationCount)
	}
}

func TestRun_RealWorldWorkflow_DDLAndIdempotentDML(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "real_world.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_user_create_tables.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS user_profiles (
  user_id INTEGER NOT NULL PRIMARY KEY,
  nickname TEXT NOT NULL,
  FOREIGN KEY(user_id) REFERENCES users(id)
);
`)
	writeSQL(t, filepath.Join(dir, "2_user_seed_data.v2026.05.06.sql"), `
INSERT INTO users (email, status)
SELECT 'alice@example.com', 'active'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = 'alice@example.com');

INSERT INTO user_profiles (user_id, nickname)
SELECT id, 'alice'
FROM users
WHERE email = 'alice@example.com'
  AND NOT EXISTS (
    SELECT 1 FROM user_profiles
    WHERE user_id = users.id
  );
`)
	writeSQL(t, filepath.Join(dir, "3_user_backfill_status.v2026.05.06.sql"), `
UPDATE users
SET status = 'active'
WHERE email = 'alice@example.com'
  AND status <> 'active';
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if err := r.Run(dir); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	var userCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'alice@example.com'`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("expected 1 user row after rerun, got %d", userCount)
	}

	var profileCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_profiles`).Scan(&profileCount); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if profileCount != 1 {
		t.Fatalf("expected 1 profile row after rerun, got %d", profileCount)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM users WHERE email = 'alice@example.com'`).Scan(&status); err != nil {
		t.Fatalf("query final status: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected user status active, got %s", status)
	}
}

func TestRun_AppliesMigrationsInAscendingSequenceOneByOne(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ordered_apply.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	// Write files intentionally out of order to verify runner sorting by sequence.
	writeSQL(t, filepath.Join(dir, "10_user_step_ten.v2026.05.06.sql"), `
INSERT INTO migration_order (step) VALUES (10);
`)
	writeSQL(t, filepath.Join(dir, "1_user_create_order_table.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS migration_order (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  step INTEGER NOT NULL
);
INSERT INTO migration_order (step) VALUES (1);
`)
	writeSQL(t, filepath.Join(dir, "2_user_step_two.v2026.05.06.sql"), `
INSERT INTO migration_order (step) VALUES (2);
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir); err != nil {
		t.Fatalf("run ordered migration failed: %v", err)
	}

	rows, err := db.Query(`SELECT step FROM migration_order ORDER BY id`)
	if err != nil {
		t.Fatalf("query migration execution order: %v", err)
	}
	defer rows.Close()

	var got []int
	for rows.Next() {
		var step int
		if err := rows.Scan(&step); err != nil {
			t.Fatalf("scan migration step: %v", err)
		}
		got = append(got, step)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration execution order: %v", err)
	}

	want := []int{1, 2, 10}
	if len(got) != len(want) {
		t.Fatalf("unexpected execution steps count, want=%v got=%v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("migrations were not applied one-by-one in ascending sequence, want=%v got=%v", want, got)
		}
	}
}

func TestRun_StopsOnFailureAndDoesNotRecordBrokenMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "failure_case.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_user_create_table.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL
);
`)
	writeSQL(t, filepath.Join(dir, "2_user_bad_statement.v2026.05.06.sql"), `
INSERT INTO users (unknown_column) VALUES ('broken');
`)
	writeSQL(t, filepath.Join(dir, "3_user_seed_data.v2026.05.06.sql"), `
INSERT INTO users (name)
SELECT 'bob'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE name = 'bob');
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir); err == nil {
		t.Fatal("expected migration run to fail on broken SQL")
	}

	var recorded int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations"`).Scan(&recorded); err != nil {
		t.Fatalf("count recorded migrations: %v", err)
	}
	if recorded != 1 {
		t.Fatalf("expected only first migration to be recorded, got %d", recorded)
	}

	var bobCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE name = 'bob'`).Scan(&bobCount); err != nil {
		t.Fatalf("count bob rows: %v", err)
	}
	if bobCount != 0 {
		t.Fatalf("expected third migration to be skipped after failure, got bob count %d", bobCount)
	}
}

func writeSQL(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sql file %s: %v", path, err)
	}
}
