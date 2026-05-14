package main

import (
	sqlmigration "github.com/go-idp/sql-migration"
	"github.com/go-zoox/cli"
)

func main() {
	app := cli.NewMultipleProgram(&cli.MultipleProgramConfig{
		Name:    "sql-migration",
		Usage:   "database sql migrations",
		Version: sqlmigration.Version,
	})

	app.Register(migrate())

	app.Run()
}
