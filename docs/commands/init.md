# init Command

The `init` command initialises a new morphic project. By default it sets up the **Go migration framework** — type-safe migration `.go` files that the morphic CLI runs in-process via the [yaegi](https://github.com/traefik/yaegi) Go interpreter. A legacy YAML-to-SQL workflow is available via the `--sql` flag.

## Overview

Running `morphic init` bootstraps everything needed to start writing Go-based migrations:

- Creates the `migrations/` directory
- Generates `migrations/go.mod` — a dedicated module that imports `github.com/ocomsoft/morphic/migrate`. This is what gives your IDE / `gopls` type-checking on the migration files; it is **not** consulted at runtime by `morphic migrate`.
- Generates `migrations/main.go` — an **optional** entry point for compiling the migrations directory into a self-contained binary (`go build -o migrate .`). `morphic migrate` does not invoke this `main()`; the file exists purely as a fallback for users who want a standalone binary (e.g. for shipping in a release artifact, or running on a host without morphic installed).
- If an existing `migrations/.schema_snapshot.yaml` is found, generates `migrations/0001_initial.go` with `CreateTable` operations for every table already defined in that snapshot, and prints instructions for fake-applying it

If no snapshot is found the command creates an empty setup and prints instructions for generating the first migration.

## Usage

```
morphic init [flags]
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
| `--config` | string | `migrations/morphic.config.yaml` | Path to the configuration file |

---

## Go Migration Workflow (Default)

### What Gets Created

```
project/
└── migrations/
    ├── go.mod          # Dedicated module: <project>/migrations  (used by IDE/gopls)
    ├── main.go         # Optional standalone-binary entry point  (NOT used by `morphic migrate`)
    └── 0001_initial.go # Only created when .schema_snapshot.yaml is found
```

### `migrations/main.go` (optional)

`morphic migrate` interprets the `.go` files in this directory via yaegi and never invokes `main()`. The generated `main.go` exists so you can `go build` the directory into a self-contained binary if you want one. Its body reads database connection details from environment variables:

```go
package main

import (
    "fmt"
    "os"

    m "github.com/ocomsoft/morphic/migrate"
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
    github.com/ocomsoft/morphic main
)
```

The module name is derived from the parent project's `go.mod`. The Go version is read from the nearest `go.work` or `go.mod` in the parent tree, ensuring `gopls` resolves the same toolchain the rest of your project uses (and that the optional standalone-binary build picks a locally-available toolchain).

### `migrations/0001_initial.go` (snapshot import)

When `migrations/.schema_snapshot.yaml` already exists at init time, `init` reads every table defined in the snapshot and produces a migration file with a `CreateTable` operation for each one:

```go
package main

import (
    "github.com/ocomsoft/morphic/migrate"
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

## Examples

### Fresh Project — No Existing Schema

```bash
# Initialise a new Go migration project (PostgreSQL default)
morphic init

# Output:
# Created migrations/main.go
# Created migrations/go.mod
#
# Initialization complete. No existing schema found.
#
# To generate your first migration:
#   morphic generate --name "initial"
#
# Then run:
#   morphic migrate up
#
# Migrations are interpreted in-process — no Go toolchain required at runtime.
```

Step-by-step after a fresh init:

```bash
# 1. Generate your first migration
morphic generate --name "initial"

# 2. Apply migrations (yaegi loads the .go files in-process; no go build)
morphic migrate up
```

### Existing Project — Snapshot Found

When a `migrations/.schema_snapshot.yaml` already exists (for example when migrating an existing project to the Go workflow):

```bash
morphic init

# Output:
# Created migrations/0001_initial.go (from existing schema snapshot)
# Created migrations/main.go
# Created migrations/go.mod
#
# Your database already has these tables applied.
# Mark this migration as applied without re-running SQL:
#
#   morphic migrate fake 0001_initial
```

Step-by-step after a snapshot-based init:

```bash
# 1. Mark the initial migration as already applied (schema already in DB)
morphic migrate fake 0001_initial

# 2. Confirm status
morphic migrate status
```

### Initialise for a Different Database

```bash
# MySQL
morphic init --database mysql

# SQLite
morphic init --database sqlite

# SQL Server
morphic init --database sqlserver
```

### Verbose Output

```bash
morphic init --verbose

# Output includes per-file creation details, snapshot parsing steps,
# and table counts when 0001_initial.go is generated.
```

---

## Post-Init Workflow Summary

| Scenario | Commands |
|----------|----------|
| Fresh project | `morphic generate --name "initial"` → `morphic migrate up` |
| Existing DB (snapshot found) | `morphic migrate fake 0001_initial` → `morphic migrate status` |

---

## Legacy SQL Workflow (`--sql`)

The `--sql` flag opts into the original YAML-to-SQL workflow. No Go files are generated. Use this only if you are maintaining a project that was created before the Go migration framework existed.

### What Gets Created

```
project/
└── migrations/
    ├── morphic.config.yaml   # Tool configuration
    └── .schema_snapshot.yaml        # Empty schema state file
```

### Usage

```bash
morphic init --sql
morphic init --sql --database mysql
```

### Output

```
Created directory: migrations/
Generated: migrations/morphic.config.yaml
Generated: migrations/.schema_snapshot.yaml

Next steps:
  1. Edit schema/schema.yaml to define your tables
  2. Run: morphic sql-migrations
```

### Generated Configuration File

`migrations/morphic.config.yaml` contains database-appropriate defaults:

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
morphic sql-migrations
```

---

## Error Handling

### Directory Already Exists

```bash
$ morphic init
Error: migrations directory already exists

# Remove it and retry, or supply --sql if you want to re-init the config only
rm -rf migrations/
morphic init
```

### Invalid Database Type

```bash
$ morphic init --database oracle
Error: unsupported database type: oracle

# Supported types:
morphic init --database postgresql
morphic init --database mysql
morphic init --database sqlite
morphic init --database sqlserver
```

### Permission Denied

```bash
$ morphic init
Error: permission denied creating directory: migrations/

# Ensure the current directory is writable, then retry
chmod 755 .
morphic init
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

### Keep the Migrations Module Tidy (optional)

`migrations/go.mod` is consulted by your IDE / `gopls`, not by `morphic migrate` (which uses yaegi and the symbol map shipped with the CLI). If you also use the optional standalone-binary path, run `go mod tidy` after adding new migrations to keep `go.sum` accurate:

```bash
cd migrations && go mod tidy
```

### No Rebuild Step Required

`morphic migrate` reads the latest migration files on every invocation — there is no compile or rebuild step between generating a migration and applying it. If you want a self-contained binary as a fallback, see the [Manual Build Guide](../manual-migration-build.md).

---

## See Also

- [migrate command](./migrate.md) — run `up`, `down`, `status`, `fake` etc. without manual builds
- [Manual Build Guide](../manual-migration-build.md) — optional: compile `migrations/` into a standalone binary (GOWORK/GOTOOLCHAIN guidance)
- [Extending the yaegi Symbol Map](../extending-yaegi-symbols.md) — let interpreted migrations import third-party packages
- [morphic Command](./morphic.md) — Generate a new migration file
- [migrate-to-go command](./migrate_to_go.md) — convert existing Goose SQL migrations to Go
- [Configuration Guide](../configuration.md) — Full configuration reference
- [Schema Format Guide](../schema-format.md) — YAML schema syntax
