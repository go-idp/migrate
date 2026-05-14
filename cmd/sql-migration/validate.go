package main

import (
	"fmt"
	"os"

	core "github.com/go-idp/sql-migration/internal/migrate"

	"github.com/go-zoox/cli"
)

func validateCmd() *cli.Command {
	flags := append([]cli.Flag{}, migrationsPathFlags()...)
	flags = append(flags,
		&cli.BoolFlag{
			Name:    "offline",
			Aliases: []string{"o"},
			Usage:   "only check migration filenames and files on disk (no database)",
			Value:   false,
		},
	)

	return &cli.Command{
		Name:  "validate",
		Usage: "check migration files; with database, also verify history checksums",
		Flags: append(flags, dbConnectionFlags()...),
		Action: func(ctx *cli.Context) error {
			dir := ctx.String("migrations-dir")
			if ctx.Bool("offline") {
				if err := core.ValidateMigrationDir(dir); err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, "validate OK (offline)")
				return nil
			}

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
			if err := runner.ValidateAgainstDB(dir); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "validate OK")
			return nil
		},
	}
}
