package migrate

import (
	"fmt"
	"strings"
)

// RunMode controls whether already-recorded migrations are skipped or re-applied.
type RunMode string

const (
	// RunModeDiff skips migrations whose sequence already exists in the history table (default).
	RunModeDiff RunMode = "diff"
	// RunModeAll executes every migration SQL file and upserts the history row for each sequence.
	RunModeAll RunMode = "all"
)

// ParseRunMode normalizes CLI/env input into RunMode.
func ParseRunMode(s string) (RunMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(RunModeDiff):
		return RunModeDiff, nil
	case string(RunModeAll):
		return RunModeAll, nil
	default:
		return "", fmt.Errorf("invalid migration mode %q: use diff or all", s)
	}
}
