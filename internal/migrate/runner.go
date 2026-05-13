package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

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

type sqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type sqlQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// DryRun validates pending migrations against the current database without persisting changes.
// It runs inside a single transaction and rolls back at the end, so migration SQL and the
// history table are not committed. Not supported for MySQL/MariaDB because DDL there often
// triggers implicit commits, so a rollback cannot reliably undo changes.
func (r *Runner) DryRun(dir string, mode RunMode) error {
	switch r.driver {
	case "mysql", "mariadb":
		return fmt.Errorf(
			"dry-run is not supported for driver %q: MySQL/MariaDB DDL usually commits implicitly, so changes cannot be rolled back; use PostgreSQL or SQLite3 for transactional dry-run, or validate against a database copy",
			r.driver,
		)
	}

	logger.Infof("starting migration dry-run: mode=%s dir=%s table=%s driver=%s", mode, dir, r.tableName, r.driver)

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin dry-run transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	logger.Infof("ensure migrations table exists (inside dry-run transaction)")
	if err := r.ensureMigrationsTableExec(tx); err != nil {
		return err
	}

	logger.Infof("load migration files")
	migrations, err := LoadMigrations(dir)
	if err != nil {
		return err
	}

	logger.Infof("load executed migration sequences")
	executedSequences, err := r.loadExecutedSequencesQuery(tx)
	if err != nil {
		return err
	}

	logger.Infof("validate pending migrations one by one")
	appliedCount := 0
	skippedCount := 0
	total := len(migrations)
	for idx, migration := range migrations {
		current := idx + 1

		if mode == RunModeDiff {
			if _, done := executedSequences[migration.Sequence]; done {
				logger.Infof(
					"[%d/%d] Ignored %s (already executed)",
					current,
					total,
					migration.Name,
				)
				skippedCount++
				continue
			}
		}

		if mode == RunModeAll {
			if _, done := executedSequences[migration.Sequence]; done {
				logger.Infof(
					"[%d/%d] reapplying %s (mode=all, dry-run) ...",
					current,
					total,
					migration.Name,
				)
			} else {
				logger.Infof(
					"[%d/%d] applying %s (dry-run) ...",
					current,
					total,
					migration.Name,
				)
			}
		} else {
			logger.Infof(
				"[%d/%d] applying %s (dry-run) ...",
				current,
				total,
				migration.Name,
			)
		}

		rowsAffected, rowsKnown, elapsed, err := r.applyMigrationDryTx(tx, migration)
		if err != nil {
			logger.Errorf(
				"[%d/%d] dry-run failed for %s: %v",
				current,
				total,
				migration.Name,
				err,
			)
			return err
		}

		if rowsKnown {
			logger.Infof(
				"[%d/%d] validated %s (rows_affected=%d, elapsed=%s)",
				current,
				total,
				migration.Name,
				rowsAffected,
				elapsed.String(),
			)
		} else {
			logger.Infof(
				"[%d/%d] validated %s (elapsed=%s)",
				current,
				total,
				migration.Name,
				elapsed.String(),
			)
		}
		appliedCount++
	}

	logger.Infof(
		"migration dry-run completed: mode=%s total=%d validated=%d skipped=%d table=%s (transaction rolled back; nothing persisted)",
		mode,
		len(migrations),
		appliedCount,
		skippedCount,
		r.tableName,
	)
	return nil
}

// Run applies migrations in ascending sequence order according to mode.
func (r *Runner) Run(dir string, mode RunMode) error {
	logger.Infof("starting migration run: mode=%s dir=%s table=%s driver=%s", mode, dir, r.tableName, r.driver)

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

	appliedCount := 0
	skippedCount := 0
	total := len(migrations)
	for idx, migration := range migrations {
		current := idx + 1

		if mode == RunModeDiff {
			if _, done := executedSequences[migration.Sequence]; done {
				logger.Infof(
					"[%d/%d] Ignored %s (already executed)",
					current,
					total,
					migration.Name,
				)
				skippedCount++
				continue
			}
		}

		if mode == RunModeAll {
			if _, done := executedSequences[migration.Sequence]; done {
				logger.Infof(
					"[%d/%d] reapplying %s (mode=all) ...",
					current,
					total,
					migration.Name,
				)
			} else {
				logger.Infof(
					"[%d/%d] applying %s ...",
					current,
					total,
					migration.Name,
				)
			}
		} else {
			logger.Infof(
				"[%d/%d] applying %s ...",
				current,
				total,
				migration.Name,
			)
		}

		rowsAffected, rowsKnown, elapsed, err := r.applyMigration(migration, mode)
		if err != nil {
			logger.Errorf(
				"[%d/%d] failed to migrate %s: %v",
				current,
				total,
				migration.Name,
				err,
			)
			return err
		}

		if rowsKnown {
			logger.Infof(
				"[%d/%d] applied %s (rows_affected=%d, elapsed=%s)",
				current,
				total,
				migration.Name,
				rowsAffected,
				elapsed.String(),
			)
		} else {
			logger.Infof(
				"[%d/%d] applied %s (elapsed=%s)",
				current,
				total,
				migration.Name,
				elapsed.String(),
			)
		}
		appliedCount++
	}

	logger.Infof(
		"migration run completed: mode=%s total=%d applied=%d skipped=%d table=%s",
		mode,
		len(migrations),
		appliedCount,
		skippedCount,
		r.tableName,
	)
	return nil
}

// ensureMigrationsTable creates the migration history table if it does not exist.
func (r *Runner) ensureMigrationsTable() error {
	ddl, err := r.migrationsTableDDL()
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ddl); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

func (r *Runner) ensureMigrationsTableExec(ex sqlExecutor) error {
	ddl, err := r.migrationsTableDDL()
	if err != nil {
		return err
	}
	if _, err := ex.Exec(ddl); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

func (r *Runner) migrationsTableDDL() (string, error) {
	if !tableNamePattern.MatchString(r.tableName) {
		return "", fmt.Errorf("invalid migrations table name %q, allowed pattern: %s", r.tableName, tableNamePattern.String())
	}

	tableName := r.quotedTableName()

	switch r.driver {
	case "mysql", "mariadb":
		return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  sequence BIGINT NOT NULL,
  version_tag VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  checksum CHAR(64) NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_migrations_sequence (sequence)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`, tableName), nil
	case "postgres":
		return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id BIGSERIAL PRIMARY KEY,
  sequence BIGINT NOT NULL,
  version_tag VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  checksum CHAR(64) NOT NULL,
  executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_migrations_sequence UNIQUE (sequence)
);
`, tableName), nil
	case "sqlite3":
		return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  sequence INTEGER NOT NULL UNIQUE,
  version_tag TEXT NOT NULL,
  name TEXT NOT NULL,
  checksum TEXT NOT NULL,
  executed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`, tableName), nil
	default:
		return "", fmt.Errorf("unsupported driver for migrations table: %s", r.driver)
	}
}

// loadExecutedSequences fetches already executed migration sequences from the database.
func (r *Runner) loadExecutedSequences() (map[int64]struct{}, error) {
	return r.loadExecutedSequencesQuery(r.db)
}

func (r *Runner) loadExecutedSequencesQuery(q sqlQuerier) (map[int64]struct{}, error) {
	rows, err := q.Query(fmt.Sprintf(`SELECT sequence FROM %s`, r.quotedTableName()))
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

// applyMigrationDryTx executes migration SQL on an open transaction only (no history row).
func (r *Runner) applyMigrationDryTx(tx *sql.Tx, migration Migration) (rowsAffected int64, rowsKnown bool, elapsed time.Duration, err error) {
	start := time.Now()

	logger.Debugf("[%s] migration SQL (dry-run):\n%s", migration.Name, migration.SQL)

	result, err := tx.Exec(migration.SQL)
	if err != nil {
		return 0, false, time.Since(start), fmt.Errorf("execute migration SQL: %w", err)
	}

	if ra, raErr := result.RowsAffected(); raErr == nil {
		rowsAffected = ra
		rowsKnown = true
	}

	var execDebug strings.Builder
	if rowsKnown {
		fmt.Fprintf(&execDebug, "rows_affected=%d", rowsAffected)
	} else {
		execDebug.WriteString("rows_affected=<unsupported>")
	}
	if lid, lidErr := result.LastInsertId(); lidErr == nil {
		fmt.Fprintf(&execDebug, ", last_insert_id=%d", lid)
	}
	logger.Debugf("[%s] migration SQL exec output (dry-run): %s", migration.Name, execDebug.String())

	return rowsAffected, rowsKnown, time.Since(start), nil
}

// upsertMigrationRecordSQL returns dialect-specific INSERT ... ON CONFLICT / ON DUPLICATE KEY for history upsert.
func (r *Runner) upsertMigrationRecordSQL() string {
	t := r.quotedTableName()
	p1, p2, p3, p4 := r.placeholder(1), r.placeholder(2), r.placeholder(3), r.placeholder(4)
	switch r.driver {
	case "mysql", "mariadb":
		return fmt.Sprintf(
			`INSERT INTO %s(sequence, version_tag, name, checksum) VALUES (%s,%s,%s,%s) ON DUPLICATE KEY UPDATE version_tag=VALUES(version_tag), name=VALUES(name), checksum=VALUES(checksum), executed_at=CURRENT_TIMESTAMP`,
			t, p1, p2, p3, p4,
		)
	case "postgres":
		return fmt.Sprintf(
			`INSERT INTO %s(sequence, version_tag, name, checksum) VALUES (%s,%s,%s,%s) ON CONFLICT (sequence) DO UPDATE SET version_tag=EXCLUDED.version_tag, name=EXCLUDED.name, checksum=EXCLUDED.checksum, executed_at=NOW()`,
			t, p1, p2, p3, p4,
		)
	case "sqlite3":
		return fmt.Sprintf(
			`INSERT INTO %s(sequence, version_tag, name, checksum) VALUES (%s,%s,%s,%s) ON CONFLICT(sequence) DO UPDATE SET version_tag=excluded.version_tag, name=excluded.name, checksum=excluded.checksum, executed_at=CURRENT_TIMESTAMP`,
			t, p1, p2, p3, p4,
		)
	default:
		return ""
	}
}

// applyMigration executes one migration and records it atomically in a transaction.
// rowsKnown is false when the driver does not report RowsAffected for this execution.
func (r *Runner) applyMigration(migration Migration, mode RunMode) (rowsAffected int64, rowsKnown bool, elapsed time.Duration, err error) {
	start := time.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return 0, false, time.Since(start), fmt.Errorf("begin migration transaction: %w", err)
	}

	logger.Debugf("[%s] migration SQL:\n%s", migration.Name, migration.SQL)

	result, err := tx.Exec(migration.SQL)
	if err != nil {
		_ = tx.Rollback()
		return 0, false, time.Since(start), fmt.Errorf("execute migration SQL: %w", err)
	}

	if ra, raErr := result.RowsAffected(); raErr == nil {
		rowsAffected = ra
		rowsKnown = true
	}

	var execDebug strings.Builder
	if rowsKnown {
		fmt.Fprintf(&execDebug, "rows_affected=%d", rowsAffected)
	} else {
		execDebug.WriteString("rows_affected=<unsupported>")
	}
	if lid, lidErr := result.LastInsertId(); lidErr == nil {
		fmt.Fprintf(&execDebug, ", last_insert_id=%d", lid)
	}
	logger.Debugf("[%s] migration SQL exec output: %s", migration.Name, execDebug.String())

	if mode == RunModeAll {
		q := r.upsertMigrationRecordSQL()
		if q == "" {
			_ = tx.Rollback()
			return 0, false, time.Since(start), fmt.Errorf("record migration: unsupported driver %q for mode=all", r.driver)
		}
		if _, err := tx.Exec(q, migration.Sequence, migration.VersionTag, migration.Name, migration.Checksum); err != nil {
			_ = tx.Rollback()
			return 0, false, time.Since(start), fmt.Errorf("record migration: %w", err)
		}
	} else {
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
				return 0, false, time.Since(start), nil
			}
			return 0, false, time.Since(start), fmt.Errorf("record migration: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, false, time.Since(start), fmt.Errorf("commit migration transaction: %w", err)
	}
	return rowsAffected, rowsKnown, time.Since(start), nil
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
