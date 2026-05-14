# sql-migration

A production-grade Go database migration CLI for MySQL, MariaDB, PostgreSQL, and SQLite3.

Go module: `github.com/go-idp/sql-migration`

## Features

- Supported drivers: `mysql`, `mariadb`, `postgres`, `sqlite3`
- Migration state is stored in a database table (default: `migrations`)
- Automatically creates the migration history table when needed
- **`migrate`**: apply SQL from files; **diff** mode (default) skips already-recorded sequences; **all** mode re-runs every file and upserts history (`checksum`, `name`, `version_tag`, `executed_at`)
- **`validate`**: check migration files on disk; with a database connection, verify that applied history matches current file checksums
- **`status`**: tabular report of `pending`, `applied`, `drift` (checksum mismatch), and `orphan` (history without a matching file) migrations
- **`commit <sql-file-path>`**: record one migration in history **without executing** its SQL (manual apply / restore alignment)
- Migration files are sorted and executed by sequence in ascending order
- Supports both DDL and DML migrations (idempotent DML is recommended)
- Logging uses `github.com/go-zoox/logger`
- Cross-platform: Linux / macOS / Windows
- Progress logs: `[current/total] applying …`
- **Dry-run** (`migrate` only: `-n`, `--dry-run`): run pending migrations in one transaction and roll back (**PostgreSQL** and **SQLite3** only; not MySQL/MariaDB)

## Build

```bash
go build -o sql-migration ./cmd/sql-migration
```

Install with Go 1.25+:

```bash
go install github.com/go-idp/sql-migration/cmd/sql-migration@latest
```

## CLI overview

```text
sql-migration [global options] <command> [command options] [arguments]
```

| Command | Purpose |
|--------|---------|
| `migrate` | Apply migrations from a directory |
| `validate` | Validate files; optionally validate against DB history |
| `status` | Show migration line status vs database |
| `commit` | Upsert history for one SQL file (no SQL execution) |

Global options: `--help`, `-h`, `--version`, `-v`.

## Configuration priority

Each option can be set via CLI flag and/or `EnvVars` on that flag. With **urfave/cli** (used under the hood), environment variables supply defaults when a flag is omitted; an explicit flag on the command line wins for that run.

### Environment variables

| Variable | Used by | Meaning Default |
|----------|---------|-----------------|
| `DB_DRIVER` | `migrate`, `validate` (not offline), `status`, `commit` | Driver name |
| `DB_HOST` | same | Host |
| `DB_PORT` | same | Port |
| `DB_USER` | same | User |
| `DB_PASS` | same | Password |
| `DB_NAME` | same | Database name (SQLite3: often the file path) |
| `MIGRATE_DIR` | `migrate`, `validate`, `status` | Migrations directory (`./migrations`) |
| `MIGRATE_MODE` | `migrate` only | `diff` or `all` (`diff`) |
| `MIGRATE_DRY_RUN` | `migrate` only | `true` enables dry-run (`-n`) |

### Common flags (connection)

- `-D`, `--driver` — `mysql` \| `mariadb` \| `postgres` \| `sqlite3`
- `--host`, `-P` / `--port`, `-u` / `--user`, `-p` / `--pass`, `-d` / `--database`
- `-r`, `--migrations-dir` — default `./migrations` (not used by `commit`)
- `-t`, `--migrations-table` — default `migrations`

## Command: `migrate`

Apply migrations from `--migrations-dir` (`-r`).

```bash
sql-migration migrate -D postgres -h 127.0.0.1 -P 5432 -u postgres -p secret -d app_db \
  -r ./migrations -m diff
```

Options:

- `-m`, `--mode` — `diff` (default) or `all`
- `-n`, `--dry-run` — transactional validation only (PostgreSQL / SQLite3; see **Dry-run** below)

## Command: `validate`

- **With database** (default): ensures the history table exists, loads files from `-r`, and checks that every **recorded** sequence has a matching file and **identical MD5 checksum** to the database.
- **Offline** (`-o`, `--offline`): only validates the migrations directory (naming, duplicates, readability). No DB connection required.

```bash
sql-migration validate --offline -r ./migrations

sql-migration validate -D postgres -h 127.0.0.1 -P 5432 -u postgres -p secret -d app_db -r ./migrations
```

## Command: `status`

Requires a database. Prints a tab-separated table:

`SEQ`, `STATUS`, `NAME`, `FILE_CHECKSUM`, `DB_CHECKSUM`

Status values: `pending`, `applied`, `drift`, `orphan`.

```bash
sql-migration status -D postgres -h 127.0.0.1 -P 5432 -u postgres -p secret -d app_db -r ./migrations
```

## Command: `commit`

Records a single migration in the history table using the file’s parsed metadata and checksum. **Does not execute** the SQL file.

```bash
sql-migration commit ./migrations/3_orders_add_index.v2026.05.14.sql \
  -D postgres -h 127.0.0.1 -P 5432 -u postgres -p secret -d app_db
```

The path can be relative or absolute. Only `-t` / `--migrations-table` applies besides DB flags (no `-r`).

## Dry-run (`migrate` only)

Dry-run connects to the **real** database, runs each applicable migration SQL inside a **single transaction**, does **not** commit history, then **rolls back** the transaction. You get parse/execution errors like a real run (where supported), without persisting schema or history.

- **Supported:** `postgres`, `sqlite3`
- **Not supported:** `mysql`, `mariadb` — DDL often causes implicit commits; use a copy DB or another engine for transactional dry-run.

`diff` and `all` still control **which** files execute; only persistence differs.

```bash
sql-migration migrate -D postgres -h 127.0.0.1 -P 5432 -u postgres -p secret -d app_db --dry-run
```

## Migration file specification

- Default directory: `./migrations` (`-r` / `MIGRATE_DIR`; not used by `commit`)
- Filename: `<sequence>_<module>_<business_desc>.<version>.sql`
- Example: `99_user_add_age.v2026.05.06.sql`
- **Sequence:** leading number before the first `_`
- **Version tag:** segment before `.sql` (e.g. `v2026.05.06`)
- **Order:** ascending by sequence

## Migration history table

- Default name: `migrations` (`-t`)
- `checksum`: lowercase hex MD5 of file bytes (same as `md5 -q` / `md5sum`)
- Unique constraint on `sequence`
- **diff:** skip if sequence exists; **all:** re-execute SQL and upsert row
- **commit:** upserts one row from the given file without running SQL

## Docker

Image builds binary `sql-migration` into `/bin/sql-migration`. Example:

```bash
docker run --rm sql-migration:latest sql-migration --version
```

(Replace the image name/tag with your registry build.)

## Testing

```bash
go test ./...
```

Coverage includes configuration, DSN generation, migration loading/sorting/validation, SQLite integration (run modes, custom table, dry-run, failures), and inspect flows (`validate` / `status` / record-without-SQL).

## More documentation

- **[docs/commands/](docs/commands/README.md)** — detailed documentation for each command (`migrate`, `validate`, `status`, `commit`).
- [docs/cli.md](docs/cli.md) — compact command reference and option tables.
