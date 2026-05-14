package migrate

import (
	"fmt"
)

// RecordWithoutSQLMigration upserts the history row for m without executing m.SQL.
func (r *Runner) RecordWithoutSQLMigration(m Migration) error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}
	q := r.upsertMigrationRecordSQL()
	if q == "" {
		return fmt.Errorf("record migration history: unsupported driver %q", r.driver)
	}
	if _, err := r.db.Exec(q, m.Sequence, m.VersionTag, m.Name, m.Checksum); err != nil {
		return fmt.Errorf("record migration history: %w", err)
	}
	return nil
}

// RecordWithoutSQL upserts history for the migration file with the given sequence under dir.
func (r *Runner) RecordWithoutSQL(dir string, sequence int64) error {
	if sequence <= 0 {
		return fmt.Errorf("sequence must be positive, got %d", sequence)
	}
	migrations, err := LoadMigrations(dir)
	if err != nil {
		return err
	}
	for i := range migrations {
		if migrations[i].Sequence == sequence {
			return r.RecordWithoutSQLMigration(migrations[i])
		}
	}
	return fmt.Errorf("no migration file with sequence %d in %s", sequence, dir)
}

// RecordWithoutSQLFile upserts history using metadata from the given SQL file path.
func (r *Runner) RecordWithoutSQLFile(sqlPath string) error {
	m, err := LoadMigrationFile(sqlPath)
	if err != nil {
		return err
	}
	return r.RecordWithoutSQLMigration(m)
}
