package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestValidateAgainstDB_OK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_mod_x.v2026.05.06.sql"), `SELECT 1;`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := r.ValidateAgainstDB(dir); err != nil {
		t.Fatalf("ValidateAgainstDB: %v", err)
	}
}

func TestValidateAgainstDB_ChecksumMismatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	p := filepath.Join(dir, "1_mod_x.v2026.05.06.sql")
	writeSQL(t, p, `SELECT 1;`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("run: %v", err)
	}

	writeSQL(t, p, `SELECT 2;`)
	if err := r.ValidateAgainstDB(dir); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestValidateAgainstDB_OrphanHistoryRow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_mod_x.v2026.05.06.sql"), `SELECT 1;`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Remove migration file; DB still has history for sequence 1.
	if err := os.Remove(filepath.Join(dir, "1_mod_x.v2026.05.06.sql")); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	if err := r.ValidateAgainstDB(dir); err == nil {
		t.Fatal("expected error for missing file for recorded sequence")
	}
}

func TestStatusReport_PendingAppliedOrphan(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "st.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_mod_a.v2026.05.06.sql"), `SELECT 1;`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	if err := r.Run(dir, RunModeDiff); err != nil {
		t.Fatalf("run: %v", err)
	}

	writeSQL(t, filepath.Join(dir, "2_mod_b.v2026.05.06.sql"), `SELECT 2;`)

	if _, err := db.Exec(`INSERT INTO "migrations" (sequence, version_tag, name, checksum) VALUES (9, 'v1', '9_mod_z.v1.sql', '00000000000000000000000000000000')`); err != nil {
		t.Fatalf("insert orphan: %v", err)
	}

	rows, err := r.StatusReport(dir)
	if err != nil {
		t.Fatalf("StatusReport: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	want := map[int64]MigrationLineStatus{
		1: StatusApplied,
		2: StatusPending,
		9: StatusOrphan,
	}
	for _, row := range rows {
		st, ok := want[row.Sequence]
		if !ok || st != row.Status {
			t.Fatalf("row seq=%d status=%s, want status=%v (ok=%v)", row.Sequence, row.Status, want[row.Sequence], ok)
		}
	}
}

func TestRecordWithoutSQL_DoesNotExecuteFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rec.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir := t.TempDir()
	writeSQL(t, filepath.Join(dir, "1_mod_t.v2026.05.06.sql"), `
CREATE TABLE IF NOT EXISTS only_if_run (
  id INTEGER PRIMARY KEY
);
`)

	r := NewRunner(db, "sqlite3", DefaultMigrationsTableName)
	p := filepath.Join(dir, "1_mod_t.v2026.05.06.sql")
	if err := r.RecordWithoutSQLFile(p); err != nil {
		t.Fatalf("RecordWithoutSQLFile: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='only_if_run'`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master: %v", err)
	}
	if n != 0 {
		t.Fatal("expected migration SQL not to run, but table exists")
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM "migrations" WHERE sequence = 1`).Scan(&n); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 history row, got %d", n)
	}
}