# `status`

Print a tab-separated report comparing **migration files on disk** to **rows in the history table**. Use this to see what would run under `migrate` (in `diff` mode), detect tampered files, or spot orphaned history rows.

## Synopsis

```text
sql-migration status [options]
```

No positional arguments. A database connection is always required.

## Output format

The command writes a **TSV-style table** to **stdout** (header row plus one row per reported sequence):

| Column | Meaning |
|--------|---------|
| `SEQ` | Migration sequence number. |
| `STATUS` | One of `pending`, `applied`, `drift`, `orphan` (see below). |
| `NAME` | Migration filename from disk when relevant, or the name stored in history for orphans. |
| `FILE_CHECKSUM` | Lowercase hex MD5 of the file bytes on disk, or `-` if no file. |
| `DB_CHECKSUM` | Checksum from history row, or `-` if not in history. |

### Status values

| Status | Meaning |
|--------|---------|
| `pending` | A file exists for this sequence; there is **no** history row. It would be applied on the next `migrate` in **`diff`** mode. |
| `applied` | A file exists and the history row exists with the **same** checksum as the file. |
| `drift` | A file exists and a history row exists, but **checksums differ** (file changed after it was recorded, or history was updated without matching the file). |
| `orphan` | A history row exists for this sequence but **no** matching file was found in `--migrations-dir`. |

Sequences appear **once** each in the output. Rows are sorted by `SEQ` ascending.

The union of reported sequences is: all sequences present in either the directory scan or the history table.

## Options

Same as [migrate](./migrate.md#migrations-location-and-history-table) for path/table, and [migrate](./migrate.md#database-connection) for the database.

| Long | Short | Env | Default | Description |
|------|-------|-----|---------|-------------|
| `--migrations-dir` | `-r` | `MIGRATE_DIR` | `./migrations` | Scan this directory for `.sql` files. |
| `--migrations-table` | `-t` | — | `migrations` | Read history from this table. |
| *(connection flags)* | | `DB_*` | — | Required; same as `migrate`. |

## Behavior details

1. Ensures the history table exists.
2. Loads all valid migrations from the directory (same rules as `migrate`).
3. Loads all history rows ordered by `sequence`.
4. Builds rows: for each file, compare to history; then append rows for history-only sequences (**orphan**).

No migration SQL is executed.

## Exit status

- **0** — report printed successfully (including empty directory + empty history: header only).
- **Non-zero** — connection failure, invalid table name, invalid migration filenames, or I/O errors.

## Examples

```bash
sql-migration status -D postgres -h 127.0.0.1 -P 5432 -u app -p secret -d appdb -r ./migrations
```

Pipe-friendly (e.g. `column -t` for alignment):

```bash
sql-migration status -D postgres … | column -t -s $'\t'
```

## Related commands

- [validate](./validate.md) — fail fast on checksum mismatch (online) instead of printing `drift`.
- [migrate](./migrate.md) — apply `pending` sequences.
- [commit](./commit.md) — manually align history for one file path.
