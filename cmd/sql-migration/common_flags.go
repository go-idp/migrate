package main

import (
	"fmt"
	"strings"

	core "github.com/go-idp/sql-migration/internal/migrate"
	"github.com/go-zoox/cli"
)

// errRequiredDBFlags matches github.com/urfave/cli/v2 required-flag errors so missing
// connection options surface as "Required flag … not set" instead of custom messages.
func errRequiredDBFlags(ctx *cli.Context) error {
	groups := [][]string{
		{"driver", "D"},
		{"host"},
		{"port", "P"},
		{"user", "u"},
		{"pass", "p"},
		{"database", "d"},
	}
	var missing []string
	for _, names := range groups {
		set := false
		for _, key := range names {
			if ctx.IsSet(strings.TrimSpace(key)) {
				set = true
				break
			}
		}
		if !set {
			missing = append(missing, names[0])
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if len(missing) == 1 {
		return fmt.Errorf("Required flag %q not set", missing[0])
	}
	joined := strings.Join(missing, ", ")
	return fmt.Errorf("Required flags %q not set", joined)
}

// dbConnectionFlags registers database flags. When required is true, urfave/cli enforces
// that each flag is set (flag or env) before the command Action runs.
func dbConnectionFlags(required bool) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "driver",
			Aliases:  []string{"D"},
			Usage:    "database driver: mysql|mariadb|postgres|sqlite3",
			EnvVars:  []string{"DB_DRIVER"},
			Required: required,
		},
		&cli.StringFlag{
			Name:     "host",
			Usage:    "database host",
			EnvVars:  []string{"DB_HOST"},
			Required: required,
		},
		&cli.StringFlag{
			Name:     "port",
			Aliases:  []string{"P"},
			Usage:    "database port",
			EnvVars:  []string{"DB_PORT"},
			Required: required,
		},
		&cli.StringFlag{
			Name:     "user",
			Aliases:  []string{"u"},
			Usage:    "database user",
			EnvVars:  []string{"DB_USER"},
			Required: required,
		},
		&cli.StringFlag{
			Name:     "pass",
			Aliases:  []string{"p"},
			Usage:    "database password",
			EnvVars:  []string{"DB_PASS"},
			Required: required,
		},
		&cli.StringFlag{
			Name:     "database",
			Aliases:  []string{"d"},
			Usage:    "database name",
			EnvVars:  []string{"DB_NAME"},
			Required: required,
		},
	}
}

func migrationsDirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "migrations-dir",
		Aliases: []string{"r"},
		Usage:   "migrations directory path",
		Value:   core.DefaultMigrationsDir,
		EnvVars: []string{"SQL_MIGRATION_DIR"},
	}
}

func migrationsTableFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "migrations-table",
		Aliases: []string{"t"},
		Usage:   "migrations record table name",
		Value:   core.DefaultMigrationsTableName,
	}
}

func migrationsPathFlags() []cli.Flag {
	return []cli.Flag{migrationsDirFlag(), migrationsTableFlag()}
}

// migrationsTableFlags is for commands that only touch the history table (no directory scan).
func migrationsTableFlags() []cli.Flag {
	return []cli.Flag{migrationsTableFlag()}
}
