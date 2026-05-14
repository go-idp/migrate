package migrate

import (
	"fmt"
	"sort"
	"strings"
)

// MigrationLineStatus classifies one logical migration sequence for reporting.
type MigrationLineStatus string

const (
	StatusPending MigrationLineStatus = "pending"
	StatusApplied MigrationLineStatus = "applied"
	StatusDrift   MigrationLineStatus = "drift"
	StatusOrphan  MigrationLineStatus = "orphan"
)

// StatusRow is one line of migration status (disk and/or DB).
type StatusRow struct {
	Sequence     int64
	Status       MigrationLineStatus
	Name         string
	FileChecksum string
	DBChecksum   string
}

// StatusReport builds pending/applied/drift/orphan rows by comparing disk files to history.
func (r *Runner) StatusReport(dir string) ([]StatusRow, error) {
	if err := r.ensureMigrationsTable(); err != nil {
		return nil, err
	}
	disk, err := LoadMigrations(dir)
	if err != nil {
		return nil, err
	}
	history, err := r.LoadAppliedHistory()
	if err != nil {
		return nil, err
	}

	dbBySeq := make(map[int64]HistoryRecord, len(history))
	for _, rec := range history {
		dbBySeq[rec.Sequence] = rec
	}

	diskBySeq := make(map[int64]Migration, len(disk))
	for _, m := range disk {
		diskBySeq[m.Sequence] = m
	}

	var rows []StatusRow

	for _, m := range disk {
		rec, inDB := dbBySeq[m.Sequence]
		if !inDB {
			rows = append(rows, StatusRow{
				Sequence:     m.Sequence,
				Status:       StatusPending,
				Name:         m.Name,
				FileChecksum: m.Checksum,
				DBChecksum:   "-",
			})
			continue
		}
		if m.Checksum != rec.Checksum {
			rows = append(rows, StatusRow{
				Sequence:     m.Sequence,
				Status:       StatusDrift,
				Name:         m.Name,
				FileChecksum: m.Checksum,
				DBChecksum:   rec.Checksum,
			})
			continue
		}
		rows = append(rows, StatusRow{
			Sequence:     m.Sequence,
			Status:       StatusApplied,
			Name:         rec.Name,
			FileChecksum: m.Checksum,
			DBChecksum:   rec.Checksum,
		})
	}

	for _, rec := range history {
		if _, onDisk := diskBySeq[rec.Sequence]; !onDisk {
			rows = append(rows, StatusRow{
				Sequence:     rec.Sequence,
				Status:       StatusOrphan,
				Name:         rec.Name,
				FileChecksum: "-",
				DBChecksum:   rec.Checksum,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Sequence < rows[j].Sequence
	})
	return rows, nil
}

// FormatStatusTable renders status rows as a tab-separated table for stdout.
func FormatStatusTable(rows []StatusRow) string {
	var b strings.Builder
	b.WriteString("SEQ\tSTATUS\tNAME\tFILE_CHECKSUM\tDB_CHECKSUM\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t%s\n", row.Sequence, row.Status, row.Name, row.FileChecksum, row.DBChecksum)
	}
	return b.String()
}
