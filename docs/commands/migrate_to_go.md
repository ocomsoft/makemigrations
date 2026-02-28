# migrate-to-go

Converts legacy Goose `.sql` migration files to Go migration files and migrates
the `goose_db_version` history table to `makemigrations_history`.

## Synopsis

    makemigrations migrate-to-go [flags]

## Detection

Auto-detects `.sql` files in the configured `migrations/` directory.
Stops with an error if none are found or if `.go` migration files already exist
(use `--force` to overwrite existing files).

## What It Does

1. Sorts `.sql` files alphabetically
2. Parses each file's `-- +goose Up` / `-- +goose Down` sections
3. Generates `0001_description.go`, `0002_description.go`, ... with `RunSQL` operations
4. Generates `main.go` and `go.mod` if not present
5. Connects to the database and migrates `goose_db_version` → `makemigrations_history`
6. Deletes the original `.sql` files

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview all actions without writing, migrating, or deleting |
| `--no-history` | `false` | Skip the database history migration step |
| `--no-delete` | `false` | Keep `.sql` files after successful conversion |
| `--force` | `false` | Overwrite existing `.go` migration files |

## Database Connection

Uses the same `MAKEMIGRATIONS_DB_*` environment variables as `makemigrations goose`:

    MAKEMIGRATIONS_DB_HOST, MAKEMIGRATIONS_DB_PORT, MAKEMIGRATIONS_DB_USER,
    MAKEMIGRATIONS_DB_PASSWORD, MAKEMIGRATIONS_DB_NAME, MAKEMIGRATIONS_DB_SSLMODE

Set these before running (or use `--no-history` to skip the DB step).

If `goose_db_version` does not exist in the database, the history migration step
is skipped with a warning and the conversion continues.

## Example

    export MAKEMIGRATIONS_DB_HOST=localhost
    export MAKEMIGRATIONS_DB_USER=myuser
    export MAKEMIGRATIONS_DB_PASSWORD=secret
    export MAKEMIGRATIONS_DB_NAME=mydb

    makemigrations migrate-to-go

    # Then build and verify
    cd migrations && go mod tidy && go build -o migrate .
    ./migrate status

## Preview Mode

Use `--dry-run` to see what would happen without making any changes:

    makemigrations migrate-to-go --dry-run --no-history

## Offline / CI Use

Use `--no-history` to skip the database step entirely (useful in CI or when
migrating the schema files without database access):

    makemigrations migrate-to-go --no-history

## After Migration

The `.schema_snapshot.yaml` is preserved — it is still used by
`makemigrations makemigrations` to diff against future schema changes.
