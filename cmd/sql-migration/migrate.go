package main

import (
	"fmt"
	"os"

	core "github.com/go-idp/sql-migration/internal/migrate"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-zoox/cli"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func migrate() *cli.Command {
	flags := append(dbConnectionFlags(true), migrationsPathFlags()...)
	flags = append(flags,
		&cli.StringFlag{
			Name:    "mode",
			Aliases: []string{"m"},
			Usage:   "run mode: diff (skip applied sequences, default) | all (re-run all SQL and upsert history)",
			Value:   string(core.RunModeDiff),
			EnvVars: []string{"SQL_MIGRATION_MODE"},
		},
		&cli.BoolFlag{
			Name:    "dry-run",
			Aliases: []string{"n"},
			Usage:   "validate migrations in one transaction and roll back (postgres, sqlite3 only; not mysql)",
			Value:   false,
			EnvVars: []string{"SQL_MIGRATION_DRY_RUN"},
		},
	)
	return &cli.Command{
		Name:  "migrate",
		Usage: "apply migrations",
		Flags: flags,
		Action: func(ctx *cli.Context) error {
			cfg := core.Config{
				Driver: ctx.String("driver"),
				Host:   ctx.String("host"),
				Port:   ctx.String("port"),
				User:   ctx.String("user"),
				Pass:   ctx.String("pass"),
				Name:   ctx.String("database"),
			}
			mode, err := core.ParseRunMode(ctx.String("mode"))
			if err != nil {
				return err
			}

			db, normalizedDriver, err := core.MustConnect(cfg)
			if err != nil {
				return err
			}
			defer db.Close()

			runner := core.NewRunner(db, normalizedDriver, ctx.String("migrations-table"))
			if ctx.Bool("dry-run") {
				if err := runner.DryRun(ctx.String("migrations-dir"), mode); err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, "dry-run completed (no changes persisted)")
				return nil
			}

			if err := runner.Run(ctx.String("migrations-dir"), mode); err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "migration completed")
			return nil
		},
	}
}
