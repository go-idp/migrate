package migrate

import (
	"path/filepath"
	"testing"
)

func TestResolveConfig_ValidateMissingField(t *testing.T) {
	cfg := Config{
		Driver: "mysql",
		Host:   "127.0.0.1",
		Port:   "3306",
		User:   "root",
		Pass:   "secret",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing db name validation error")
	}
}

func TestBuildDSN_SQLiteUsesDatabaseName(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	cfg := Config{
		Driver: "sqlite3",
		Host:   "ignored-host",
		Port:   "0",
		User:   "ignored-user",
		Pass:   "ignored-pass",
		Name:   dbPath,
	}

	dsn, err := BuildDSN(cfg)
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}

	if dsn != dbPath {
		t.Fatalf("unexpected sqlite dsn, want %s, got %s", dbPath, dsn)
	}
}
