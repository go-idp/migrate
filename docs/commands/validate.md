# `validate`

Validate migration assets: either **filesystem-only** (no database) or **filesystem + database history** (checksum consistency for every recorded sequence).

## Synopsis

```text
sql-migration validate [options]
```

There are no positional arguments.

## Modes

### Offline (`--offline` / `-o`)

- Does **not** connect to a database.
- Does **not** require `DB_*` variables or connection flags (they may still appear in `--help` but are unused when offline).
- Runs the same logical checks as loading a migrations directory: readable directory, valid filenames, no duplicate sequences, file contents readable.

Success prints:

```text
validate OK (offline)
```

Use this in CI to catch naming or ordering issues before deployment.

### Online (default: `--offline` not set)

- Requires full database configuration (same as `migrate`).
- Creates the history table if it does not exist.
- Loads all migrations from `--migrations-dir`.
- Reads all rows from the history table (`sequence`, `version_tag`, `name`, `checksum`).
- For **each row** in history:
  - There must be a migration **file on disk** with the same `sequence`.
  - The file’s **MD5 checksum** (hex) must equal the checksum stored in the database.

If any history row points to a missing file or a checksum mismatch, the command fails with a descriptive error.

Success prints:

```text
validate OK
```

**Note:** Online validate does **not** execute migration SQL. It only compares metadata already recorded in the database to the current files.

## Options

### Offline toggle

| Long | Short | Default | Description |
|------|-------|---------|-------------|
| `--offline` | `-o` | `false` | If set, only validate the migrations directory; skip DB. |

### Migrations directory and table (always available on the CLI)

| Long | Short | Environment variable | Default | Description |
|------|-------|----------------------|---------|-------------|
| `--migrations-dir` | `-r` | `MIGRATE_DIR` | `./migrations` | Directory of `.sql` files. |
| `--migrations-table` | `-t` | — | `migrations` | History table name (used in online mode). |

### Database connection (required unless `--offline`)

Same as [migrate](./migrate.md#database-connection): `--driver` / `-D`, `--host`, `--port` / `-P`, `--user` / `-u`, `--pass` / `-p`, `--database` / `-d` with `DB_*` env vars.

## Exit status

- **0** — validation passed for the selected mode.
- **Non-zero** — invalid files, duplicate sequences, connection failure (online), or checksum / missing-file mismatch (online).

## Examples

CI filesystem check only:

```bash
sql-migration validate --offline -r ./migrations
```

Pre-deploy check against staging:

```bash
sql-migration validate -D postgres -h db.internal -P 5432 -u app -p "$DB_PASS" -d app_staging -r ./migrations
```

Custom history table:

```bash
sql-migration validate -D postgres … -t schema_migrations -r ./migrations
```

## Related commands

- [migrate](./migrate.md) — apply pending migrations.
- [status](./status.md) — human-readable view of pending vs applied vs drift vs orphan.
- [commit](./commit.md) — record a file in history without running SQL (can create `drift` if misused; validate online to detect).
