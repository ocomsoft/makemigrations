# Design: `makemigrations migrate-to-go`

**Date:** 2026-02-28
**Status:** Approved

## Problem

Teams using the legacy Goose/YAML workflow need a migration path to the Go migration framework. This requires:

1. Converting existing Goose `.sql` migration files into typed `.go` migration files (using `RunSQL` wrappers)
2. Migrating the Goose history table (`goose_db_version`) to the Go framework history table (`makemigrations_history`)
3. Cleaning up the old `.sql` files once the transition is complete

## Command

```
makemigrations migrate-to-go [flags]
```

## Detection

The command auto-detects the old-style workflow by scanning the `migrations/` directory (from `makemigrations.config.yaml`) for:

- `*.sql` files — the Goose migrations to convert (required; errors if none found)
- `.schema_snapshot.yaml` — confirms old-style workflow; kept in place after migration (still used by `makemigrations makemigrations` for future diffs)

**Guards:**
- No `*.sql` files found → error: "no SQL migration files found in migrations/"
- `.go` migration files already exist → error with suggestion to use `--force` to overwrite

## File Conversion

### Sorting

SQL files are sorted alphabetically. This preserves correct order for both Goose naming conventions:
- Timestamp style: `20240101120000_description.sql`
- Sequential style: `00001_description.sql`

### Naming

The Goose version prefix is stripped and a new 4-digit sequential number is assigned:

```
20240101120000_initial.sql     → 0001_initial.go
20240102120000_add_phone.sql   → 0002_add_phone.go
20240103120000_add_index.sql   → 0003_add_index.go
```

Description is extracted by removing the leading numeric prefix (timestamp or sequential) and the `.sql` extension.

### SQL Parsing

Goose SQL format markers are stripped; raw SQL content is preserved:

- `-- +goose Up` → start of ForwardSQL
- `-- +goose Down` → start of BackwardSQL (empty string if section absent)
- `-- +goose StatementBegin` / `-- +goose StatementEnd` → stripped (markers only)
- Everything else → preserved as SQL content

### Generated File Format

```go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0002_add_phone",
        Dependencies: []string{"0001_initial"},
        Operations: []m.Operation{
            &m.RunSQL{
                ForwardSQL:  `ALTER TABLE users ADD COLUMN phone VARCHAR(20)`,
                BackwardSQL: `ALTER TABLE users DROP COLUMN phone`,
            },
        },
    })
}
```

- First migration: `Dependencies: []string{}`
- Each subsequent migration depends on the previous one (linear chain)
- `main.go` and `go.mod` are generated if not already present (same as `init`)

## History Migration

### Source: `goose_db_version`

Standard Goose history table. Applied migrations are determined by taking the most recent record per `version_id`:

```sql
SELECT version_id
FROM goose_db_version
WHERE id IN (
    SELECT MAX(id) FROM goose_db_version GROUP BY version_id
)
AND is_applied = true
```

### Mapping

The Goose `version_id` is extracted from the `.sql` filename prefix (e.g. `20240101120000` from `20240101120000_initial.sql`). This is matched against the query results. If a version is applied in Goose, the corresponding new Go migration name (e.g. `0001_initial`) is inserted into `makemigrations_history`.

### Destination: `makemigrations_history`

The Go framework history table is created if it does not exist (via `MigrationRecorder.EnsureTable()`), then rows are inserted for each applied migration.

### Database Connection

Uses the same `MAKEMIGRATIONS_DB_*` environment variables as the `goose` command:

```
MAKEMIGRATIONS_DB_HOST, MAKEMIGRATIONS_DB_PORT, MAKEMIGRATIONS_DB_USER,
MAKEMIGRATIONS_DB_PASSWORD, MAKEMIGRATIONS_DB_NAME, MAKEMIGRATIONS_DB_SSLMODE
```

DB driver is determined from `cfg.Database.Type` in `makemigrations.config.yaml`.

## Deletion

`.sql` files are deleted **only after**:
1. All `.go` files are successfully written to disk
2. History migration completes successfully (or `--no-history` is set)

On any failure, `.sql` files are preserved. `.go` and `.sql` files coexist safely.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview all actions (file creation, history migration, deletions) without executing |
| `--no-history` | `false` | Skip DB history migration step (offline/CI use) |
| `--no-delete` | `false` | Keep `.sql` files even after successful conversion |
| `--force` | `false` | Overwrite existing `.go` migration files |

## Output (success)

```
Detected 3 SQL migration files in migrations/

Converting migrations:
  20240101120000_initial.sql     → migrations/0001_initial.go       ✓
  20240102120000_add_phone.sql   → migrations/0002_add_phone.go     ✓
  20240103120000_add_index.sql   → migrations/0003_add_index.go     ✓

Generated migrations/main.go
Generated migrations/go.mod

Migrating history (goose_db_version → makemigrations_history):
  0001_initial    applied 2024-01-01 12:00:00   ✓
  0002_add_phone  applied 2024-01-02 12:00:00   ✓
  0003_add_index  pending                        -

Removing SQL files:
  migrations/20240101120000_initial.sql     ✓
  migrations/20240102120000_add_phone.sql   ✓
  migrations/20240103120000_add_index.sql   ✓

Migration complete. Next steps:

  cd migrations && go mod tidy && go build -o migrate .
  ./migrate status
```

## Error Handling

| Scenario | Behaviour |
|----------|-----------|
| No `.sql` files found | Error, exit |
| `.go` files exist without `--force` | Error, exit |
| SQL file cannot be parsed | Error, abort (no files written) |
| `.go` file write fails | Error, abort (no deletions) |
| DB connection fails with `--no-history` not set | Error after file generation; `.go` files kept; no deletions |
| `goose_db_version` table not found | Warning: skip history migration; continue to deletion if `--no-history` implied |
| `.sql` deletion fails | Warning per file; continue deleting remaining files |

## Files Created / Modified

| File | Action |
|------|--------|
| `cmd/migrate_to_go.go` | New — Cobra command definition and `runMigrateToGo` entry point |
| `cmd/migrate_to_go_test.go` | New — unit + integration tests |
| `internal/gooseparser/parser.go` | New — Goose SQL file parser (`ParseGooseFile`) |
| `internal/gooseparser/parser_test.go` | New — parser tests |

The `migrate_to_go.go` command reuses:
- `setupGooseDB()` from `cmd/goose.go` for the DB connection
- `codegen.NewGoGenerator()` for `main.go` / `go.mod` generation
- `migrate.NewMigrationRecorder()` for `makemigrations_history` management
- `config.LoadOrDefault()` for configuration

## Testing

- **Unit tests**: Goose SQL parser (`ParseGooseFile`) with all marker variations
- **Integration tests**: temp directory with sample `.sql` files, verify `.go` output and deletions
- **DB tests**: SQLite in-memory DB with mock `goose_db_version` data, verify `makemigrations_history` populated correctly
- **`--dry-run` test**: verify no files written, no DB changes, no deletions
