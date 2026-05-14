package main

import (
	"fmt"
	"os"

	core "github.com/go-idp/sql-migration/internal/migrate"

	"github.com/go-zoox/cli"
)

func statusCmd() *cli.Command {
	flags := append(migrationsPathFlags(), dbConnectionFlags()...)
	return &cli.Command{
		Name:  "status",
		Usage: "show applied, pending, drift, and orphan migrations",
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
			if err := cfg.Validate(); err != nil {
				return err
			}

			db, normalizedDriver, err := core.MustConnect(cfg)
			if err != nil {
				return err
			}
			defer db.Close()

			runner := core.NewRunner(db, normalizedDriver, ctx.String("migrations-table"))
			rows, err := runner.StatusReport(ctx.String("migrations-dir"))
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, core.FormatStatusTable(rows))
			return nil
		},
	}
}
