package migrate

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	migrationNamePattern      = regexp.MustCompile(`^(\d+)_([A-Za-z0-9]+)_([A-Za-z0-9_]+)\.(v[A-Za-z0-9]+(?:\.[A-Za-z0-9]+)*)\.sql$`)
	migrationSequencePrefix   = regexp.MustCompile(`^(\d+)(?:_|$)`)
)

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
		sequence, versionTag, err := parseMigrationMetadata(fileName)
		if err != nil {
			return nil, err
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
			VersionTag: versionTag,
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

// LoadMigrationFile reads a single migration SQL file by path (absolute or relative).
func LoadMigrationFile(path string) (Migration, error) {
	path = filepath.Clean(path)
	fileName := filepath.Base(path)
	sequence, versionTag, err := parseMigrationMetadata(fileName)
	if err != nil {
		return Migration{}, err
	}

	sqlContent, err := os.ReadFile(path)
	if err != nil {
		return Migration{}, fmt.Errorf("read migration file %q: %w", path, err)
	}

	return Migration{
		Sequence:   sequence,
		VersionTag: versionTag,
		Name:       fileName,
		Path:       path,
		SQL:        string(sqlContent),
		Checksum:   checksum(sqlContent),
	}, nil
}

func parseMigrationMetadata(fileName string) (sequence int64, versionTag string, err error) {
	if matches := migrationNamePattern.FindStringSubmatch(fileName); len(matches) > 0 {
		sequence, err = strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("parse migration sequence from %q: %w", fileName, err)
		}
		return sequence, matches[4], nil
	}

	if matches := migrationSequencePrefix.FindStringSubmatch(fileName); len(matches) > 0 {
		sequence, err = strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("parse migration sequence from %q: %w", fileName, err)
		}
	}

	return sequence, parseVersionTag(fileName), nil
}

func parseVersionTag(fileName string) string {
	base := strings.TrimSuffix(fileName, ".sql")
	if idx := strings.Index(base, ".v"); idx >= 0 {
		return base[idx+1:]
	}
	if idx := strings.LastIndex(base, "."); idx >= 0 && idx < len(base)-1 {
		return base[idx+1:]
	}
	return ""
}

func checksum(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}
