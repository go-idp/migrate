package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

var migrationNamePattern = regexp.MustCompile(`^(\d+)_([A-Za-z0-9]+)_([A-Za-z0-9_]+)\.(v[A-Za-z0-9]+(?:\.[A-Za-z0-9]+)*)\.sql$`)

// Migration describes one SQL file plus parsed metadata from its filename.
type Migration struct {
	Sequence   int64
	VersionTag string
	Name       string
	Path       string
	SQL        string
	Checksum   string
}

// LoadMigrations reads, validates, and sorts migration files by sequence in ascending order.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]Migration, 0)
	sequences := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		fileName := entry.Name()
		matches := migrationNamePattern.FindStringSubmatch(fileName)
		if len(matches) == 0 {
			return nil, fmt.Errorf("invalid migration filename %q, expected: <序号>_<模块>_<业务描述>.<版本>.sql", fileName)
		}

		sequence, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration sequence from %q: %w", fileName, err)
		}
		// A sequence can only appear once to guarantee deterministic execution order.
		if existingName, ok := sequences[sequence]; ok {
			return nil, fmt.Errorf("duplicate migration sequence %d in %q and %q", sequence, existingName, fileName)
		}
		sequences[sequence] = fileName

		path := filepath.Join(dir, fileName)
		sqlContent, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration file %q: %w", fileName, err)
		}

		migrations = append(migrations, Migration{
			Sequence:   sequence,
			VersionTag: matches[4],
			Name:       fileName,
			Path:       path,
			SQL:        string(sqlContent),
			Checksum:   checksum(sqlContent),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Sequence < migrations[j].Sequence
	})
	return migrations, nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
