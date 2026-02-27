# Architecture Documentation

## Overview

Makemigrations is a YAML-first database migration tool for Go that generates typed, compiled Go migration files from declarative schema definitions. It follows a Django-inspired workflow while remaining idiomatic Go: migrations are real Go source files that register themselves via `init()`, are compiled into a standalone binary, and executed without any external migration runner.

The tool supports two distinct workflows:

- **Go migrations (primary)** — Generates `.go` migration files. The compiled binary is the single source of truth for migration state. No separate state file is required.
- **SQL migrations (legacy, opt-in via `--sql`)** — Generates Goose-compatible `.sql` files applied via `makemigrations goose up`.

---

## System Architecture

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        Developer Workflow                         │
│                                                                   │
│   schema/schema.yaml  ──►  makemigrations makemigrations         │
│                                    │                             │
│                         migrations/0001_initial.go               │
│                         migrations/0002_add_users.go  ...        │
│                                    │                             │
│                        go build -o migrations/migrate            │
│                                    │                             │
│                      ./migrations/migrate up | down | status     │
└──────────────────────────────────────────────────────────────────┘
                                    │
┌──────────────────────────────────────────────────────────────────┐
│                     makemigrations CLI (cmd/)                     │
│                                                                   │
│  ┌────────────────┐  ┌──────────────────┐  ┌─────────────────┐  │
│  │  makemigrations│  │  init / go-init  │  │  sql-migrations │  │
│  │  (Go generator)│  │  (bootstrapper)  │  │  (legacy SQL)   │  │
│  └────────────────┘  └──────────────────┘  └─────────────────┘  │
│                                                                   │
│  ┌────────────────┐  ┌──────────────────┐  ┌─────────────────┐  │
│  │  db2schema     │  │  struct2schema   │  │  schema2diagram │  │
│  │  (introspect)  │  │  (Go → YAML)     │  │  (visualise)    │  │
│  └────────────────┘  └──────────────────┘  └─────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
                                    │
┌──────────────────────────────────────────────────────────────────┐
│                     Core Processing Layer                         │
│                                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐            │
│  │ YAML Parser │  │ Diff Engine │  │  Go Codegen  │            │
│  │ (schema.yaml│  │ (SchemaDiff)│  │  (GoGenerator│            │
│  │  → Schema)  │  │             │  │   go/format) │            │
│  └─────────────┘  └─────────────┘  └──────────────┘            │
│                                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐            │
│  │   Registry  │  │    Graph    │  │ SchemaState  │            │
│  │  (init()    │  │  (DAG +     │  │  (in-memory  │            │
│  │   pattern)  │  │   Kahn's)   │  │   replay)    │            │
│  └─────────────┘  └─────────────┘  └──────────────┘            │
└──────────────────────────────────────────────────────────────────┘
                                    │
┌──────────────────────────────────────────────────────────────────┐
│                   Database Provider Layer (internal/providers/)   │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │PostgreSQL│  │  MySQL   │  │  SQLite  │  │SQLServer │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Redshift │  │ClickHouse│  │   TiDB   │  │ Vertica  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │   YDB    │  │  Turso   │  │StarRocks │  │AuroraDSQL│        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└──────────────────────────────────────────────────────────────────┘
                                    │
┌──────────────────────────────────────────────────────────────────┐
│                      Compiled Migrations Binary                   │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Runner  │  │ Recorder │  │   App    │  │  Cobra   │        │
│  │  Up/Down │  │ history  │  │ (CLI in  │  │ Commands │        │
│  │  ShowSQL │  │  table   │  │  binary) │  │          │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└──────────────────────────────────────────────────────────────────┘
```

---

## Primary Workflow: Go Migrations

### End-to-End Developer Flow

```
1. Developer edits schema/schema.yaml
         │
2. makemigrations makemigrations
   ├── Parses schema/schema.yaml  (internal/yaml)
   ├── Runs compiled binary: ./migrations/migrate dag --format json
   │        └── Binary emits DAGOutput JSON (graph + SchemaState)
   ├── Diffs YAML schema against reconstructed SchemaState
   │        └── internal/yaml: SchemaDiff
   └── Writes migrations/NNNN_<name>.go  (internal/codegen: GoGenerator)
         │
3. Developer compiles
   └── cd migrations && go mod tidy && go build -o migrate .
         │
4. Developer applies
   ├── ./migrations/migrate up          (apply pending)
   ├── ./migrations/migrate down        (rollback)
   ├── ./migrations/migrate status      (show applied/pending)
   ├── ./migrations/migrate showsql     (preview SQL without executing)
   └── ./migrations/migrate dag         (visualise DAG)
```

### Branch Detection and Merge Migrations

When two developers independently create migrations from the same parent, the graph has multiple leaf nodes. The `makemigrations makemigrations --merge` flag generates a merge migration:

```
0001_initial ──► 0002_add_users   (developer A)
             └─► 0002_add_posts   (developer B)
                     │
             makemigrations makemigrations --merge
                     │
             0003_merge.go  (Dependencies: ["0002_add_users", "0002_add_posts"], Operations: [])
```

The merge migration has two parents and empty operations. It exists solely to re-linearise the graph.

### Squash Migrations

Old migrations can be collapsed into a single squash migration using `SquashGenerator` (`internal/codegen/squash_generator.go`). The resulting migration carries a `Replaces` field listing the names of all migrations it supersedes. The Runner skips individual migrations when their names appear in a `Replaces` list.

---

## Legacy Workflow: SQL Migrations

The SQL workflow is opt-in and retained for compatibility. It uses YAML snapshots and generates Goose-compatible `.sql` files.

```
1. makemigrations init --sql
   └── Creates migrations/ with .schema_snapshot.yaml
         │
2. Developer edits schema/schema.yaml
         │
3. makemigrations sql-migrations
   ├── Diffs schema against .schema_snapshot.yaml
   └── Writes migrations/NNNN_<name>.sql (-- +goose Up / +goose Down)
         │
4. makemigrations goose up
   └── Delegates to the Goose migration runner
```

Key differences from the Go workflow:

| Concern              | Go migrations (primary)        | SQL migrations (legacy)            |
|----------------------|--------------------------------|------------------------------------|
| State storage        | Compiled binary (DAG replay)   | `.schema_snapshot.yaml` file       |
| Migration format     | `.go` (typed, compiled)        | `.sql` (Goose format)              |
| Execution            | Compiled binary: `migrate up`  | `makemigrations goose up`          |
| Branch detection     | Graph leaves, `--merge` flag   | Not supported                      |
| VCS merge conflicts  | None (binary is rebuilt)       | Possible (snapshot file)           |

---

## Core Components

### 1. Command Layer (`cmd/`)

Each command is in its own source file. The CLI is built with Cobra.

| File                   | Command                        | Purpose                                              |
|------------------------|--------------------------------|------------------------------------------------------|
| `root.go`              | `makemigrations`               | Root command, Viper config loading                   |
| `go_migrations.go`     | `makemigrations makemigrations`| Go migration generator (primary workflow)            |
| `go_init.go`           | `makemigrations init`          | Bootstrap `migrations/main.go` + `go.mod`            |
| `sql_migrations.go`    | `makemigrations sql-migrations`| Legacy SQL migration generator                       |
| `init_sql.go`          | `makemigrations init --sql`    | Legacy SQL project setup                             |
| `goose.go`             | `makemigrations goose`         | Goose runner integration (legacy)                    |
| `db2schema.go`         | `makemigrations db2schema`     | Reverse-engineer DB to YAML schema                   |
| `struct2schema.go`     | `makemigrations struct2schema` | Convert Go structs to YAML schema                    |
| `schema2diagram.go`    | `makemigrations schema2diagram`| Visualise schema as diagram                          |
| `dump_sql.go`          | `makemigrations dump-sql`      | Generate SQL without writing migration files         |
| `find_includes.go`     | `makemigrations find-includes` | Discover schema includes from Go modules             |

### 2. `migrate/` Package (Runtime Library)

This is the library imported by all generated migration files and by the compiled binary. It is self-contained and has no dependency on the `cmd/` or `internal/` packages.

#### 2.1 Type System (`migrate/types.go`)

```go
// Migration is a single migration node in the DAG.
type Migration struct {
    Name         string      // Unique identifier e.g. "0001_initial"
    Dependencies []string    // Parent migration names
    Operations   []Operation // Schema changes applied in order
    Replaces     []string    // For squash migrations: names of replaced migrations
}

// Field is a database column definition used in operations.
type Field struct {
    Name       string
    Type       string      // varchar, text, integer, uuid, boolean, timestamp, ...
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

type ManyToMany struct {
    Table string
}

type Index struct {
    Name   string
    Fields []string
    Unique bool
}
```

#### 2.2 Operation Interface and Concrete Types (`migrate/operations.go`)

```go
type Operation interface {
    TypeName() string           // e.g. "CreateTable"
    TableName() string          // primary table affected
    Describe() string           // human-readable description
    ForwardSQL(provider) string // SQL to apply
    ReverseSQL(provider) string // SQL to rollback
    Mutate(*SchemaState) error  // mutates in-memory state
}
```

The 10 concrete operation types are:

| Type            | Description                              |
|-----------------|------------------------------------------|
| `CreateTable`   | CREATE TABLE with fields and indexes     |
| `DropTable`     | DROP TABLE                               |
| `RenameTable`   | ALTER TABLE ... RENAME TO ...            |
| `AddField`      | ALTER TABLE ... ADD COLUMN ...           |
| `DropField`     | ALTER TABLE ... DROP COLUMN ...          |
| `AlterField`    | ALTER TABLE ... ALTER COLUMN ...         |
| `RenameField`   | ALTER TABLE ... RENAME COLUMN ...        |
| `AddIndex`      | CREATE [UNIQUE] INDEX ...                |
| `DropIndex`     | DROP INDEX ...                           |
| `RunSQL`        | Arbitrary SQL (forward + reverse pair)   |

#### 2.3 Registry (`migrate/registry.go`)

The Registry is the core of the `init()` registration pattern. Each generated `.go` file calls `m.Register()` from its `init()` function. The global registry is populated before `main()` runs, ensuring all migrations are available at startup.

```go
// Global registry populated by all init() calls.
var globalRegistry = NewRegistry()

// Register adds a migration. Panics on nil or duplicate name.
func Register(m *Migration) { globalRegistry.Register(m) }

// GlobalRegistry is used by App to build the Graph.
func GlobalRegistry() *Registry { return globalRegistry }
```

The `Registry` struct holds a `map[string]*Migration` plus an insertion-order slice. It is protected by a `sync.RWMutex` for safe concurrent use during binary startup.

#### 2.4 Graph and DAG (`migrate/graph.go`)

`Graph` is a directed acyclic graph where each node is a `Migration` and each directed edge represents a dependency.

```
BuildGraph(registry)
    └── wires parent/child pointers from Migration.Dependencies
    └── calls detectCycles() (DFS white/grey/black colouring)

Graph.Linearize()
    └── Kahn's algorithm with alphabetical tie-breaking for determinism
    └── Returns []*Migration in topological order

Graph.ReconstructState()
    └── Calls Linearize(), then replays all Operation.Mutate() calls
    └── Returns *SchemaState representing the full current schema

Graph.ToDAGOutput()
    └── Produces DAGOutput (JSON-serialisable) including SchemaState
    └── Emitted by `./migrations/migrate dag --format json`

Graph.DetectBranches()
    └── Returns leaf groups when multiple leaves exist (concurrent branches)
```

The `DAGOutput` JSON is the mechanism by which `makemigrations makemigrations` reads the current compiled schema state without parsing Go source files.

#### 2.5 SchemaState (`migrate/state.go`)

`SchemaState` is the in-memory representation of the database schema at any point in the graph. It eliminates the need for a separate snapshot file in the Go workflow.

```go
type SchemaState struct {
    Tables map[string]*TableState
}

type TableState struct {
    Name    string
    Fields  []Field
    Indexes []Index
}
```

Mutation methods: `AddTable`, `DropTable`, `RenameTable`, `AddField`, `DropField`, `AlterField`, `RenameField`, `AddIndex`, `DropIndex`. Each returns an error if the precondition is violated (e.g. adding a field to a non-existent table).

#### 2.6 Runner (`migrate/runner.go`)

`Runner` executes migrations against a live database. It receives a `*Graph`, a `providers.Provider`, a `*sql.DB`, and a `*MigrationRecorder`.

- `Up(to string)` — Linearises the graph, skips applied migrations (queried from `makemigrations_history`), applies each pending migration in a transaction, records it.
- `Down(steps int, to string)` — Rolls back in reverse topological order.
- `Status()` — Prints applied/pending status for each migration.
- `ShowSQL()` — Prints SQL for pending migrations without executing.

#### 2.7 MigrationRecorder (`migrate/recorder.go`)

Manages the `makemigrations_history` table in the target database:

```sql
CREATE TABLE IF NOT EXISTS makemigrations_history (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    applied_at TEXT DEFAULT CURRENT_TIMESTAMP
)
```

Methods: `EnsureTable`, `GetApplied`, `RecordApplied`, `RecordRolledBack`, `Fake`.

#### 2.8 App (`migrate/app.go`)

`App` is the embedded Cobra CLI inside the compiled migration binary. It is constructed in each generated `migrations/main.go` and wires together Registry → Graph → Runner.

Commands exposed by the compiled binary:

| Subcommand       | Description                                          |
|------------------|------------------------------------------------------|
| `up [--to NAME]` | Apply pending migrations                             |
| `down [--steps N] [--to NAME]` | Rollback migrations                   |
| `status`         | Show applied/pending status                          |
| `showsql`        | Print SQL for pending migrations                     |
| `fake NAME`      | Mark a migration applied without executing SQL       |
| `dag [--format ascii\|json]` | Visualise or export the migration graph  |

### 3. Code Generation (`internal/codegen/`)

#### 3.1 GoGenerator (`internal/codegen/go_generator.go`)

Converts a `yaml.SchemaDiff` into a compilable `.go` source file. Uses `go/format` to ensure the output is always correctly formatted Go.

```go
type GoGenerator struct{}

// GenerateMigration produces a .go file for a single migration.
// The file is in package main, imports migrate aliased as "m", and
// calls m.Register() from its init() function.
func (g *GoGenerator) GenerateMigration(
    name string,
    dependencies []string,
    diff *yaml.SchemaDiff,
    currentSchema *yaml.Schema,
    previousSchema *yaml.Schema,
) ([]byte, error)

// GenerateMainGo produces migrations/main.go (the binary entry point).
func (g *GoGenerator) GenerateMainGo(databaseType string) ([]byte, error)

// GenerateGoMod produces migrations/go.mod.
func (g *GoGenerator) GenerateGoMod(moduleName string) ([]byte, error)
```

Example of a generated migration file:

```go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0001_initial",
        Dependencies: []string{},
        Operations: []m.Operation{
            m.CreateTable{
                TableName_: "users",
                Fields: []m.Field{
                    {Name: "id", Type: "integer", PrimaryKey: true},
                    {Name: "email", Type: "varchar", Length: 255},
                },
                Indexes: []m.Index{},
            },
        },
    })
}
```

#### 3.2 MergeGenerator (`internal/codegen/merge_generator.go`)

Generates a merge migration when `--merge` is passed. The generated file has two or more entries in `Dependencies` and an empty `Operations` slice.

#### 3.3 SquashGenerator (`internal/codegen/squash_generator.go`)

Collapses a range of migrations into a single squash migration. The result populates `Replaces` with the names of all squashed migrations. The Runner treats any migration whose name appears in `Replaces` as superseded.

### 4. YAML Processing (`internal/yaml/`)

Responsible for reading `schema.yaml` files and computing diffs.

| File                      | Responsibility                                          |
|---------------------------|---------------------------------------------------------|
| `parser.go`               | Unmarshal YAML into `Schema` structs                    |
| `diff.go`                 | Compare two `Schema` values → `SchemaDiff`              |
| `types.go`                | Re-exports `internal/types` for YAML package consumers  |
| `state.go`                | `StateManager` — loads/saves `.schema_snapshot.yaml`    |
| `merger.go`               | Merges multiple schema sources                          |
| `include_processor.go`    | Resolves `include:` directives from Go modules          |
| `module_resolver.go`      | Locates Go module roots for include resolution          |
| `migration_generator.go`  | Legacy SQL generation from `SchemaDiff`                 |
| `header.go`               | Chain metadata read/write (legacy chain workflow)       |
| `chain.go`                | Chain traversal and fork detection (legacy)             |

The `SchemaDiff` type describes the delta between two schemas:

```go
type SchemaDiff struct {
    AddedTables    []Table
    RemovedTables  []Table
    ModifiedTables []TableDiff
}

type TableDiff struct {
    TableName    string
    AddedFields  []Field
    RemovedFields []Field
    AlteredFields []FieldDiff
    AddedIndexes  []Index
    RemovedIndexes []Index
}
```

### 5. Configuration System (`internal/config/`)

Configuration is loaded via Viper with the following priority (highest to lowest):

1. Command-line flags
2. Environment variables (`MAKEMIGRATIONS_*`)
3. Config file (`migrations/makemigrations.config.yaml`)
4. Default values

Key config fields:

```go
type MigrationConfig struct {
    Directory           string // default: "migrations"
    DatabaseType        string // postgresql, mysql, sqlite, etc.
    EnableChainMetadata bool   // enables chain metadata in SQL headers (legacy)
}
```

### 6. Database Provider System (`internal/providers/`)

All 12 providers implement a common interface used by the Runner to generate database-specific DDL at migration apply time.

```go
type Provider interface {
    GenerateCreateTable(table *TableState) (string, error)
    GenerateDropTable(tableName string) string
    GenerateAddColumn(tableName string, field *Field) string
    GenerateDropColumn(tableName string, fieldName string) string
    GenerateAlterColumn(tableName string, oldField, newField *Field) (string, error)
    GenerateRenameTable(oldName, newName string) string
    GenerateRenameColumn(tableName, oldName, newName string) string
    GenerateCreateIndex(tableName string, index *Index) string
    GenerateDropIndex(indexName string) string
    QuoteName(name string) string
    ConvertFieldType(field *Field) string
}
```

Provider factory:

```go
func NewProvider(dbType string) (Provider, error)
```

Supported databases: PostgreSQL, MySQL, SQLite, SQL Server, Redshift, ClickHouse, TiDB, Vertica, YDB, Turso, StarRocks, AuroraDSQL.

### 7. Type System (`internal/types/`)

Central schema type definitions shared across `internal/yaml/`, `internal/codegen/`, and `cmd/`:

```go
type Schema struct {
    Database DatabaseConfig
    Include  []Include   // External schema imports from Go modules
    Defaults Defaults    // Database-specific field defaults
    Tables   []Table
}

type Table struct {
    Name    string
    Fields  []Field
    Indexes []Index
}

type Field struct {
    Name       string
    Type       string
    PrimaryKey bool
    Nullable   *bool
    Default    string
    ForeignKey *ForeignKey
    // ... additional column properties
}
```

`DatabaseType` is a string type alias, not an enum, allowing custom database names.

### 8. Specialised Utilities

#### Struct2Schema (`internal/struct2schema/`)

AST-based conversion of Go structs to YAML schema:

- Parses Go source files using `go/ast`
- Interprets struct tags: `db`, `gorm`, `sql`, `bun`
- Maps Go types to schema field types
- Detects foreign key relationships via tag conventions

#### DB2Schema (`cmd/db2schema.go`)

Reverse-engineers a live database to a YAML schema file. Supports all 12 providers. Useful for bootstrapping schema files from an existing database.

---

## Design Patterns

### 1. Init() Registration Pattern

The fundamental pattern for Go migrations. Each generated `.go` file in the `migrations/` directory contains exactly one `init()` function that calls `m.Register()`. Because Go runs all `init()` functions before `main()`, the global registry is fully populated by the time the binary executes any command.

This pattern requires no reflection, no file system scanning at runtime, and no external configuration. The compiled binary is self-contained.

### 2. DAG as Single Source of Truth

The `Graph` built from the `Registry` is the authoritative record of migration history and schema state. There is no separate snapshot file in the Go workflow. The `makemigrations` tool queries the binary via `dag --format json` to read the current state before generating a new migration.

### 3. State Reconstruction by Replay

`SchemaState` is rebuilt by replaying all operations in topological order (`Graph.ReconstructState()`). This is the same approach used by the Runner during `up` to avoid re-running already-applied migrations, and by the `makemigrations` generator to determine what the current schema looks like.

### 4. Provider Strategy Pattern

Each database provider implements the same `Provider` interface. The Runner receives a provider at construction time and delegates all SQL generation to it. This isolates database-specific logic and allows mock providers in tests.

### 5. Command Pattern

Each CLI command is its own source file in `cmd/`. Each command is a Cobra `*cobra.Command` registered on the root. Business logic is implemented in `internal/` packages; commands are thin wrappers.

### 6. Factory Pattern

`NewProvider(dbType string)` and `NewApp(cfg Config)` are the primary factory functions. They centralise construction decisions and prevent the rest of the codebase from importing provider implementations directly.

---

## Data Flow

### Go Migration Generation Flow

```
1. Developer edits schema/schema.yaml
         │
2. cmd/go_migrations.go
   │
   ├── internal/yaml: Parse schema/schema.yaml → Schema
   │
   ├── os/exec: Run ./migrations/migrate dag --format json
   │       │
   │       └── migrate/graph.go: ToDAGOutput() → JSON
   │               └── Includes SchemaState (fully reconstructed)
   │
   ├── internal/yaml: Convert SchemaState → Schema (for diffing)
   │
   ├── internal/yaml/diff.go: Diff(previousSchema, currentSchema) → SchemaDiff
   │
   ├── internal/codegen/go_generator.go: GenerateMigration(name, deps, diff) → []byte
   │       └── go/format: format source code
   │
   └── Write migrations/NNNN_<name>.go
```

### Migration Apply Flow (compiled binary)

```
1. Developer runs ./migrations/migrate up
         │
2. migrate/app.go: buildRunner()
   │
   ├── migrate/graph.go: BuildGraph(GlobalRegistry())
   │       └── Validates all dependencies exist, no cycles
   │
   ├── migrate/recorder.go: GetApplied() → map[string]bool
   │
   └── migrate/runner.go: Up("")
           │
           ├── graph.Linearize() → []*Migration (topological order)
           │
           ├── Replay already-applied migrations → SchemaState
           │
           └── For each pending migration:
                   ├── op.ForwardSQL(provider) → SQL string
                   ├── db.Exec(SQL)
                   ├── op.Mutate(state) → update in-memory state
                   └── recorder.RecordApplied(name)
```

### Struct2Schema Flow

```
1. cmd/struct2schema.go
         │
2. internal/struct2schema: Parse Go source files via go/ast
         │
3. Extract struct definitions + tags
         │
4. Map Go types → schema field types
         │
5. Detect relationships (foreign keys via tags)
         │
6. internal/yaml: Write schema.yaml
```

---

## Error Handling Strategy

### Validation Layers

1. **YAML Syntax** — Caught by `gopkg.in/yaml.v3` during schema parsing; reported with file and line context.
2. **Schema Semantics** — Type checking, required fields, constraint validation in `internal/yaml`.
3. **Graph Integrity** — Missing dependencies and cycles detected in `migrate/graph.go` before any SQL is generated or executed.
4. **Operation Preconditions** — `SchemaState` mutation methods return errors for violated preconditions (duplicate table, missing field, etc.).
5. **Runtime** — Database connectivity, permission errors, and SQL execution failures are wrapped and returned from `Runner`.

### Error Categories

- **Fatal** — Invalid config, unparseable YAML, graph cycle. Execution stops immediately.
- **Graph errors** — Missing dependency, duplicate migration name. The binary panics on duplicate registration (caught at startup).
- **Validation errors** — Collected and reported in full before stopping.
- **Warnings** — Destructive operations (DropTable, DropField) are logged prominently and flagged in generated code.

---

## Security Considerations

1. **No direct SQL execution by the generator** — `makemigrations makemigrations` only writes `.go` files. SQL is only executed when the developer explicitly runs the compiled binary.
2. **Input validation** — Strict YAML schema validation before any processing.
3. **SQL injection prevention** — Identifier quoting via `provider.QuoteName()` throughout all DDL generation.
4. **No credential storage** — Database credentials are passed via environment variables or DSN at runtime by the developer; never stored in generated files or config committed to VCS.
5. **Destructive operations are explicit** — `DropTable`, `DropField`, and `AlterField` (type changes) are always visible in the generated `.go` file and reviewed before the binary is compiled.

---

## Extension Points

### Adding a New Database Provider

1. Implement `internal/providers.Provider`.
2. Add a case to the `NewProvider` factory function.
3. Add type mapping constants.
4. Write provider-specific DDL tests.
5. Document any limitations (e.g. unsupported DDL operations) as comments.

### Adding a New Operation Type

1. Add the struct to `migrate/operations.go` implementing `Operation`.
2. Add a `Mutate(*SchemaState)` implementation.
3. Implement `ForwardSQL` and `ReverseSQL` for all providers.
4. Handle the new type in `internal/codegen/go_generator.go` (code emission).
5. Add a case to `internal/yaml/diff.go` (diff detection).

### Adding a New CLI Command

1. Create a new file in `cmd/` (one command per file).
2. Register the command on the root in `cmd/root.go`.
3. Implement business logic in the relevant `internal/` package.
4. Add documentation in `docs/commands/`.

---

## Testing Strategy

### Test Levels

1. **Unit tests** — Component-level; each package has `_test.go` files. Registry, Graph, SchemaState, and all Operation types have table-driven unit tests.
2. **Integration tests** — `integration_test.go` at the project root and `yaml_integration_test.go` exercise full pipelines: parse schema → diff → generate → compile → run.
3. **Provider tests** — Each provider is tested for correct DDL output for all 10 operation types.
4. **End-to-end tests** — Full command execution via `go test ./...` invoking CLI commands and verifying file output.

### Key Test Files

| File                                            | Coverage area                                  |
|-------------------------------------------------|------------------------------------------------|
| `migrate/registry_test.go`                      | Registration, panic on duplicate/nil           |
| `migrate/graph_test.go`                         | Topological sort, cycle detection, branches    |
| `migrate/state_test.go`                         | SchemaState mutation correctness               |
| `migrate/operations_test.go`                    | All 10 operation types, forward/reverse SQL    |
| `migrate/runner_test.go`                        | Up/Down/Status/ShowSQL                         |
| `internal/codegen/go_generator_test.go`         | Generated Go source correctness                |
| `internal/codegen/merge_generator_test.go`      | Merge migration generation                     |
| `internal/codegen/squash_generator_test.go`     | Squash migration generation                    |
| `cmd/go_migrations_test.go`                     | End-to-end generation command                  |
| `integration_test.go`                           | Full parse → generate → compile pipeline       |

---

## Key Architectural Decisions

### 1. Compiled Binary as Source of Truth

**Decision:** The migration binary is rebuilt from `.go` source files and queried via `dag --format json` to determine current schema state.

**Rationale:** Eliminates the `.schema_snapshot.yaml` file that caused merge conflicts in parallel development. The binary is always rebuilt from committed source, so the state is reproducible and VCS-friendly. The `DAGOutput` JSON is a stable, machine-readable interface between the generator and the binary.

### 2. Init() Registration Over File Scanning

**Decision:** Migrations self-register via `init()` into a global registry rather than being discovered by scanning the file system.

**Rationale:** Go-native; no reflection, no file system access, no special naming conventions beyond what the generator enforces. Compilation errors are caught at build time. The registry is populated before `main()` runs, so startup is deterministic.

### 3. Typed Operations Over Raw SQL

**Decision:** Migrations express changes as typed `Operation` structs rather than raw SQL strings.

**Rationale:** Typed operations allow `SchemaState` reconstruction by replaying `Mutate()` calls — no database connection required to determine current schema. Operations are also provider-agnostic: the same migration file works against any supported database.

### 4. Kahn's Algorithm with Alphabetical Tie-Breaking

**Decision:** Topological sort uses Kahn's algorithm; nodes at the same level are sorted alphabetically.

**Rationale:** Deterministic ordering is essential: two developers running `migrate up` on the same graph must apply migrations in the same order. Alphabetical tie-breaking is predictable and requires no additional metadata.

### 5. Database Provider Abstraction

**Decision:** All DDL generation is delegated to a `Provider` interface with 12 implementations.

**Rationale:** Isolates database-specific SQL from operation logic. Adding a new database requires implementing one interface, not modifying operation types. Enables mock providers in tests.

### 6. YAML as Schema Definition Language

**Decision:** Schema is declared in `schema.yaml`, not inferred from Go structs or a live database.

**Rationale:** YAML is human-readable, version-control friendly, and supports modular composition via `include:` directives. It is the single authoritative source of intent; the migration files are derived from it, not the reverse.

---

## Module Structure

```
github.com/ocomsoft/makemigrations
│
├── main.go                        Entry point for the makemigrations CLI
├── cmd/                           One file per CLI command
│   ├── root.go
│   ├── go_migrations.go           makemigrations makemigrations (primary)
│   ├── go_init.go                 makemigrations init
│   ├── sql_migrations.go          makemigrations sql-migrations (legacy)
│   ├── init_sql.go                makemigrations init --sql (legacy)
│   ├── goose.go                   makemigrations goose (legacy)
│   ├── db2schema.go
│   ├── struct2schema.go
│   ├── schema2diagram.go
│   ├── dump_sql.go
│   └── find_includes.go
│
├── migrate/                       Runtime library (imported by generated files)
│   ├── types.go                   Migration, Field, ForeignKey, ManyToMany, Index
│   ├── operations.go              Operation interface + 10 concrete types
│   ├── registry.go                Registry + global Register() + GlobalRegistry()
│   ├── graph.go                   Graph (DAG), BuildGraph, Linearize, ReconstructState
│   ├── state.go                   SchemaState, TableState, mutation methods
│   ├── runner.go                  Runner: Up, Down, Status, ShowSQL
│   ├── recorder.go                MigrationRecorder (makemigrations_history table)
│   ├── app.go                     App (Cobra CLI embedded in migration binary)
│   ├── config.go                  Config for App (DSN, database type)
│   ├── provider_bridge.go         Wires providers.Provider into Runner
│   └── dag_ascii.go               ASCII DAG renderer
│
├── internal/
│   ├── codegen/                   Go source code generation
│   │   ├── go_generator.go        GoGenerator: migration .go files, main.go, go.mod
│   │   ├── merge_generator.go     MergeGenerator: merge migrations
│   │   └── squash_generator.go    SquashGenerator: squash migrations
│   ├── yaml/                      YAML schema processing
│   │   ├── types.go               Re-exports internal/types
│   │   ├── parser.go              YAML → Schema
│   │   ├── diff.go                Schema → SchemaDiff
│   │   ├── state.go               StateManager (.schema_snapshot.yaml, legacy)
│   │   ├── merger.go              Multi-source schema merge
│   │   ├── include_processor.go   Include directive resolution
│   │   ├── module_resolver.go     Go module root discovery
│   │   ├── migration_generator.go Legacy SQL generation
│   │   ├── header.go              Chain metadata (legacy SQL workflow)
│   │   └── chain.go               Chain traversal and fork detection (legacy)
│   ├── config/                    Viper-based config loading
│   ├── types/                     Canonical schema types (Schema, Table, Field, Index)
│   ├── providers/                 12 database provider implementations
│   ├── scanner/                   Schema file discovery
│   ├── struct2schema/             Go AST → YAML schema
│   ├── diff/                      Diff utilities
│   ├── merger/                    Schema merging utilities
│   ├── parser/                    Schema parsing utilities
│   ├── analyzer/                  Schema semantic validation
│   └── writer/                    File writing utilities
│
└── migrations/                    Generated per-project (not committed to this repo)
    ├── go.mod                     Standalone module importing migrate/
    ├── main.go                    Binary entry point: NewApp(cfg).Run(os.Args[1:])
    ├── 0001_initial.go            Generated migration file
    ├── 0002_add_users.go          Generated migration file
    └── ...
```
