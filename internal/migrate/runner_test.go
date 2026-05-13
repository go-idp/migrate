package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-zoox/logger"
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
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	if err := r.Run(dir, RunModeDiff); err != nil {
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
	if err := r.Run(dir, RunModeDiff); err != nil {
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
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if err := r.Run(dir, RunModeDiff); err != nil {
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
	if err := r.Run(dir, RunModeDiff); err != nil {
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

func TestApplyMigration_SqliteReportsRowsAffectedForInsert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apply_insert.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.ensureMigrationsTable(); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}

	sqlBody := `CREATE TABLE IF NOT EXISTS items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  label TEXT NOT NULL
);
INSERT INTO items (label) VALUES ('a'), ('b');`
	m := Migration{
		Sequence:   1,
		VersionTag: "v2026.05.06",
		Name:       "1_items_seed.v2026.05.06.sql",
		SQL:        sqlBody,
		Checksum:   checksum([]byte(sqlBody)),
	}

	rowsAffected, rowsKnown, elapsed, err := r.applyMigration(m, RunModeDiff)
	if err != nil {
		t.Fatalf("applyMigration: %v", err)
	}
	if !rowsKnown {
		t.Fatal("expected RowsAffected to be supported for sqlite migration Exec")
	}
	if rowsAffected < 1 {
		t.Fatalf("expected at least one changed row from INSERT, got %d", rowsAffected)
	}
	if elapsed < 0 {
		t.Fatalf("elapsed must be non-negative, got %v", elapsed)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations"`).Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 migration row recorded, got %d", n)
	}

	var itemCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if itemCount != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", itemCount)
	}
}

func TestApplyMigration_FailedExecDoesNotRecordMigrationRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apply_fail.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.ensureMigrationsTable(); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}

	sqlBody := `THIS IS NOT VALID SQL;`
	m := Migration{
		Sequence:   1,
		VersionTag: "v2026.05.06",
		Name:       "1_bad_syntax.v2026.05.06.sql",
		SQL:        sqlBody,
		Checksum:   checksum([]byte(sqlBody)),
	}

	_, _, _, err = r.applyMigration(m, RunModeDiff)
	if err == nil {
		t.Fatal("expected applyMigration to fail on invalid SQL")
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations"`).Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no migration row after failed exec, got %d", n)
	}
}

func TestDryRun_SqliteRollsBackAllChangesThenRealRunPersists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dryrun_then_run.db")
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
SELECT 'dryrun_alice'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE name = 'dryrun_alice');
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.DryRun(dir, RunModeDiff); err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&tableCount); err != nil {
		t.Fatalf("sqlite_master users: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("after dry-run expected no users table, got count=%d", tableCount)
	}

	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("real run: %v", err)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&tableCount); err != nil {
		t.Fatalf("sqlite_master users after run: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("expected users table after real run, got count=%d", tableCount)
	}

	var nUsers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE name = 'dryrun_alice'`).Scan(&nUsers); err != nil {
		t.Fatalf("count alice: %v", err)
	}
	if nUsers != 1 {
		t.Fatalf("expected 1 row after real run, got %d", nUsers)
	}

	var rec int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations"`).Scan(&rec); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if rec != 2 {
		t.Fatalf("expected 2 migration records, got %d", rec)
	}
}

func TestDryRun_SqliteStopsOnFailureAndLeavesNoCommittedState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dryrun_fail.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_ok.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL
);
`)
	writeSQL(t, filepath.Join(dir, "2_bad.v2026.05.06.sql"), `
INSERT INTO users (oops) VALUES (1);
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.DryRun(dir, RunModeDiff); err == nil {
		t.Fatal("expected dry-run to fail on bad SQL")
	}

	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&tableCount); err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("after failed dry-run expected no users table, got %d", tableCount)
	}
}

func TestDryRun_MysqlNotSupported(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dryrun_mysql_driver.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_x.v2026.05.06.sql"), `SELECT 1;`)

	r := NewRunner(db, "mysql", DefaultMigrationsTableName)
	if err := r.DryRun(dir, RunModeDiff); err == nil {
		t.Fatal("expected dry-run to reject mysql driver")
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
	if err := r.Run(dir, RunModeDiff); err == nil {
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

func TestRun_WithDebugLogLevel_CompletesSuccessfully(t *testing.T) {
	prevLevel := logger.GetLevel()
	if err := logger.SetLevel("debug"); err != nil {
		t.Fatalf("set debug level: %v", err)
	}
	t.Cleanup(func() {
		_ = logger.SetLevel(prevLevel)
	})

	dbPath := filepath.Join(t.TempDir(), "debug_level.db")
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

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("run at debug level: %v", err)
	}
}

func TestRun_ModeAll_UpsertsHistoryAfterFileChanged(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mode_all.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	path := filepath.Join(dir, "1_kv_seed.v2026.05.06.sql")
	sqlV1 := `CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY);
INSERT OR IGNORE INTO kv(k) VALUES ('a');
`
	writeSQL(t, path, sqlV1)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("first diff run: %v", err)
	}

	wantChecksum1 := checksum([]byte(sqlV1))
	var gotChecksum string
	if err := db.QueryRow(`SELECT checksum FROM "migrations" WHERE sequence = 1`).Scan(&gotChecksum); err != nil {
		t.Fatalf("read checksum after diff: %v", err)
	}
	if gotChecksum != wantChecksum1 {
		t.Fatalf("checksum after diff: got %s want %s", gotChecksum, wantChecksum1)
	}

	sqlV2 := `CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY);
INSERT OR IGNORE INTO kv(k) VALUES ('b');
`
	writeSQL(t, path, sqlV2)

	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("second diff run: %v", err)
	}
	if err := db.QueryRow(`SELECT checksum FROM "migrations" WHERE sequence = 1`).Scan(&gotChecksum); err != nil {
		t.Fatalf("read checksum after second diff: %v", err)
	}
	if gotChecksum != wantChecksum1 {
		t.Fatalf("diff mode must not refresh checksum when skipped: got %s want %s", gotChecksum, wantChecksum1)
	}

	if err := r.Run(dir, RunModeAll); err != nil {
		t.Fatalf("all mode run: %v", err)
	}
	wantChecksum2 := checksum([]byte(sqlV2))
	if err := db.QueryRow(`SELECT checksum FROM "migrations" WHERE sequence = 1`).Scan(&gotChecksum); err != nil {
		t.Fatalf("read checksum after all: %v", err)
	}
	if gotChecksum != wantChecksum2 {
		t.Fatalf("all mode should upsert checksum: got %s want %s", gotChecksum, wantChecksum2)
	}

	var keyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kv`).Scan(&keyCount); err != nil {
		t.Fatalf("count kv: %v", err)
	}
	if keyCount != 2 {
		t.Fatalf("expected rows a and b in kv, got count %d", keyCount)
	}
}

func TestApplyMigration_ModeAll_UpsertsExistingHistoryRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apply_all.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.ensureMigrationsTable(); err != nil {
		t.Fatalf("ensure migrations table: %v", err)
	}

	sql1 := `CREATE TABLE IF NOT EXISTS t (x INTEGER NOT NULL);`
	m1 := Migration{
		Sequence:   1,
		VersionTag: "v2026.05.06",
		Name:       "1_create.v2026.05.06.sql",
		SQL:        sql1,
		Checksum:   checksum([]byte(sql1)),
	}
	if _, _, _, err := r.applyMigration(m1, RunModeDiff); err != nil {
		t.Fatalf("first applyMigration: %v", err)
	}

	sql2 := `INSERT INTO t VALUES (7);`
	m2 := Migration{
		Sequence:   1,
		VersionTag: "v2026.05.07",
		Name:       "1_create.v2026.05.07.sql",
		SQL:        sql2,
		Checksum:   checksum([]byte(sql2)),
	}
	if _, _, _, err := r.applyMigration(m2, RunModeAll); err != nil {
		t.Fatalf("applyMigration all: %v", err)
	}

	var gotName, gotChecksum, gotVer string
	err = db.QueryRow(`SELECT name, checksum, version_tag FROM "migrations" WHERE sequence = 1`).Scan(&gotName, &gotChecksum, &gotVer)
	if err != nil {
		t.Fatalf("query migrations row: %v", err)
	}
	if gotName != m2.Name || gotChecksum != m2.Checksum || gotVer != m2.VersionTag {
		t.Fatalf("upserted row got name=%q checksum=%q ver=%q; want %q %q %q", gotName, gotChecksum, gotVer, m2.Name, m2.Checksum, m2.VersionTag)
	}

	var x int
	if err := db.QueryRow(`SELECT x FROM t WHERE x = 7`).Scan(&x); err != nil {
		t.Fatalf("expected inserted row: %v", err)
	}
}

func TestEnsureMigrationsTable_InvalidTableNameRejected(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "invalid_table.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	r := NewRunner(db, "sqlite3", `bad-name`)
	if err := r.ensureMigrationsTable(); err == nil {
		t.Fatal("expected error for invalid migrations table name")
	}
}

func writeSQL(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sql file %s: %v", path, err)
	}
}
