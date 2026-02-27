# makemigrations Command

The `makemigrations` command is the **primary command** for generating Go-based database migrations from YAML schema definitions. It implements a Django-style migration workflow where each migration is a typed Go file registered in a DAG (directed acyclic graph).

## Overview

The `makemigrations` command compares the desired schema (defined in YAML files) against the current schema (reconstructed by replaying all registered Go migration files) and generates a new `.go` migration file containing typed operations for each detected change.

Unlike the SQL-mode commands, Go migrations are compiled into a standalone binary (`./migrations/migrate`) that manages migration state, runs `up`/`down` operations, and emits the DAG structure for introspection.

## Usage

```bash
makemigrations makemigrations [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--check` | bool | `false` | Exit with error code 1 if migrations are needed (CI/CD mode) |
| `--dry-run` | bool | `false` | Print generated migration source without writing a file |
| `--merge` | bool | `false` | Generate a merge migration for detected concurrent branches |
| `--name` | string | auto-generated | Custom name suffix for the migration file |
| `--verbose` | bool | `false` | Show detailed pipeline output |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## How It Works

The command runs a five-step pipeline each time it is invoked.

### Step 1 — Scan for existing Go migration files

The command scans the `migrations/` directory (as configured) for `*.go` files, excluding `main.go`. If no migration files exist, the current schema state is treated as empty.

### Step 2 — Query the DAG for the current schema state

When migration files exist, the command:

1. Compiles all `*.go` files in the migrations directory into a temporary binary using `go build`.
2. Executes `<binary> dag --format json` to retrieve `DAGOutput` — a JSON structure containing:
   - The full migration graph (names, dependencies, operations)
   - The reconstructed `SchemaState` (all tables, fields, and indexes after replaying every migration in topological order)
   - The list of leaf migrations (the "tips" of the graph that a new migration must depend on)
   - Whether the graph has branches (concurrent development)

The temporary binary is discarded after the query.

### Step 3 — Parse the YAML schema

The command parses `schema/schema.yaml` (and any files it includes) to produce the **desired** schema state. This uses the same YAML parser as all other `makemigrations` commands.

### Step 4 — Diff the two schemas

The diff engine compares:
- **Previous state**: the `SchemaState` reconstructed from the DAG (or empty if no migrations exist)
- **Current state**: the desired schema from YAML

Detected changes include table additions, removals, renames, field additions, removals, modifications, renames, and index additions and removals.

### Step 5 — Generate or check

Depending on the flags:
- **`--check`**: If any changes are detected, exit with error code 1. No file is written.
- **`--merge`**: Generate a merge migration (see [Branch and Merge Workflow](#branch-and-merge-workflow)).
- **Default**: Generate a new `.go` migration file in the migrations directory.

## Generated File Format

Each generated file is in `package main` and calls `m.Register()` from an `init()` function. This ensures the migration is automatically registered when the migrations binary starts.

```go
// migrations/0001_initial.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0001_initial",
        Dependencies: []string{},
        Operations: []m.Operation{
            &m.CreateTable{
                Name: "users",
                Fields: []m.Field{
                    {Name: "id", Type: "uuid", PrimaryKey: true, Nullable: true},
                    {Name: "email", Type: "varchar", Length: 255, Nullable: true},
                    {Name: "created_at", Type: "timestamp", AutoCreate: true, Nullable: true},
                },
                Indexes: []m.Index{
                    {Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
                },
            },
        },
    })
}
```

### File Naming Convention

```
migrations/NNNN_name.go
```

Where `NNNN` is a zero-padded four-digit sequence number based on the count of existing migration files, and `name` is either the `--name` flag value (lowercased, spaces replaced with underscores) or a name auto-generated from the diff content.

Examples:
- `migrations/0001_initial.go`
- `migrations/0002_add_products.go`
- `migrations/0003_rename_user_email.go`
- `migrations/0004_merge.go` (merge migration)

## Operation Types

There are 10 typed operation types. Each operation implements `Up()` (forward SQL), `Down()` (reverse SQL), and `Mutate()` (updates the in-memory schema state for DAG traversal).

### CreateTable

Creates a new database table with the specified fields and indexes.

```go
&m.CreateTable{
    Name: "products",
    Fields: []m.Field{
        {Name: "id", Type: "uuid", PrimaryKey: true, Nullable: true},
        {Name: "name", Type: "varchar", Length: 255, Nullable: true},
        {Name: "price", Type: "decimal", Precision: 10, Scale: 2, Nullable: true},
        {Name: "active", Type: "boolean", Default: "true", Nullable: true},
        {Name: "created_at", Type: "timestamp", AutoCreate: true, Nullable: true},
        {Name: "updated_at", Type: "timestamp", AutoUpdate: true, Nullable: true},
    },
    Indexes: []m.Index{
        {Name: "idx_products_name", Fields: []string{"name"}, Unique: false},
    },
},
```

- **Destructive**: No
- **Down**: emits `DROP TABLE`

### DropTable

Drops an existing database table.

```go
&m.DropTable{Name: "old_sessions"},
```

- **Destructive**: Yes — all data in the table is lost
- **Down**: reconstructs `CREATE TABLE` from the pre-drop schema state

### RenameTable

Renames an existing table.

```go
&m.RenameTable{OldName: "users", NewName: "accounts"},
```

- **Destructive**: No
- **Down**: emits the reverse rename

### AddField

Adds a new column to an existing table.

```go
&m.AddField{
    Table: "users",
    Field: m.Field{
        Name:     "phone",
        Type:     "varchar",
        Length:   20,
        Nullable: true,
    },
},
```

- **Destructive**: No
- **Down**: emits `DROP COLUMN`

### DropField

Removes a column from an existing table.

```go
&m.DropField{Table: "users", Field: "legacy_token"},
```

- **Destructive**: Yes — all data in that column is lost
- **Down**: reconstructs `ADD COLUMN` from the pre-drop schema state

### AlterField

Changes a column's type, length, nullability, default, or other constraints. Both the old and new field definitions are stored so the operation can be reversed exactly.

```go
&m.AlterField{
    Table: "users",
    OldField: m.Field{Name: "status", Type: "varchar", Length: 50, Nullable: true},
    NewField: m.Field{Name: "status", Type: "varchar", Length: 100, Nullable: true},
},
```

- **Destructive**: No (though incompatible type changes may fail at the database level)
- **Down**: emits the reverse `ALTER COLUMN` restoring the old definition

### RenameField

Renames a column in an existing table.

```go
&m.RenameField{Table: "users", OldName: "username", NewName: "display_name"},
```

- **Destructive**: No
- **Down**: emits the reverse rename

### AddIndex

Creates an index on one or more columns of an existing table.

```go
&m.AddIndex{
    Table: "orders",
    Index: m.Index{
        Name:   "idx_orders_user_id",
        Fields: []string{"user_id", "created_at"},
        Unique: false,
    },
},
```

- **Destructive**: No
- **Down**: emits `DROP INDEX`

### DropIndex

Drops an index from a table.

```go
&m.DropIndex{Table: "orders", Index: "idx_orders_legacy"},
```

- **Destructive**: No (index can be recreated)
- **Down**: reconstructs `CREATE INDEX` from the pre-drop schema state

### RunSQL

Executes raw SQL directly. Used for data migrations, custom constraints, triggers, or any operation that cannot be expressed as a typed operation. `RunSQL` does not update the schema state.

```go
&m.RunSQL{
    ForwardSQL:  "UPDATE users SET status = 'active' WHERE status IS NULL;",
    BackwardSQL: "UPDATE users SET status = NULL WHERE status = 'active';",
},
```

- **Destructive**: No (depends entirely on the SQL content)
- **Down**: executes `BackwardSQL`
- **Note**: `RunSQL` operations are not auto-generated by the diff engine. Add them manually when needed.

## Field Type Reference

The `m.Field` struct supports the following properties:

| Property | Type | Description |
|----------|------|-------------|
| `Name` | string | Column name (required) |
| `Type` | string | Column type: `varchar`, `text`, `integer`, `bigint`, `boolean`, `uuid`, `timestamp`, `date`, `decimal`, `json`, `jsonb`, `foreign_key` |
| `PrimaryKey` | bool | Mark as primary key |
| `Nullable` | bool | Allow NULL values |
| `Default` | string | Default value reference: `"new_uuid"`, `"now"`, `"true"`, `"false"` |
| `Length` | int | Character length for `varchar` |
| `Precision` | int | Total digits for `decimal`/`numeric` |
| `Scale` | int | Decimal places for `decimal`/`numeric` |
| `AutoCreate` | bool | Automatically set on row creation (`created_at` pattern) |
| `AutoUpdate` | bool | Automatically set on row update (`updated_at` pattern) |
| `ForeignKey` | `*m.ForeignKey` | Foreign key constraint |
| `ManyToMany` | `*m.ManyToMany` | Many-to-many relationship via junction table |

### ForeignKey

```go
m.Field{
    Name: "user_id",
    Type: "foreign_key",
    ForeignKey: &m.ForeignKey{
        Table:    "users",
        OnDelete: "CASCADE",
    },
},
```

## Examples

### Basic Usage

```bash
# Generate a migration from detected schema changes
makemigrations makemigrations

# Output (when changes are detected)
Created migrations/0002_add_products.go

# Output (when no changes are detected)
No changes detected.
```

### With a Custom Name

```bash
makemigrations makemigrations --name "add_products"
# Generates: migrations/0002_add_products.go

makemigrations makemigrations --name "Add User Preferences"
# Generates: migrations/0003_add_user_preferences.go
```

### Dry Run

Preview the generated Go source without writing a file:

```bash
makemigrations makemigrations --dry-run
```

```go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0002_add_products",
        Dependencies: []string{"0001_initial"},
        Operations: []m.Operation{
            &m.CreateTable{
                Name: "products",
                Fields: []m.Field{
                    {Name: "id", Type: "uuid", PrimaryKey: true, Nullable: true},
                    {Name: "name", Type: "varchar", Length: 255, Nullable: true},
                },
            },
        },
    })
}
```

### CI/CD Check Mode

```bash
makemigrations makemigrations --check

# Exit codes:
# 0 — schema is up to date with all migrations
# 1 — migrations are needed or an error occurred
```

### Verbose Output

```bash
makemigrations makemigrations --verbose

# Output
Building migration binary from migrations/...
No changes detected.
```

## Full Example Workflow

### Starting a New Project

```bash
# 1. Initialise the migrations directory
makemigrations init-go

# 2. Edit the schema
vim schema/schema.yaml

# 3. Generate the first migration
makemigrations makemigrations --name "initial"
# Created migrations/0001_initial.go

# 4. Build and run the migrations binary
cd migrations && go mod tidy && go build -o migrate .

# 5. Apply migrations
./migrations/migrate up
```

### Adding a New Table

```bash
# 1. Add the 'products' table to schema/schema.yaml

# 2. Generate the migration
makemigrations makemigrations --name "add_products"
# Created migrations/0002_add_products.go

# 3. Review the generated file
cat migrations/0002_add_products.go

# 4. Rebuild the binary
cd migrations && go build -o migrate .

# 5. Apply
./migrations/migrate up
```

### Altering an Existing Field

```bash
# 1. Change 'status' field from varchar(50) to varchar(100) in schema/schema.yaml

# 2. Generate
makemigrations makemigrations --name "expand_user_status"
# Created migrations/0003_expand_user_status.go

# 3. Build and apply
cd migrations && go build -o migrate . && ./migrate up
```

## Branch and Merge Workflow

When two developers generate migrations from the same parent migration concurrently, the DAG gains two leaf nodes — a branching structure. The command detects this automatically.

### Detecting Branches

```bash
makemigrations makemigrations

# Output when branches are detected
WARNING: Branches detected: 0002_add_products, 0002_add_orders
Run 'makemigrations makemigrations --merge' to generate a merge migration.
```

### Generating a Merge Migration

A merge migration has two (or more) entries in `Dependencies` and an empty `Operations` list. It unifies the branches into a single leaf so subsequent migrations have one clear parent.

```bash
makemigrations makemigrations --merge
# Created merge migration: migrations/0003_merge_0002_add_products_and_0002_add_orders.go
# Dependencies: 0002_add_products, 0002_add_orders
```

The generated file looks like:

```go
// migrations/0003_merge_0002_add_products_and_0002_add_orders.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name: "0003_merge_0002_add_products_and_0002_add_orders",
        Dependencies: []string{
            "0002_add_products",
            "0002_add_orders",
        },
        Operations: []m.Operation{},
    })
}
```

After the merge migration is committed, both branches can apply `./migrate up` in any order. The merge node ensures the graph remains acyclic with a single leaf.

### Merge with Dry Run

```bash
makemigrations makemigrations --merge --dry-run
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/check-migrations.yml
name: Check Migrations
on: [push, pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Install makemigrations
        run: go install github.com/ocomsoft/makemigrations@latest
      - name: Check for pending migrations
        run: makemigrations makemigrations --check
```

### Shell Script

```bash
#!/bin/bash
# dev-migrate.sh
set -e

echo "Checking for schema changes..."
if makemigrations makemigrations --check 2>/dev/null; then
    echo "No migrations needed"
else
    echo "Generating migrations..."
    makemigrations makemigrations --verbose

    echo "Rebuilding migration binary..."
    cd migrations && go build -o migrate .

    echo "Applying migrations..."
    ./migrate up

    echo "Done"
fi
```

## The Migrations Directory Structure

After initialisation and several generated migrations, the `migrations/` directory looks like:

```
migrations/
├── go.mod              # Module file: myproject/migrations
├── go.sum
├── main.go             # Entry point — runs the migrate app
├── 0001_initial.go     # Auto-generated
├── 0002_add_products.go
├── 0003_expand_user_status.go
└── migrate             # Compiled binary (gitignored)
```

### main.go

`main.go` is generated once by `makemigrations init-go` and must not be deleted:

```go
package main

import (
    "fmt"
    "os"

    m "github.com/ocomsoft/makemigrations/migrate"
)

func main() {
    app := m.NewApp(m.Config{
        Registry: m.GlobalRegistry(),
    })
    if err := app.Run(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### go.mod

`go.mod` is also generated once and pins the `makemigrations` runtime:

```
module myproject/migrations

go 1.24

require (
    github.com/ocomsoft/makemigrations v0.3.0
)
```

## After Generating a Migration

Every time a new migration file is generated you must rebuild the binary before applying:

```bash
cd migrations && go mod tidy && go build -o migrate .
./migrations/migrate up
```

To verify the migration was applied:

```bash
./migrations/migrate status
```

To roll back the last migration:

```bash
./migrations/migrate down
```

To view the full DAG:

```bash
./migrations/migrate dag
./migrations/migrate dag --format json
```

## Configuration Integration

The command reads `migrations/makemigrations.config.yaml`:

```yaml
database:
  type: postgresql          # Target database: postgresql, mysql, sqlite, sqlserver

migration:
  directory: migrations     # Where .go migration files are written
```

## Error Handling

### Common Errors

**No schema files found**
```
Error: parsing YAML schema: no schema files found
```
Create `schema/schema.yaml` or check the search paths.

**Build failure in migrations directory**
```
Error: querying migration DAG: building migration binary: ...
```
Run `cd migrations && go mod tidy && go build -o migrate .` manually to see the compiler error. Often caused by a missing `go.sum` entry after adding dependencies.

**Missing dependency**
```
Error: querying migration DAG: running dag command: migration "0003_add_orders" depends on "0002_missing" which is not registered
```
A migration file references a dependency that does not exist. Check the `Dependencies` field in the affected migration file.

**Branches detected without --merge**
```
WARNING: Branches detected: 0002_add_products, 0002_add_orders
Run 'makemigrations makemigrations --merge' to generate a merge migration.
```
Run with `--merge` to resolve.

**Check mode failure**
```
Error: migrations needed: 3 changes detected
```
Exit code 1. Schema and migrations are out of sync. Generate the migration and commit it.

## See Also

- [init-go Command](./init.md) — Initialise the migrations directory for Go migrations
- [Schema Format Guide](../schema-format.md) — Complete YAML schema reference
- [Configuration Guide](../configuration.md) — Configuration options
- [Architecture Guide](../architecture.md) — How the DAG and migration framework work
