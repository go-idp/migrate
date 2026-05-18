# CLI reference — sql-migration

## Synopsis

```text
sql-migration [--help] [--version] <command> [options] [args]
```

Binary name: `sql-migration`  
Default migrations directory: `./migrations`  
Default history table: `migrations`

**Per-command details:** [docs/commands/README.md](commands/README.md) — full docs for `migrate`, `validate`, `status`, and `commit`.

---

## migrate

Apply SQL migrations from disk.

```text
sql-migration migrate [options]
```

**Options**

| Flag | Env | Description |
|------|-----|-------------|
| `-D`, `--driver` | `DB_DRIVER` | `mysql`, `mariadb`, `postgres`, `sqlite3` |
| `--host` | `DB_HOST` | Host |
| `-P`, `--port` | `DB_PORT` | Port |
| `-u`, `--user` | `DB_USER` | User |
| `-p`, `--pass` | `DB_PASS` | Password |
| `-d`, `--database` | `DB_NAME` | Database (SQLite: path) |
| `-r`, `--migrations-dir` | `SQL_MIGRATION_DIR` | Directory of `.sql` files (default `./migrations`) |
| `-t`, `--migrations-table` | — | History table (default `migrations`) |
| `-m`, `--mode` | `SQL_MIGRATION_MODE` | `diff` (default) or `all` |
| `-n`, `--dry-run` | `SQL_MIGRATION_DRY_RUN` | Transational trial run; PG/SQLite only |

**Example**

```bash
sql-migration migrate -D mysql -h 127.0.0.1 -P 3306 -u root -p secret -d app_db
```

---

## validate

Validate migration files; optionally validate database history against files.

```text
sql-migration validate [options]
```

| Flag | Env | Description |
|------|-----|-------------|
| `-o`, `--offline` | — | Only check files under `-r`; no DB |
| `-r`, `--migrations-dir` | `SQL_MIGRATION_DIR` | Migrations directory |
| `-t`, `--migrations-table` | — | History table |
| *(DB flags)* | `DB_*` | Required when not `--offline` |

**Examples**

```bash
sql-migration validate -o -r ./migrations
sql-migration validate -D postgres … -r ./migrations
```

---

## status

Print migration status for each known sequence (disk ∪ history).

```text
sql-migration status [options]
```

Requires database + `-r`. Output columns: `SEQ`, `STATUS`, `NAME`, `FILE_CHECKSUM`, `DB_CHECKSUM`.

**Statuses**

- `pending` — file exists, not in history  
- `applied` — file and history checksum match  
- `drift` — file and history disagree on checksum  
- `orphan` — history row for a sequence with no file on disk  

---

## commit

Upsert history for **one** SQL file **without** executing its contents.

```text
sql-migration commit [options] <sql-file-path>
```

| Flag | Env | Description |
|------|-----|-------------|
| `-t`, `--migrations-table` | — | History table |
| *(DB flags)* | `DB_*` | Required |

The file basename must follow `<sequence>_<module>_<desc>.<version>.sql`.

**Example**

```bash
sql-migration commit ./deploy/12_api_ratelimit.v2026.05.14.sql -D postgres …
```

---

## Migration filenames

Pattern:

```text
<sequence>_<module>_<business_desc>.<version>.sql
```

Example: `10_billing_invoice_index.v2026.05.06.sql`

---

## History table

Created automatically when needed. Stores `sequence`, `version_tag`, `name`, `checksum` (MD5 hex), `executed_at`.
