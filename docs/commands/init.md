# init Command

The `init` command initialises a new makemigrations project. By default it sets up the **Go migration framework** — a compiled, type-safe migration binary that lives alongside your application. A legacy YAML-to-SQL workflow is available via the `--sql` flag.

## Overview

Running `makemigrations init` bootstraps everything needed to start writing Go-based migrations:

- Creates the `migrations/` directory
- Generates `migrations/main.go` — the entry point for the compiled migration binary
- Generates `migrations/go.mod` — a dedicated module that imports `github.com/ocomsoft/makemigrations/migrate`
- **If `*.sql` Goose migration files are detected**, automatically runs `migrate-to-go` to convert them to Go migrations (see [Auto-upgrade from Goose SQL migrations](#auto-upgrade-from-goose-sql-migrations))
- If an existing `migrations/.schema_snapshot.yaml` is found, generates `migrations/0001_initial.go` with `CreateTable` operations for every table already defined in that snapshot, and prints instructions for fake-applying it

If no snapshot is found and no SQL files are present, the command creates an empty setup and prints instructions for generating the first migration.

## Usage

```
makemigrations init [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database` | string | `postgresql` | Target database type — influences go.mod hints and generated config. Supported values: `postgresql`, `mysql`, `sqlite`, `sqlserver` |
| `--verbose` | bool | `false` | Print detailed output during initialisation |
| `--sql` | bool | `false` | Use the legacy YAML-to-SQL workflow instead of generating Go migration files |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to the configuration file |

---

## Go Migration Workflow (Default)

### What Gets Created

```
project/
└── migrations/
    ├── go.mod          # Dedicated module: <project>/migrations
    ├── main.go         # Entry point for the compiled migrate binary
    └── 0001_initial.go # Only created when .schema_snapshot.yaml is found
```

### `migrations/main.go`

The generated entry point reads database connection details from environment variables and runs the compiled CLI:

```go
package main

import (
    "fmt"
    "os"

    m "github.com/ocomsoft/makemigrations/migrate"
)

func main() {
    app := m.NewApp(m.Config{
        DatabaseType: m.EnvOr("DB_TYPE", "postgresql"),
        DatabaseURL:  m.EnvOr("DATABASE_URL", ""),
    })
    if err := app.Run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### `migrations/go.mod`

A self-contained module so migration dependencies are isolated from the main application:

```
module <your-project>/migrations

go 1.25.7     ← matches the parent project's Go version

require (
    github.com/ocomsoft/makemigrations main
)
```

The module name is derived from the parent project's `go.mod`. The Go version is read from the nearest `go.work` or `go.mod` in the parent tree so the binary is always built with a locally-available toolchain.

### `migrations/0001_initial.go` (snapshot import)

When `migrations/.schema_snapshot.yaml` already exists at init time, `init` reads every table defined in the snapshot and produces a migration file with a `CreateTable` operation for each one:

```go
package main

import (
    "github.com/ocomsoft/makemigrations/migrate"
)

func init() {
    migrate.Register(&migrate.Migration{
        Name: "0001_initial",
        Up: func(db migrate.DB) error {
            return migrate.Batch(db,
                migrate.CreateTable("users", func(t *migrate.Table) {
                    t.UUID("id").PrimaryKey()
                    t.String("email", 255).NotNull()
                    t.Timestamp("created_at").Default("CURRENT_TIMESTAMP")
                }),
                migrate.CreateTable("orders", func(t *migrate.Table) {
                    t.UUID("id").PrimaryKey()
                    t.ForeignKey("user_id", "users", "id").NotNull()
                    t.Integer("quantity").Default(1)
                }),
            )
        },
        Down: func(db migrate.DB) error {
            return migrate.Batch(db,
                migrate.DropTable("orders"),
                migrate.DropTable("users"),
            )
        },
    })
}
```

---

## Auto-upgrade from Goose SQL migrations

If `*.sql` files following the Goose naming convention (`00001_name.sql`) are detected inside the `migrations/` directory when `init` is run, it **automatically delegates to `migrate-to-go`** rather than performing a bare scaffold. This means a single command upgrades an existing Goose project to the Go migration framework:

```bash
# migrations/ contains 00001_initial.sql, 00002_add_phone.sql, ...
makemigrations init

# Output:
# Detected 2 Goose SQL migration(s) in migrations — running migrate-to-go...
# ✓ Created migrations/0001_initial.go
# ✓ Created migrations/0002_add_phone.go
# ✓ Created migrations/main.go
# ✓ Created migrations/go.mod
# ✓ Created migrations/0003_schema_state.go (schema-state bootstrap, SchemaOnly)
# ✗ Deleted migrations/00001_initial.sql
# ✗ Deleted migrations/00002_add_phone.sql
```

After this completes:

```bash
# Apply all migrations (build is handled automatically)
makemigrations migrate up

# Or fake them all if the schema is already applied
makemigrations migrate fake 0001_initial
makemigrations migrate fake 0002_add_phone
makemigrations migrate status
```

See the [migrate-to-go command](./migrate_to_go.md) for full documentation of the conversion process.

---

## Examples

### Fresh Project — No Existing Schema

```bash
# Initialise a new Go migration project (PostgreSQL default)
makemigrations init

# Output:
# Created migrations/main.go
# Created migrations/go.mod
#
# Initialization complete. No existing schema found.
#
# To generate your first migration:
#   makemigrations makemigrations --name "initial"
#
# Then build and run:
#   cd migrations && go mod tidy && go build -o migrate .
#   ./migrate up
```

Step-by-step after a fresh init:

```bash
# 1. Generate your first migration
makemigrations makemigrations --name "initial"

# 2. Apply migrations to the database (build is handled automatically)
makemigrations migrate up
```

### Existing Project — Snapshot Found

When a `migrations/.schema_snapshot.yaml` already exists (for example when migrating an existing project to the Go workflow):

```bash
makemigrations init

# Output:
# Created migrations/0001_initial.go (from existing schema snapshot)
# Created migrations/main.go
# Created migrations/go.mod
#
# Your database already has these tables applied.
# Mark this migration as applied without re-running SQL:
#
#   makemigrations migrate fake 0001_initial
```

Step-by-step after a snapshot-based init:

```bash
# 1. Mark the initial migration as already applied (schema already in DB)
makemigrations migrate fake 0001_initial

# 2. Confirm status
makemigrations migrate status
```

### Initialise for a Different Database

```bash
# MySQL
makemigrations init --database mysql

# SQLite
makemigrations init --database sqlite

# SQL Server
makemigrations init --database sqlserver
```

### Verbose Output

```bash
makemigrations init --verbose

# Output includes per-file creation details, snapshot parsing steps,
# and table counts when 0001_initial.go is generated.
```

---

## Post-Init Workflow Summary

| Scenario | Commands |
|----------|----------|
| Fresh project | `makemigrations makemigrations --name "initial"` → `makemigrations migrate up` |
| Existing DB (snapshot found) | `makemigrations migrate fake 0001_initial` → `makemigrations migrate status` |
| Existing Goose SQL migrations | `makemigrations init` auto-converts them → `makemigrations migrate fake <each>` or `makemigrations migrate up` |

---

## Legacy SQL Workflow (`--sql`)

The `--sql` flag opts into the original YAML-to-SQL workflow. No Go files are generated. Use this only if you are maintaining a project that was created before the Go migration framework existed.

### What Gets Created

```
project/
└── migrations/
    ├── makemigrations.config.yaml   # Tool configuration
    └── .schema_snapshot.yaml        # Empty schema state file
```

### Usage

```bash
makemigrations init --sql
makemigrations init --sql --database mysql
```

### Output

```
Created directory: migrations/
Generated: migrations/makemigrations.config.yaml
Generated: migrations/.schema_snapshot.yaml

Next steps:
  1. Edit schema/schema.yaml to define your tables
  2. Run: makemigrations sql-migrations
```

### Generated Configuration File

`migrations/makemigrations.config.yaml` contains database-appropriate defaults:

```yaml
database:
  type: postgresql         # postgresql, mysql, sqlserver, sqlite
  default_schema: public
  quote_identifiers: true

migration:
  directory: migrations
  file_prefix: "20060102150405"
  snapshot_file: .schema_snapshot.yaml
  auto_apply: false
  include_down_sql: true
  review_comment_prefix: "-- REVIEW: "
  rejection_comment_prefix: "-- REJECTED: "
  silent: false
  destructive_operations:
    - table_removed
    - field_removed
    - index_removed
    - table_renamed
    - field_renamed
    - field_modified

schema:
  search_paths: []
  ignore_modules: []
  schema_file_name: schema.yaml
  validate_strict: false

output:
  verbose: false
  color_enabled: true
  timestamp_format: "2006-01-02 15:04:05"
```

### Post-Init SQL Workflow

```bash
# After init --sql, define your schema then generate SQL migrations
makemigrations sql-migrations
```

---

## Error Handling

### Directory Already Exists

```bash
$ makemigrations init
Error: migrations directory already exists

# Remove it and retry, or supply --sql if you want to re-init the config only
rm -rf migrations/
makemigrations init
```

### Invalid Database Type

```bash
$ makemigrations init --database oracle
Error: unsupported database type: oracle

# Supported types:
makemigrations init --database postgresql
makemigrations init --database mysql
makemigrations init --database sqlite
makemigrations init --database sqlserver
```

### Permission Denied

```bash
$ makemigrations init
Error: permission denied creating directory: migrations/

# Ensure the current directory is writable, then retry
chmod 755 .
makemigrations init
```

---

## Best Practices

### Commit Generated Files

All generated files should be committed to version control:

```bash
git add migrations/go.mod migrations/main.go
git add migrations/0001_initial.go   # if generated from snapshot
git commit -m "chore: initialise Go migration framework"
```

### Keep the Migrations Module Tidy

Run `go mod tidy` inside `migrations/` after every new migration file is added so the lock file stays up to date:

```bash
cd migrations && go mod tidy
```

### Rebuild After Changes

The migration binary must be recompiled whenever migration files are added or changed. `makemigrations migrate` handles this automatically, or do it manually:

```bash
cd migrations && go build -o migrate .
```

See the [Manual Build Guide](../manual-migration-build.md) if you need to control `GOWORK` or `GOTOOLCHAIN` explicitly.

---

## See Also

- [migrate command](./migrate.md) — run `up`, `down`, `status`, `fake` etc. without manual builds
- [migrate-to-go command](./migrate_to_go.md) — convert Goose SQL migrations to Go
- [Manual Build Guide](../manual-migration-build.md) — build the binary with explicit GOWORK/GOTOOLCHAIN
- [makemigrations Command](./makemigrations.md) — Generate a new migration file
- [Configuration Guide](../configuration.md) — Full configuration reference
- [Schema Format Guide](../schema-format.md) — YAML schema syntax
