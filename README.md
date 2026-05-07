# migrate

A production-grade Go database migration CLI for MySQL, MariaDB, PostgreSQL, and SQLite3.

## Features

- Supported drivers: `mysql`, `mariadb`, `postgres`, `sqlite3`
- Migration state is stored in a database table (default: `migrations`)
- Automatically creates the migration history table on startup
- Re-running the tool safely skips already applied sequences
- Migration files are sorted and executed by sequence in ascending order
- Supports both DDL and DML migrations (idempotent DML is recommended)
- Logging is powered by `github.com/go-zoox/logger`
- Cross-platform support: Linux / macOS / Windows
- Runtime progress logs: `[current/total] applying file (filename) ...`

## Build

```bash
go build -o migrate ./cmd/migrate
```

## Configuration Priority

Environment variables have higher priority than CLI flags.

Environment variables:

- `DB_DRIVER`
- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASS`
- `DB_NAME`

CLI flags:

- `-D` driver
- `-h` host
- `-P` port
- `-u` username
- `-p` password
- `-d` database name (required)
- `-m` migrations directory (default: `./migrations`)
- `-t` migrations history table (default: `migrations`)

## Usage

```bash
migrate -D <driver> -h <host> -P <port> -u <user> -p <password> -d <database> [-m <migrations_dir>] [-t <migrations_table>]
```

Example (CLI only):

```bash
./migrate -D mysql -h 127.0.0.1 -P 3306 -u root -p secret -d app_db
```

Example (environment variables override CLI flags):

```bash
DB_DRIVER=postgres \
DB_HOST=127.0.0.1 \
DB_PORT=5432 \
DB_USER=postgres \
DB_PASS=secret \
DB_NAME=app_db \
./migrate -D mysql -h 1.1.1.1 -P 3306 -u root -p root -d ignored_by_env
```

## Migration File Specification

- Default directory: `./migrations` (overridable with `-m`)
- Filename format: `<sequence>_<module>_<business_desc>.<version>.sql`
- Example: `99_user_add_age.v2026.05.06.sql`
- Sequence extraction rule: the numeric part before the first underscore
- Version tag extraction rule: the suffix before `.sql` (for example, `v2026.05.06`)
- Execution order: ascending by sequence

## Migration History Table

- Default table name: `migrations` (overridable with `-t`)
- Built-in unique constraint ensures one execution per sequence
- Re-running the CLI skips sequences that are already recorded

## Testing

```bash
go test ./...
```

Current test coverage includes:

- Configuration validation and DSN generation
- Migration filename validation, sorting, and duplicate sequence detection
- SQLite integration: rerun skip logic and custom migration table support
- Real-world scenarios: DDL + idempotent DML + failure stop and record checks
