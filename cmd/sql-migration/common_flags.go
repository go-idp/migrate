package main

import (
	core "github.com/go-idp/sql-migration/internal/migrate"
	"github.com/go-zoox/cli"
)

func dbConnectionFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "driver",
			Aliases: []string{"D"},
			Usage:   "database driver: mysql|mariadb|postgres|sqlite3",
			EnvVars: []string{"DB_DRIVER"},
		},
		&cli.StringFlag{
			Name:    "host",
			Usage:   "database host",
			EnvVars: []string{"DB_HOST"},
		},
		&cli.StringFlag{
			Name:    "port",
			Aliases: []string{"P"},
			Usage:   "database port",
			EnvVars: []string{"DB_PORT"},
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Usage:   "database user",
			EnvVars: []string{"DB_USER"},
		},
		&cli.StringFlag{
			Name:    "pass",
			Aliases: []string{"p"},
			Usage:   "database password",
			EnvVars: []string{"DB_PASS"},
		},
		&cli.StringFlag{
			Name:    "database",
			Aliases: []string{"d"},
			Usage:   "database name",
			EnvVars: []string{"DB_NAME"},
		},
	}
}

func migrationsDirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "migrations-dir",
		Aliases: []string{"r"},
		Usage:   "migrations directory path",
		Value:   core.DefaultMigrationsDir,
		EnvVars: []string{"MIGRATE_DIR"},
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
