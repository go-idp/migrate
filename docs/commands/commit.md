# `commit`

Insert or update a **single** migration record in the history table using metadata derived from a **SQL file path**. The file’s SQL is **not executed** against the database.

Use this when:

- SQL was applied manually or by another tool and you need history to match reality.
- You restored a database from backup and need to mark specific sequences without re-running DDL.

Misuse can cause **`drift`** on the next [validate](./validate.md) or [status](./status.md) if the database state does not actually match the file’s checksum and intent.

## Synopsis

```text
sql-migration commit [options] <sql-file-path>
```

- **`sql-file-path`** (positional, required): Path to one `.sql` file. May be relative or absolute. The **basename** must follow the project’s migration naming convention (see [Filename rules](#filename-rules)).

Only the **first** positional argument is used as the file path. Additional arguments are ignored by current implementations but should be avoided for clarity.

## What it does

1. Validates database connection flags (same requirements as `migrate`).
2. Connects and builds a runner for `--migrations-table`.
3. Reads the file: parses `sequence`, version tag, display `name`, and MD5 **checksum** from the file bytes (same rules as directory-based loading).
4. Ensures the history table exists.
5. Runs a driver-specific **UPSERT** (`INSERT … ON CONFLICT` / `ON DUPLICATE KEY`) so an existing row for the same `sequence` is replaced with the file’s current metadata and checksum.

On success, stdout shows something like:

```text
recorded 10_billing_tax.v2026.05.06.sql (sequence 10) in migration history (SQL was not executed)
```

## Options

### Positional

| Argument | Description |
|----------|-------------|
| `<sql-file-path>` | One migration `.sql` file. |

### History table

| Long | Short | Default | Description |
|------|-------|---------|-------------|
| `--migrations-table` | `-t` | `migrations` | Target history table. |

### Database connection

Full set of flags; **same as** [migrate](./migrate.md#database-connection) (`DB_DRIVER`, `DB_HOST`, etc.).

There is **no** `--migrations-dir` / `-r` flag on this command: only the explicit file path matters for the payload.

## Filename rules

The basename must match:

```text
<sequence>_<module>_<business_desc>.<version>.sql
```

Examples:

- `3_orders_add_index.v2026.05.14.sql`
- `10_user_email_unique.v2026.05.06.sql`

The tool derives:

- **sequence** — integer before the first `_`
- **version tag** — the segment after the second underscore group and before `.sql` (e.g. `v2026.05.14`)
- **checksum** — MD5 hex over raw file bytes

If the name does not match the pattern, `commit` fails before touching the database.

## Exit status

- **0** — history upsert succeeded.
- **Non-zero** — missing path argument, invalid filename, unreadable file, connection failure, invalid table name, or database error on upsert.

## Examples

Relative path:

```bash
sql-migration commit ./migrations/4_api_ratelimit.v2026.05.14.sql \
  -D postgres -h 127.0.0.1 -P 5432 -u app -p secret -d appdb
```

Absolute path:

```bash
sql-migration commit /deploy/migrations/4_api_ratelimit.v2026.05.14.sql -D postgres …
```

Custom history table:

```bash
sql-migration commit ./m/1_init.v1.sql -D postgres … -t schema_migrations
```

## Safety notes

- **No SQL execution** means schema/data might not match the recorded checksum’s implied contents. Prefer running [validate](./validate.md) online after operational changes.
- Upsert **overwrites** metadata for that `sequence`; use [status](./status.md) before and after if unsure.

## Related commands

- [migrate](./migrate.md) — normal path: execute SQL and record history.
- [validate](./validate.md) — verify files vs history.
- [status](./status.md) — inspect current line state.
