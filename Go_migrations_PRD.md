# Product Requirements Document: Django-Style Migration Framework

## Overview

Replace the current SQL-file generation pipeline with a Django-style migration framework that generates **compiled Go migration files** with a dependency graph (DAG), typed operations, merge migrations, and state reconstruction. The compiled migration binary becomes the single source of truth — eliminating snapshot caches, comment headers, and any secondary state tracking.

This elevates `makemigrations` from "SQL file generator" to the first Go migration framework with full Django parity: dependency DAGs, branch detection, merge migrations, squashable operations, and compiled standalone binaries.

## Motivation

### Current Limitations

The existing tool generates Goose-compatible `.sql` files from YAML schema diffs. This works but has fundamental constraints:

- **Linear versioning only** — no dependency graph, no branch-awareness
- **No merge migrations** — team members creating migrations concurrently causes ordering conflicts
- **No squash capability** — migration directories grow indefinitely
- **No state reconstruction** — relies on a flat `.schema_snapshot.yaml` file that can drift from reality
- **Goose coupling** — the runner is a thin wrapper around Goose, limiting control over execution
- **No operation-level abstraction** — changes are raw SQL strings, not inspectable or transformable objects

### Gap in the Go Ecosystem

No existing Go migration tool provides Django's full feature set:

| Feature | Atlas | Goose | golang-migrate | This Proposal |
|---|---|---|---|---|
| Declarative schema → auto-diff | ✅ (HCL/SQL) | ✗ | ✗ | ✅ (YAML) |
| Typed operations model | ✗ (raw SQL) | ✗ | ✗ | **✅** |
| Dependency DAG | ✗ (linear) | ✗ (linear) | ✗ (linear) | **✅** |
| Merge migrations | ✗ (rebase only) | ✗ | ✗ | **✅** |
| Squash migrations | ✗ | ✗ | ✗ | **✅** |
| State reconstruction via replay | ✗ | ✗ | ✗ | **✅** |
| Generated code (not hand-written) | ✗ | ✗ | ✗ | **✅** |
| Compiled migration binary | ✗ | ✗ | ✗ | **✅** |
| No dev database required for diffing | ✗ | N/A | N/A | **✅** |

Atlas is the closest competitor but fundamentally different: it treats branches as errors requiring rebase, not a first-class workflow feature. It generates raw SQL (no operation-level abstraction), requires a dev database for diffing, and is increasingly moving key features behind commercial licensing.

## Architecture

### Core Concept: The Compiled Binary Is the Source of Truth

The `migrations/` directory is its own Go module. Each migration is a `.go` file containing typed operations that self-register via `init()`. Compiling the directory produces a standalone binary that can report its own DAG, reconstruct schema state, and execute migrations — with zero external dependencies.

```
makemigrations makemigrations --name "add_phone"

Internally:
  1. go build ./migrations -o /tmp/migrate
  2. /tmp/migrate dag --format json > /tmp/dag_tree.json
  3. Parse dag_tree.json → know leaves, full graph, current schema state
  4. Diff YAML schema against reconstructed state
  5. Generate new 0003_add_phone.go
```

No snapshot cache. No comment headers. No AST parsing. The binary tells you everything.

### Project Structure

```
myproject/
├── schema/
│   └── schema.yaml                     # Declarative YAML schema
├── migrations/
│   ├── go.mod                          # Own module: depends on migrate library
│   ├── go.sum
│   ├── main.go                         # Generated once — wires CLI, never changes
│   ├── 0001_initial.go                 # Generated migration
│   ├── 0002_add_user_phone.go          # Generated migration
│   ├── 0003_feature_a.go              # Developer A's branch
│   ├── 0003_feature_b.go              # Developer B's branch
│   ├── 0004_merge.go                  # Auto-generated merge
│   └── 0005_backfill_slugs.go         # Hand-written data migration (RunSQL)
├── go.mod                              # Application module
└── main.go
```

The `migrations/go.mod` depends on a single library:

```
module myproject/migrations

go 1.24

require (
    github.com/ocomsoft/makemigrations/migrate v0.3.0
)
```

### Two Deployment Artifacts

**`github.com/ocomsoft/makemigrations`** — the CLI tool (developer dependency only):
- YAML parsing, schema diffing, code generation
- All existing `internal/` packages, refactored
- Only needed to *generate* new migration files

**`github.com/ocomsoft/makemigrations/migrate`** — the runtime library (production dependency):
- Types, operations, registry, graph, runner, recorder
- Small, stable public API that generated code imports
- Needed to *compile and run* migrations

In CI/CD, you do not need the `makemigrations` tool. You just `go build ./migrations && ./migrate up`.

## Generated Migration Files

### Standard Migration

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
                    {Name: "id", Type: "uuid", PrimaryKey: true, Default: "new_uuid"},
                    {Name: "email", Type: "varchar", Length: 255, Nullable: false},
                    {Name: "display_name", Type: "varchar", Length: 100, Nullable: true},
                    {Name: "is_active", Type: "boolean", Default: "true"},
                    {Name: "created_at", Type: "timestamp", Default: "now", AutoCreate: true},
                    {Name: "updated_at", Type: "timestamp", Default: "now", AutoUpdate: true},
                },
                Indexes: []m.Index{
                    {Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
                },
            },
            &m.CreateTable{
                Name: "posts",
                Fields: []m.Field{
                    {Name: "id", Type: "uuid", PrimaryKey: true, Default: "new_uuid"},
                    {Name: "title", Type: "varchar", Length: 200, Nullable: false},
                    {Name: "body", Type: "text"},
                    {Name: "user_id", Type: "foreign_key", ForeignKey: &m.ForeignKey{
                        Table: "users", OnDelete: "CASCADE",
                    }},
                },
            },
        },
    })
}
```

### Incremental Change

```go
// migrations/0002_add_user_phone.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0002_add_user_phone",
        Dependencies: []string{"0001_initial"},
        Operations: []m.Operation{
            &m.AddField{
                Table: "users",
                Field: m.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
            },
            &m.AddIndex{
                Table: "users",
                Index: m.Index{Name: "idx_users_phone", Fields: []string{"phone"}},
            },
        },
    })
}
```

### Merge Migration

```go
// migrations/0004_merge.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0004_merge_feature_a_and_b",
        Dependencies: []string{"0003_feature_a", "0003_feature_b"},
        Operations:   []m.Operation{},
    })
}
```

### Hand-Written Data Migration (Escape Hatch)

```go
// migrations/0005_backfill_slugs.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0005_backfill_slugs",
        Dependencies: []string{"0004_merge_feature_a_and_b"},
        Operations: []m.Operation{
            &m.RunSQL{
                Forward:  "UPDATE posts SET slug = lower(replace(title, ' ', '-')) WHERE slug IS NULL",
                Backward: "UPDATE posts SET slug = NULL",
            },
        },
    })
}
```

### Generated `main.go` (Created Once by `init`)

```go
// migrations/main.go
package main

import (
    "os"
    m "github.com/ocomsoft/makemigrations/migrate"
)

func main() {
    app := m.NewApp(m.Config{
        DatabaseType: m.EnvOr("MAKEMIGRATIONS_DATABASE_TYPE", "postgresql"),
        DatabaseURL:  m.EnvOr("DATABASE_URL", ""),
        DBHost:       m.EnvOr("MAKEMIGRATIONS_DB_HOST", "localhost"),
        DBPort:       m.EnvOr("MAKEMIGRATIONS_DB_PORT", "5432"),
        DBUser:       m.EnvOr("MAKEMIGRATIONS_DB_USER", "postgres"),
        DBPassword:   m.EnvOr("MAKEMIGRATIONS_DB_PASSWORD", ""),
        DBName:       m.EnvOr("MAKEMIGRATIONS_DB_NAME", ""),
        DBSSLMode:    m.EnvOr("MAKEMIGRATIONS_DB_SSLMODE", "disable"),
    })

    if err := app.Run(os.Args[1:]); err != nil {
        os.Exit(1)
    }
}
```

## The DAG Output

The compiled binary's `dag` command is the mechanism by which the `makemigrations` CLI queries existing migration state. It produces both machine-parseable JSON and human-readable ASCII.

### JSON Format (`./migrate dag --format json`)

```json
{
  "migrations": [
    {
      "name": "0001_initial",
      "dependencies": [],
      "operations": [
        {"type": "create_table", "table": "users", "description": "Create table users (6 fields)"},
        {"type": "create_table", "table": "posts", "description": "Create table posts (4 fields)"}
      ]
    },
    {
      "name": "0002_add_phone",
      "dependencies": ["0001_initial"],
      "operations": [
        {"type": "add_field", "table": "users", "field": "phone", "description": "Add varchar(20) field phone to users"},
        {"type": "add_index", "table": "users", "index": "idx_users_phone", "description": "Add index idx_users_phone on users(phone)"}
      ]
    }
  ],
  "roots": ["0001_initial"],
  "leaves": ["0002_add_phone"],
  "has_branches": false,
  "schema_state": {
    "tables": [
      {
        "name": "users",
        "fields": [
          {"name": "id", "type": "uuid", "primary_key": true, "default": "new_uuid"},
          {"name": "email", "type": "varchar", "length": 255, "nullable": false},
          {"name": "phone", "type": "varchar", "length": 20, "nullable": true}
        ],
        "indexes": [
          {"name": "idx_users_email", "fields": ["email"], "unique": true},
          {"name": "idx_users_phone", "fields": ["phone"]}
        ]
      }
    ]
  }
}
```

The `schema_state` is the full reconstructed state after replaying all operations through the graph. This is what `makemigrations` diffs against the current YAML schema.

### ASCII Format (`./migrate dag`)

```
Migration Graph
===============

  0001_initial
  │  Create table users (6 fields)
  │  Create table posts (4 fields)
  │
  └─► 0002_add_phone
     │  Add field users.phone varchar(20)
     │  Add index idx_users_phone on users(phone)
     │
     ├─► 0003_feature_a
     │    Create table categories (3 fields)
     │
     └─► 0003_feature_b
          Add field posts.published_at timestamp

Roots:  0001_initial
Leaves: 0003_feature_a, 0003_feature_b
⚠ Branches detected — run makemigrations --merge
```

## The `migrate` Library Package

Public API shipped as `github.com/ocomsoft/makemigrations/migrate`.

### Package Layout

```
migrate/
├── app.go           # CLI app (up, down, status, showsql, dag)
├── types.go         # Migration, Field, Index, ForeignKey structs
├── operations.go    # Operation interface + all concrete types
├── registry.go      # Global registry populated by init() calls
├── graph.go         # DAG, topological sort, branch detection, state reconstruction
├── state.go         # In-memory SchemaState for operation replay
├── runner.go        # Executes migrations against a database
├── recorder.go      # Manages makemigrations_history table
└── providers.go     # Re-exports existing provider layer
```

### Types (`types.go`)

```go
type Migration struct {
    Name         string
    Dependencies []string
    Operations   []Operation
    Replaces     []string    // for squashed migrations
}

type Field struct {
    Name       string
    Type       string
    PrimaryKey bool
    Nullable   bool
    Default    string
    Length     int
    Precision  int
    Scale      int
    AutoCreate bool
    AutoUpdate bool
    ForeignKey *ForeignKey
    ManyToMany *ManyToMany
}

type ForeignKey struct {
    Table    string
    OnDelete string
    OnUpdate string
}

type Index struct {
    Name   string
    Fields []string
    Unique bool
}
```

### Operation Interface (`operations.go`)

```go
type Operation interface {
    Forward(p Provider, state *SchemaState, defaults map[string]string) (string, error)
    Backward(p Provider, state *SchemaState, defaults map[string]string) (string, error)
    Mutate(state *SchemaState) error
    Describe() string
    TypeName() string
    TableName() string
    IsDestructive() bool
}
```

### Operation Types

| Operation | Destructive | Maps From (existing diff engine) |
|---|---|---|
| `CreateTable` | No | `table_added` |
| `DropTable` | **Yes** | `table_removed` |
| `RenameTable` | No | `table_renamed` |
| `AddField` | No | `field_added` |
| `DropField` | **Yes** | `field_removed` |
| `AlterField` | No | `field_modified` |
| `RenameField` | No | `field_renamed` |
| `AddIndex` | No | `index_added` |
| `DropIndex` | No | `index_removed` |
| `RunSQL` | No | *(manual escape hatch)* |

Each operation's `Forward()` method delegates to the existing provider interface — `provider.GenerateCreateTable()`, `provider.GenerateAddColumn()`, etc. The provider layer stays completely untouched.

`Mutate()` applies the operation to an in-memory `SchemaState`, enabling schema reconstruction at any point in the DAG by replaying operations from root to target node.

### Registry (`registry.go`)

```go
var globalRegistry = &Registry{
    migrations: make(map[string]*Migration),
}

func Register(m *Migration) {
    if err := globalRegistry.add(m); err != nil {
        panic(fmt.Sprintf("migration registration error: %v", err))
    }
}
```

Called by each migration file's `init()`. Panics on duplicate names.

### Graph (`graph.go`)

```go
type Graph struct {
    nodes map[string]*node
}

type node struct {
    migration *Migration
    parents   []*node
    children  []*node
}

func BuildGraph(reg *Registry) (*Graph, error)
func (g *Graph) Linearize() ([]*Migration, error)    // Kahn's algorithm — topological sort
func (g *Graph) Leaves() []string                     // Nodes with no children
func (g *Graph) Roots() []string                      // Nodes with no parents
func (g *Graph) DetectBranches() [][]string            // Multiple leaves = branches
func (g *Graph) ReconstructState() (*SchemaState, error) // Replay all operations
func (g *Graph) ToDAGOutput() (*DAGOutput, error)     // JSON-serialisable representation
```

### DAG Output Structure

```go
type DAGOutput struct {
    Migrations  []MigrationSummary `json:"migrations"`
    Roots       []string           `json:"roots"`
    Leaves      []string           `json:"leaves"`
    HasBranches bool               `json:"has_branches"`
    SchemaState *SchemaState       `json:"schema_state"`
}

type MigrationSummary struct {
    Name         string             `json:"name"`
    Dependencies []string           `json:"dependencies"`
    Operations   []OperationSummary `json:"operations"`
}

type OperationSummary struct {
    Type        string `json:"type"`
    Table       string `json:"table,omitempty"`
    Description string `json:"description"`
}
```

### Runner (`runner.go`)

```go
type Runner struct {
    graph    *Graph
    provider Provider
    db       *sql.DB
    recorder *MigrationRecorder
}

func (r *Runner) Migrate() error {
    plan, _ := r.graph.Linearize()
    applied := r.recorder.GetApplied()
    state := NewSchemaState()

    for _, mig := range plan {
        if applied[mig.Name] {
            for _, op := range mig.Operations {
                op.Mutate(state)
            }
            continue
        }

        for _, op := range mig.Operations {
            sql, _ := op.Forward(r.provider, state, defaults)
            if sql != "" {
                r.db.Exec(sql)
            }
            op.Mutate(state)
        }
        r.recorder.RecordApplied(mig.Name)
    }
    return nil
}
```

### Migration History Table

Replaces Goose's `goose_db_version`:

```sql
CREATE TABLE IF NOT EXISTS makemigrations_history (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Stores full migration names (not version numbers), supporting non-linear DAG execution.

## The `makemigrations makemigrations` Flow

```go
func runMakeMigrations(name string, dryRun, check bool) error {
    migrationsDir := getMigrationsDir()

    // 1. Build the migration binary into a temp location
    tmpBin, err := buildMigrationBinary(migrationsDir)
    // If first migration, no binary yet — that's fine
    defer os.Remove(tmpBin)

    // 2. Run it to get the DAG
    var dagOutput DAGOutput
    if tmpBin != "" {
        dagJSON, _ := exec.Command(tmpBin, "dag", "--format", "json").Output()
        json.Unmarshal(dagJSON, &dagOutput)
    }

    // 3. Parse the current YAML schema
    currentSchema, _ := yaml.ParseSchemaDir("schema/")

    // 4. Diff against the reconstructed state from the DAG
    previousSchema := dagOutput.SchemaState.ToYAMLSchema()
    diff, _ := diffEngine.CompareSchemas(previousSchema, currentSchema)
    if !diff.HasChanges {
        fmt.Println("No changes detected.")
        return nil
    }

    if check {
        return fmt.Errorf("migrations needed")
    }

    // 5. Check for branches — auto-merge if needed
    if dagOutput.HasBranches {
        mergeCode := codegen.GenerateMergeMigration(dagOutput.Leaves)
        // write merge file, rebuild binary, re-query DAG
    }

    // 6. Generate the new migration Go file
    migrationName := generateName(name, diff.Changes)
    code := codegen.GenerateMigrationFile(migrationName, dagOutput.Leaves, diff.Changes)

    if dryRun {
        fmt.Println(code)
        return nil
    }

    // 7. Write it
    filepath := filepath.Join(migrationsDir, migrationName+".go")
    os.WriteFile(filepath, []byte(code), 0644)
    fmt.Printf("Created %s\n", filepath)
    return nil
}
```

### Build Cost

On a typical migrations directory with 20-50 small Go files and one dependency, the first build takes 2-3 seconds. Subsequent builds with Go build cache hit take under 500ms. Comparable to Django's `makemigrations` importing all app models and building its graph.

Optional optimisation: cache the binary and only rebuild when a `.go` file has changed (compare mtimes).

## Command Line Interface

### Generation Commands (require `makemigrations` CLI)

```bash
# Generate migration from YAML schema changes
makemigrations makemigrations --name "add_user_phone"

# Preview what would be generated
makemigrations makemigrations --dry-run

# Check for unmigrated changes (CI/CD)
makemigrations makemigrations --check

# Detect branches and generate merge migration
makemigrations makemigrations --merge

# Initialize a new project with migrations/ directory
makemigrations init
```

### Runtime Commands (compiled binary — no external tools needed)

```bash
# Build
cd migrations && go build -o migrate .

# Apply all pending migrations
./migrate up

# Apply up to a specific migration
./migrate up --to 0003_feature_a

# Rollback one migration
./migrate down

# Rollback N migrations
./migrate down --steps 3

# Rollback to a specific migration
./migrate down --to 0001_initial

# Show migration status (applied/pending)
./migrate status

# Print SQL without executing
./migrate showsql

# Show the DAG (ASCII)
./migrate dag

# Show the DAG (JSON — for tooling)
./migrate dag --format json
```

## Impact on Existing Codebase

### Unchanged

| Component | Reason |
|---|---|
| YAML parser (`internal/yaml/parser.go`) | Input format unchanged |
| Diff engine (`internal/yaml/diff.go`) | Change detection unchanged |
| Types (`internal/types/`) | Data model unchanged |
| Providers (`internal/providers/`) | Operations call them at runtime |
| Config (`internal/config/`) | Minor additions for new settings |
| Scanner (`internal/scanner/`) | Module schema discovery unchanged |

### Replaced

| Component | Replaced By |
|---|---|
| SQL converter (`internal/yaml/sql_converter.go` — 1,517 lines) | `internal/codegen/go_generator.go` + operations model |
| State manager (`internal/yaml/state.go`) | Graph-based state reconstruction |
| Writer (`internal/writer/`) | Go file output |
| Goose integration (`cmd/goose.go`) | `migrate/app.go` (compiled into binary) |

### New Packages

| Package | Purpose | Estimated LOC |
|---|---|---|
| `migrate/` (public library) | Types, operations, registry, graph, runner, recorder, app | 1,500–2,000 |
| `internal/codegen/` | Go code generator (template-based) | 300–500 |

### Net Change

The 1,517-line monolithic `sql_converter.go` gets decomposed into the operations model as a natural side effect. Total new code: approximately 2,000–3,000 lines, with the existing codebase shrinking as the SQL converter and Goose wrapper are removed.

## Implementation Phases

### Phase 1: Operations Model + Registry (~500–800 LOC)

Define the `Operation` interface and all concrete types (`CreateTable`, `AddField`, `AlterField`, etc.) with `Forward()`, `Backward()`, `Mutate()`, and `Describe()` methods. Implement the global registry with `Register()` / `init()` pattern. Map directly from existing `ChangeType` constants.

**Deliverable:** `migrate/types.go`, `migrate/operations.go`, `migrate/registry.go`, `migrate/state.go`

### Phase 2: Go Code Generator (~300–500 LOC)

Template-based generator that converts diff engine changes into Go migration source files. Produces valid, `gofmt`-compatible Go code with proper imports. Generates `main.go` during `init`.

**Deliverable:** `internal/codegen/go_generator.go`

### Phase 3: Migration Graph (~400–600 LOC)

DAG construction from registry, Kahn's algorithm for topological sort, cycle detection, root/leaf identification. `ReconstructState()` replays all operations to produce the full schema state. `ToDAGOutput()` produces the JSON-serialisable representation.

**Deliverable:** `migrate/graph.go`

### Phase 4: Branch Detection + Merge Migrations (~200–300 LOC)

Detect when multiple leaves exist (team members created migrations independently). Auto-generate merge migrations with both leaves as dependencies and empty operations.

**Deliverable:** Extensions to `migrate/graph.go`, `internal/codegen/merge_generator.go`

### Phase 5: DAG Command + Binary Query (~300–400 LOC)

Implement the `dag` subcommand with `--format json` and `--format ascii` output. ASCII renderer with tree-drawing characters. Wire the `makemigrations makemigrations` command to build the binary, query it, and parse the JSON output.

**Deliverable:** `migrate/app.go`, `migrate/dag_ascii.go`, updates to `cmd/makemigrations.go`

### Phase 6: Migration Runner (~300–400 LOC)

Execute migrations against a database. Manage the `makemigrations_history` table. Support `up`, `down`, `status`, and `showsql` commands. Delegate SQL generation to existing providers.

**Deliverable:** `migrate/runner.go`, `migrate/recorder.go`

### Phase 7: Squash Migrations (~200–300 LOC)

Given a range of migrations, reconstruct the net operations and generate a single replacement migration with a `Replaces` field listing the originals.

**Deliverable:** `internal/codegen/squash_generator.go`, extensions to `migrate/graph.go`

## Example Workflows

### Solo Developer

```bash
# 1. Edit schema
vim schema/schema.yaml          # Add phone field to users

# 2. Generate migration
makemigrations makemigrations --name "add_phone"
# Created migrations/0002_add_user_phone.go

# 3. Build and apply
cd migrations && go build -o migrate .
./migrate up
# Applying 0002_add_user_phone... done
```

### Team Development (Branching)

```bash
# Developer A (feature-a branch):
makemigrations makemigrations --name "feature_a"
# Created migrations/0003_feature_a.go (depends on 0002)
git add . && git commit

# Developer B (feature-b branch):
makemigrations makemigrations --name "feature_b"
# Created migrations/0003_feature_b.go (depends on 0002)
git add . && git commit

# After merge to main:
makemigrations makemigrations
# ⚠ Branches detected: 0003_feature_a, 0003_feature_b
# Created migrations/0004_merge_feature_a_and_b.go
# No additional schema changes detected.

# Deploy:
cd migrations && go build -o migrate .
./migrate up
# Applying 0003_feature_a... done
# Applying 0003_feature_b... done
# Applying 0004_merge_feature_a_and_b... done
```

### CI/CD Pipeline

```yaml
# .github/workflows/migrations.yml
- name: Check for unmigrated changes
  run: makemigrations makemigrations --check

- name: Build migration binary
  run: cd migrations && go build -o migrate .

- name: Show migration plan
  run: ./migrations/migrate showsql

- name: Apply migrations
  run: ./migrations/migrate up
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
```

### Inspecting the Graph

```bash
cd migrations && go build -o migrate .
./migrate dag

# Migration Graph
# ===============
#
#   0001_initial
#   │  Create table users (6 fields)
#   │  Create table posts (4 fields)
#   │
#   └─► 0002_add_phone
#      │  Add field users.phone varchar(20)
#      │  Add index idx_users_phone on users(phone)
#      │
#      ├─► 0003_feature_a
#      │    Create table categories (3 fields)
#      │
#      └─► 0003_feature_b
#           Add field posts.published_at timestamp
#      │
#      └─► 0004_merge ◄─┘
#           (merge)
#
# Roots:  0001_initial
# Leaves: 0004_merge
# ✓ No branches — graph is linear
```

## Design Decisions

### Why Go Code Generation Over Tengo Scripts

Tengo (a Go scripting language) was evaluated and rejected. Migration files are pure data declarations — they don't compute anything. The DAG building, topological sort, and state reconstruction all run in Go regardless. Tengo would add a VM dependency, require type marshalling between Tengo maps and Go structs, produce cryptic runtime errors, and provide no benefit since the scripting flexibility is never used. Go gives compile-time validation, IDE support, debugging with `dlv`, and zero runtime overhead.

### Why Compiled Binary Over Snapshot Cache

Previous designs considered snapshot caches (`.schema_state.yaml`) and comment header scanning. The compiled binary approach eliminates all secondary state:

| Problem | Solution |
|---|---|
| How to find migrations | `go build` compiles all `.go` files — it's just how Go works |
| How to read the graph | Build binary, run `dag --format json`, parse output |
| How to know current schema state | Included in DAG output via operation replay |
| Snapshot cache drift | No cache — binary is always authoritative |
| Comment headers drift | No comment headers needed |
| Detecting branches | `has_branches` + `leaves` in DAG output |

### Why Custom Runner Over Goose

Goose doesn't understand DAGs, merge migrations, or typed operations. Building a custom runner gives full control over execution order, state tracking, and the `makemigrations_history` table format. The runner itself is simple (~300-400 LOC) — the real complexity lives in the graph and operations, which are needed regardless.

## Success Metrics

- Full Django migration parity: DAG, merge migrations, squash, state reconstruction
- Zero external dependencies at deploy time (single compiled binary)
- Existing YAML schemas work without modification
- Existing provider layer works without modification
- Sub-second migration generation with warm build cache
- Migration files are valid, `gofmt`-compatible Go source
- Branch conflicts detected and resolved automatically via merge migrations

## Future Enhancements

- **Migration squashing** — collapse a range of migrations into a single equivalent
- **`RunGo` operation** — hand-written Go functions for complex data migrations (compiled into the binary)
- **Dry-run against live database** — compare DAG state against actual database state to detect drift
- **Interactive conflict resolution** — when merge migrations have schema conflicts (same table modified differently), offer resolution strategies
- **Migration testing** — `./migrate test` that applies all migrations to a temp database and verifies round-trip (up then down)
- **Partial index / expression index support** — extend the `Index` type
- **Composite primary key support** — extend the `Field` / `CreateTable` types
- **`ON UPDATE` foreign key support** — already present in the `ForeignKey` struct, needs provider implementation
