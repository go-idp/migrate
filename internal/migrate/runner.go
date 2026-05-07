package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-zoox/logger"
)

type Runner struct {
	db        *sql.DB
	driver    string
	tableName string
}

var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// NewRunner builds a migration runner with driver normalization and table fallback.
func NewRunner(db *sql.DB, driver string, tableName string) *Runner {
	if strings.TrimSpace(tableName) == "" {
		tableName = DefaultMigrationsTableName
	}

	return &Runner{
		db:        db,
		driver:    normalizeDriver(driver),
		tableName: tableName,
	}
}

// Run applies pending migrations in ascending sequence order.
func (r *Runner) Run(dir string) error {
	logger.Infof("starting migration run: dir=%s table=%s driver=%s", dir, r.tableName, r.driver)

	logger.Infof("ensure migrations table exists")
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	logger.Infof("load migration files")
	migrations, err := LoadMigrations(dir)
	if err != nil {
		return err
	}

	logger.Infof("load executed migration sequences")
	executedSequences, err := r.loadExecutedSequences()
	if err != nil {
		return err
	}

	logger.Infof("apply pending migrations one by one")
	appliedCount := 0
	skippedCount := 0
	total := len(migrations)
	for idx, migration := range migrations {
		current := idx + 1

		if _, done := executedSequences[migration.Sequence]; done {
			logger.Warnf(
				"[%d/%d] skip %s (already executed)",
				current,
				total,
				migration.Name,
			)
			skippedCount++
			continue
		}

		logger.Infof(
			"[%d/%d] applying %s ...",
			current,
			total,
			migration.Name,
		)
		if err := r.applyMigration(migration); err != nil {
			logger.Errorf(
				"[%d/%d] failed to migrate %s: %v",
				current,
				total,
				migration.Name,
				err,
			)
			return err
		}

		logger.Infof(
			"[%d/%d] applied %s",
			current,
			total,
			migration.Name,
		)
		appliedCount++
	}

	logger.Infof(
		"migration run completed: total=%d applied=%d skipped=%d table=%s",
		len(migrations),
		appliedCount,
		skippedCount,
		r.tableName,
	)
	return nil
}

// ensureMigrationsTable creates the migration history table if it does not exist.
func (r *Runner) ensureMigrationsTable() error {
	if !tableNamePattern.MatchString(r.tableName) {
		return fmt.Errorf("invalid migrations table name %q, allowed pattern: %s", r.tableName, tableNamePattern.String())
	}

	tableName := r.quotedTableName()

	var ddl string
	switch r.driver {
	case "mysql", "mariadb":
		ddl = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  sequence BIGINT NOT NULL,
  version_tag VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  checksum CHAR(64) NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_migrations_sequence (sequence)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`, tableName)
	case "postgres":
		ddl = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id BIGSERIAL PRIMARY KEY,
  sequence BIGINT NOT NULL,
  version_tag VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  checksum CHAR(64) NOT NULL,
  executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_migrations_sequence UNIQUE (sequence)
);
`, tableName)
	case "sqlite3":
		ddl = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  sequence INTEGER NOT NULL UNIQUE,
  version_tag TEXT NOT NULL,
  name TEXT NOT NULL,
  checksum TEXT NOT NULL,
  executed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`, tableName)
	default:
		return fmt.Errorf("unsupported driver for migrations table: %s", r.driver)
	}

	if _, err := r.db.Exec(ddl); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

// loadExecutedSequences fetches already executed migration sequences from the database.
func (r *Runner) loadExecutedSequences() (map[int64]struct{}, error) {
	rows, err := r.db.Query(fmt.Sprintf(`SELECT sequence FROM %s`, r.quotedTableName()))
	if err != nil {
		return nil, fmt.Errorf("query executed migrations: %w", err)
	}
	defer rows.Close()

	sequences := make(map[int64]struct{})
	for rows.Next() {
		var sequence int64
		if err := rows.Scan(&sequence); err != nil {
			return nil, fmt.Errorf("scan executed migration sequence: %w", err)
		}
		sequences[sequence] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate executed migration sequences: %w", err)
	}
	return sequences, nil
}

// applyMigration executes one migration and records it atomically in a transaction.
func (r *Runner) applyMigration(migration Migration) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	if _, err := tx.Exec(migration.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	insertSQL := fmt.Sprintf(
		`INSERT INTO %s(sequence, version_tag, name, checksum) VALUES (%s, %s, %s, %s)`,
		r.quotedTableName(),
		r.placeholder(1),
		r.placeholder(2),
		r.placeholder(3),
		r.placeholder(4),
	)
	if _, err := tx.Exec(insertSQL, migration.Sequence, migration.VersionTag, migration.Name, migration.Checksum); err != nil {
		_ = tx.Rollback()
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	return nil
}

// placeholder returns SQL placeholder syntax for the active database driver.
func (r *Runner) placeholder(position int) string {
	if r.driver == "postgres" {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}

// quotedTableName returns a dialect-safe quoted table name.
func (r *Runner) quotedTableName() string {
	if r.driver == "mysql" || r.driver == "mariadb" {
		return fmt.Sprintf("`%s`", r.tableName)
	}
	return fmt.Sprintf(`"%s"`, r.tableName)
}

// isUniqueViolation checks whether an error is caused by unique constraint conflicts.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation")
}

// MustConnect opens and pings the target database before returning the handle.
func MustConnect(cfg Config) (*sql.DB, string, error) {
	dsn, err := BuildDSN(cfg)
	if err != nil {
		return nil, "", err
	}

	driverName, err := SQLDriverName(cfg.Driver)
	if err != nil {
		return nil, "", err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, "", err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, "", errors.New("connect database failed: " + err.Error())
	}
	return db, driverName, nil
}
