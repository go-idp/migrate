package main

import (
	"fmt"
	"os"

	core "github.com/go-idp/sql-migration/internal/migrate"

	"github.com/go-zoox/cli"
)

func commitCmd() *cli.Command {
	flags := append(migrationsTableFlags(), dbConnectionFlags(true)...)
	return &cli.Command{
		Name:      "commit",
		Usage:     "upsert history for a migration SQL file without executing it",
		ArgsUsage: "<sql-file-path>",
		Flags:     flags,
		Action: func(ctx *cli.Context) error {
			args := ctx.Args()
			if !args.Present() || args.First() == "" {
				return fmt.Errorf("missing required argument: sql file path (e.g. sql-migration commit ./migrations/1_mod_x.v1.sql)")
			}
			sqlPath := args.First()

			cfg := core.Config{
				Driver: ctx.String("driver"),
				Host:   ctx.String("host"),
				Port:   ctx.String("port"),
				User:   ctx.String("user"),
				Pass:   ctx.String("pass"),
				Name:   ctx.String("database"),
			}
			db, normalizedDriver, err := core.MustConnect(cfg)
			if err != nil {
				return err
			}
			defer db.Close()

			runner := core.NewRunner(db, normalizedDriver, ctx.String("migrations-table"))
			m, err := core.LoadMigrationFile(sqlPath)
			if err != nil {
				return err
			}
			if err := runner.RecordWithoutSQLMigration(m); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "recorded %s (sequence %d) in migration history (SQL was not executed)\n", m.Name, m.Sequence)
			return nil
		},
	}
}
