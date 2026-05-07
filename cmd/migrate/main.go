package main

import (
	"fmt"
	"os"

	mg "github.com/go-idp/migrate"
	"github.com/go-idp/migrate/internal/migrate"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-zoox/cli"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func main() {
	// Build the CLI interface and bind flags to env vars.
	app := cli.NewSingleProgram(&cli.SingleProgramConfig{
		Name:      "migrate",
		Usage:     "database sql migrations",
		UsageText: usageText,
		Version:   mg.Version,
		HideHelp:  true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "driver",
				Aliases: []string{"D"},
				Usage:   "database driver: mysql|mariadb|postgres|sqlite3",
				EnvVars: []string{"DB_DRIVER"},
			},
			&cli.StringFlag{
				Name:    "host",
				Aliases: []string{"h"},
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
			&cli.StringFlag{
				Name:    "migrations-dir",
				Aliases: []string{"m"},
				Usage:   "migrations directory path",
				Value:   migrate.DefaultMigrationsDir,
			},
			&cli.StringFlag{
				Name:    "migrations-table",
				Aliases: []string{"t"},
				Usage:   "migrations record table name",
				Value:   migrate.DefaultMigrationsTableName,
			},
		},
	})

	app.Command(func(ctx *cli.Context) error {
		// Collect runtime config from CLI/env and validate required fields.
		cfg := migrate.Config{
			Driver: ctx.String("driver"),
			Host:   ctx.String("host"),
			Port:   ctx.String("port"),
			User:   ctx.String("user"),
			Pass:   ctx.String("pass"),
			Name:   ctx.String("database"),
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprint(os.Stderr, usageText)
			return err
		}

		db, normalizedDriver, err := migrate.MustConnect(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		// Execute migrations from the configured directory into the configured history table.
		runner := migrate.NewRunner(db, normalizedDriver, ctx.String("migrations-table"))
		if err := runner.Run(ctx.String("migrations-dir")); err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, "migration completed")
		return nil
	})

	app.Run()
}

const usageText = `Usage:
  migrate -D <driver> -h <host> -P <port> -u <user> -p <password> -d <database> [-m <migrations_dir>] [-t <migrations_table>]

Environment variables (override command-line flags):
  DB_DRIVER, DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME

Migration directory:
  default: ./migrations (override with -m)

Migrations table:
  default: migrations (override with -t)
`
