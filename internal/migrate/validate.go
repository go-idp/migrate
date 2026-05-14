package migrate

import "fmt"

// ValidateMigrationDir checks that migration files under dir are readable and well-formed.
func ValidateMigrationDir(dir string) error {
	if _, err := LoadMigrations(dir); err != nil {
		return err
	}
	return nil
}

// ValidateAgainstDB ensures every recorded migration still matches the on-disk file (checksum),
// and that each DB sequence has a matching migration file.
func (r *Runner) ValidateAgainstDB(dir string) error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}
	disk, err := LoadMigrations(dir)
	if err != nil {
		return err
	}
	diskBySeq := make(map[int64]Migration, len(disk))
	for _, m := range disk {
		diskBySeq[m.Sequence] = m
	}

	history, err := r.LoadAppliedHistory()
	if err != nil {
		return err
	}

	for _, rec := range history {
		m, ok := diskBySeq[rec.Sequence]
		if !ok {
			return fmt.Errorf("history sequence %d (%s) has no matching migration file on disk", rec.Sequence, rec.Name)
		}
		if m.Checksum != rec.Checksum {
			return fmt.Errorf(
				"checksum mismatch for sequence %d (%s): database=%s file=%s",
				rec.Sequence,
				m.Name,
				rec.Checksum,
				m.Checksum,
			)
		}
	}
	return nil
}
