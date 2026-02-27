# Design: Django-Style Go Migration Framework

**Date:** 2026-02-27
**Status:** Approved
**PRD:** `Go_migrations_PRD.md`

## Summary

Replace the current SQL-file generation pipeline with a Django-style migration framework that generates compiled Go migration files with a dependency DAG, typed operations, merge migrations, and state reconstruction. The compiled migration binary becomes the single source of truth.

## Module Strategy

Single Go module (`github.com/ocomsoft/makemigrations`). The `migrate/` directory is a package, not a separate module. Generated user project `migrations/go.mod` files import `github.com/ocomsoft/makemigrations/migrate` at whatever version of the CLI tool they are using. Decoupling into a separate module is deferred.

## Package Structure

```
github.com/ocomsoft/makemigrations
├── cmd/
│   ├── sql_migrations.go        → RENAMED from makemigrations.go (existing SQL workflow, unchanged)
│   ├── go_migrations.go         → NEW: "makemigrations" command (Go workflow)
│   ├── init.go                  → EXTENDED: detects .schema_snapshot.yaml, generates initial .go migration
│   └── ...existing files unchanged...
├── internal/
│   ├── codegen/
│   │   ├── go_generator.go      → generates .go migration files from SchemaDiff
│   │   ├── merge_generator.go   → generates merge migration .go files
│   │   └── squash_generator.go  → generates squash migration .go files
│   └── ...existing packages unchanged...
└── migrate/
    ├── app.go                   → CLI app (up/down/status/showsql/dag/fake)
    ├── types.go                 → Migration, Field, Index, ForeignKey
    ├── operations.go            → Operation interface + all concrete types
    ├── registry.go              → Global registry, Register()
    ├── graph.go                 → DAG, Kahn's sort, branch detection, ReconstructState
    ├── state.go                 → In-memory SchemaState
    ├── runner.go                → Executes migrations against DB
    ├── recorder.go              → makemigrations_history table
    └── dag_ascii.go             → ASCII tree renderer
```

## Implementation Order (Outside-In)

### Step 1 — `migrate/` types + operations + registry (PRD Phase 1)
Define `Migration`, `Field`, `Index`, `Operation` interface, all 10 concrete operation types (`CreateTable`, `DropTable`, `RenameTable`, `AddField`, `DropField`, `AlterField`, `RenameField`, `AddIndex`, `DropIndex`, `RunSQL`), global `Register()` registry. Each operation's `Forward()`/`Backward()` delegates to the existing `Provider` interface. `Mutate()` updates in-memory `SchemaState`.

**Deliverables:** `migrate/types.go`, `migrate/operations.go`, `migrate/registry.go`, `migrate/state.go`

### Step 2 — Go code generator (PRD Phase 2)
`internal/codegen/go_generator.go` converts a `SchemaDiff` + leaf names → valid `gofmt`-compatible `.go` migration source. Also generates `main.go` and `go.mod` for `init`.

**Deliverables:** `internal/codegen/go_generator.go`

### Step 3 — Migration graph (PRD Phase 3)
DAG construction from registry, Kahn's topological sort, cycle detection, root/leaf identification, `ReconstructState()`, `ToDAGOutput()`.

**Deliverables:** `migrate/graph.go`

### Step 4 — DAG command + binary query loop (PRD Phase 5)
`migrate/app.go` wires the `dag` subcommand with `--format json` and ASCII output. `cmd/go_migrations.go` builds the migrations binary, runs `dag --format json`, parses output, diffs against YAML schema, generates the next `.go` file.

**Deliverables:** `migrate/app.go`, `migrate/dag_ascii.go`, `cmd/go_migrations.go`

### Step 5 — Branch detection + merge migrations (PRD Phase 4)
Detect multiple leaves → auto-generate merge `.go` file. Wired into `makemigrations makemigrations` flow.

**Deliverables:** Extensions to `migrate/graph.go`, `internal/codegen/merge_generator.go`

### Step 6 — Runner + recorder (PRD Phase 6)
`migrate/runner.go` executes `up`/`down`/`status`/`showsql`. `migrate/recorder.go` manages `makemigrations_history` table. `fake` subcommand inserts a row without executing SQL.

**Deliverables:** `migrate/runner.go`, `migrate/recorder.go`

### Step 7 — Squash migrations (PRD Phase 7)
`internal/codegen/squash_generator.go` collapses a migration range into a single replacement with a `Replaces` field.

**Deliverables:** `internal/codegen/squash_generator.go`, extensions to `migrate/graph.go`

### Step 8 — Backward compatibility
- Rename `cmd/makemigrations.go` → `cmd/sql_migrations.go`, command `makemigrations` → `sql-migrations`
- Extend `makemigrations init`: detect `.schema_snapshot.yaml` → generate `0001_initial.go` from it
- Print `fake` instructions so users can mark the initial migration applied without re-running SQL

## Backward Compatibility: Initial Migration Path

When `migrations/.schema_snapshot.yaml` exists during `makemigrations init`:

1. Parse snapshot as current schema state
2. Generate `migrations/0001_initial.go` with `CreateTable` operations for every table
3. Generate `migrations/main.go` and `migrations/go.mod`
4. Print:

```
Created migrations/0001_initial.go (from existing schema snapshot)
Created migrations/main.go
Created migrations/go.mod

Your database already has these tables applied. Mark this migration as applied:

  cd migrations && go build -o migrate .
  ./migrate fake 0001_initial
```

The `fake` subcommand inserts into `makemigrations_history` without executing SQL.

## Testing Strategy

### Unit tests
- `migrate/` — registry duplicate detection, graph topological sort, cycle detection, branch detection, `ReconstructState()` correctness, `SchemaState` mutations per operation type
- `internal/codegen/` — generator output matches expected `.go` source per operation type, valid `gofmt` output, merge generation, squash generation

### Integration tests
- Full round-trip: YAML schema → `SchemaDiff` → `go_generator` → `.go` source → registry → `ReconstructState()` matches original schema
- Binary build + `dag --format json` round-trip against a real generated migrations directory
- `fake` command inserts correct row without executing SQL

### End-to-end tests
- `runner.go` `up`/`down` against SQLite using a fixture migrations directory
- Verify `makemigrations_history` populated correctly
- Verify `down` reverses schema changes

### Lint
All new code passes `golangci-lint run --no-config ./...` before each step is marked complete.

## Unchanged Components

| Component | Reason |
|---|---|
| `internal/yaml/parser.go` | Input format unchanged |
| `internal/yaml/diff.go` | Change detection unchanged |
| `internal/types/` | Data model unchanged |
| `internal/providers/` | Operations delegate to them at runtime |
| `internal/config/` | Minor additions only |
| `internal/scanner/` | Module schema discovery unchanged |

## Replaced Components

| Component | Replaced By |
|---|---|
| SQL converter (`internal/yaml/sql_converter.go`) | `internal/codegen/go_generator.go` + operations model |
| State manager (`internal/yaml/state.go`) | Graph-based state reconstruction |
| Goose integration (`cmd/goose.go`) | `migrate/app.go` compiled into binary |
| `cmd/makemigrations.go` | `cmd/sql_migrations.go` (renamed, unchanged) + `cmd/go_migrations.go` (new) |
