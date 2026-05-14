# Command reference

Detailed documentation for each `sql-migration` subcommand.

| Command | Summary |
|---------|---------|
| [migrate](./migrate.md) | Apply SQL migration files to the database (`diff` / `all`, optional dry-run). |
| [validate](./validate.md) | Check migration files on disk; optionally verify history checksums against the database. |
| [status](./status.md) | Print `pending` / `applied` / `drift` / `orphan` lines for every known sequence. |
| [commit](./commit.md) | Upsert one migration into history **without executing** its SQL file. |

Global program options:

- `sql-migration --help` / `-h` — list commands and global flags.
- `sql-migration --version` / `-v` — print version.

See the [repository README](../../README.md) for an overview, build instructions, and migration file naming rules.
