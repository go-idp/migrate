# `migrate`

Apply SQL migrations from a directory to the target database. This is the primary command for executing DDL/DML defined in versioned `.sql` files.

## Synopsis

```text
sql-migration migrate [options]
```

There are no positional arguments. All inputs come from flags (and their environment-variable defaults, where defined).

## What it does

1. Validates database connection settings (`driver`, `host`, `port`, `user`, `pass`, `database`).
2. Opens a connection and pings the server (`MustConnect`).
3. Ensures the migration **history table** exists (`CREATE TABLE IF NOT EXISTS`).
4. Loads and sorts all valid `.sql` files from `--migrations-dir` (`LoadMigrations`).
5. Loads existing **sequence** numbers already recorded in the history table.
6. For each file, according to **mode**:
   - **`diff` (default):** if the file’s sequence is already in history, skip it; otherwise run the SQL in a transaction and insert a history row.
   - **`all`:** run every file’s SQL, then **upsert** the history row for that sequence (re-applies changes and refreshes checksum / name / version tag / `executed_at` where supported).
7. Stops on the first SQL or history error; earlier transactions are already committed per migration (one transaction per applied migration in normal run).

Unless **`--dry-run`** is set, successful completion prints:

```text
migration completed
```

## Options

### Database connection

| Long | Short | Environment variable | Required | Description |
|------|-------|----------------------|----------|-------------|
| `--driver` | `-D` | `DB_DRIVER` | Yes | `mysql`, `mariadb`, `postgres`, `sqlite3` (aliases normalized, e.g. `postgresql` → `postgres`). |
| `--host` | — | `DB_HOST` | Yes | Database host. |
| `--port` | `-P` | `DB_PORT` | Yes | Port. |
| `--user` | `-u` | `DB_USER` | Yes | Username. |
| `--pass` | `-p` | `DB_PASS` | Yes | Password. |
| `--database` | `-d` | `DB_NAME` | Yes | Database name. For **SQLite3**, this is typically the path to the database file. |

Note: **`--host` intentionally has no `-h` alias** (conflicts with global help in the underlying CLI stack).

### Migrations location and history table

| Long | Short | Environment variable | Default | Description |
|------|-------|----------------------|---------|-------------|
| `--migrations-dir` | `-r` | `SQL_MIGRATION_DIR` | `./migrations` | Directory containing `.sql` migration files. |
| `--migrations-table` | `-t` | — | `migrations` | Table name for execution history. Must match `^[A-Za-z_][A-Za-z0-9_]*$`. |

### Run behavior

| Long | Short | Environment variable | Default | Description |
|------|-------|----------------------|---------|-------------|
| `--mode` | `-m` | `SQL_MIGRATION_MODE` | `diff` | `diff` — skip already-recorded sequences. `all` — run every file and upsert history. |
| `--dry-run` | `-n` | `SQL_MIGRATION_DRY_RUN` | `false` | See [Dry-run](#dry-run) below. |

## Dry-run

When `--dry-run` / `-n` is enabled (or `SQL_MIGRATION_DRY_RUN` is true):

- The runner uses a **single database transaction** for the whole dry-run.
- It creates the history table **inside that transaction** only (rolled back at the end).
- It executes applicable migration SQL against the transaction but **does not persist** history rows to the real table after rollback.

**Supported drivers:** `postgres`, `sqlite3`.

**Not supported:** `mysql`, `mariadb`. The tool returns an error because DDL on these engines often causes implicit commits, so a rollback cannot reliably undo everything.

`diff` and `all` still control which files are executed during the dry-run; only persistence differs.

On success:

```text
dry-run completed (no changes persisted)
```

## Exit status

- **0** — all migrations in the selected mode completed successfully (or dry-run succeeded).
- **Non-zero** — connection failure, invalid mode, unsupported dry-run driver, invalid migration filenames, SQL error, or history insert/upsert error.

## Examples

PostgreSQL, default `diff` mode:

```bash
sql-migration migrate -D postgres -h 127.0.0.1 -P 5432 -u app -p secret -d appdb -r ./migrations
```

MySQL with `all` mode (re-run every script, upsert history):

```bash
sql-migration migrate -D mysql -h 127.0.0.1 -P 3306 -u root -p secret -d appdb -m all
```

SQLite3 (`DB_NAME` is the database file path; **all** connection fields must still be non-empty for validation — use placeholders if your driver ignores them):

```bash
sql-migration migrate -D sqlite3 -h localhost -P 0 -u x -p x -d ./local.db -r ./migrations
```

Dry-run on PostgreSQL:

```bash
sql-migration migrate -D postgres -h 127.0.0.1 -P 5432 -u app -p secret -d appdb -n
```

## Related commands

- [validate](./validate.md) — check files, or verify file checksums against recorded history **without** applying SQL.
- [status](./status.md) — inspect pending / applied / drift / orphan before or after a run.
- [commit](./commit.md) — add a history row for one file when SQL was applied manually; **does not** run the file.

## See also

- Migration filename rules and checksums: [README § Migration file specification](../../README.md#migration-file-specification)
