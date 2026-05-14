package migrate

import (
	"fmt"
)

// HistoryRecord is one row from the migrations history table.
type HistoryRecord struct {
	Sequence   int64
	VersionTag string
	Name       string
	Checksum   string
}

// LoadAppliedHistory returns recorded migrations ordered by sequence ascending.
func (r *Runner) LoadAppliedHistory() ([]HistoryRecord, error) {
	q := fmt.Sprintf(
		`SELECT sequence, version_tag, name, checksum FROM %s ORDER BY sequence ASC`,
		r.quotedTableName(),
	)
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query migration history: %w", err)
	}
	defer rows.Close()

	var out []HistoryRecord
	for rows.Next() {
		var rec HistoryRecord
		if err := rows.Scan(&rec.Sequence, &rec.VersionTag, &rec.Name, &rec.Checksum); err != nil {
			return nil, fmt.Errorf("scan migration history: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration history: %w", err)
	}
	return out, nil
}
