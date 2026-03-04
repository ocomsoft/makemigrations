# Type Mappings DAG Integration — Design

**Date:** 2026-03-05
**Status:** Approved
**Scope:** Go migration DAG integration for `type_mappings` + docs update

---

## Background

The `type_mappings` section already exists in `schema.yaml` and `internal/types/types.go`, and is
wired to SQL-mode generation (`dump_sql`, `yaml_common.go`). Every provider implements
`SetTypeMappings(map[string]string)`. However, the Go migration DAG has no knowledge of
`type_mappings` — `SchemaState` does not track them, there is no `SetTypeMappings` DAG operation,
and `schemaStateToYAMLSchema()` does not repopulate them. This means TypeMappings changes are
never detected as needing a new migration, and the DAG runner does not apply them when generating
SQL.

This design fills that gap using the same pattern as `Defaults`/`SetDefaults`.

---

## Approach: Mirror SetDefaults

All changes follow the existing `Defaults`/`SetDefaults` pattern exactly.

---

## Section 1: Data Model

### `migrate/state.go`

Add `TypeMappings` field to `SchemaState`:

```go
type SchemaState struct {
    Tables       map[string]*TableState `json:"tables"`
    Defaults     map[string]string      `json:"defaults,omitempty"`
    TypeMappings map[string]string      `json:"type_mappings,omitempty"` // active provider's type mappings
}
```

Add `SetTypeMappings` method:

```go
// SetTypeMappings updates the active type mappings on the state.
// Called by SetTypeMappings operations during migration traversal.
func (s *SchemaState) SetTypeMappings(m map[string]string) {
    s.TypeMappings = m
}
```

`TypeMappings` stores only the **active database provider's** flattened map (same trade-off as
`Defaults`). On PostgreSQL it holds `schema.TypeMappings.PostgreSQL`.

---

## Section 2: New DAG Operation

### `migrate/operations.go`

New `SetTypeMappings` struct (after `SetDefaults`):

```go
// SetTypeMappings is a migration operation that records the active schema type mappings
// for the target database provider. It emits no SQL — its only effect is updating
// SchemaState.TypeMappings so that subsequent operations use the correct type overrides.
type SetTypeMappings struct {
    TypeMappings map[string]string
}

func (op *SetTypeMappings) TypeName() string                                          { return "set_type_mappings" }
func (op *SetTypeMappings) TableName() string                                         { return "" }
func (op *SetTypeMappings) IsDestructive() bool                                       { return false }
func (op *SetTypeMappings) Describe() string                                          { return "Set schema type mappings" }
func (op *SetTypeMappings) Up(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
    return "", nil
}
func (op *SetTypeMappings) Down(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
    return "", nil
}
func (op *SetTypeMappings) Mutate(state *SchemaState) error {
    state.SetTypeMappings(op.TypeMappings)
    return nil
}
```

### `migrate/runner.go`

Sync provider TypeMappings from state **before each `op.Up()`/`op.Down()`** in
`applyMigration`, `rollbackMigration`, and `ShowSQL`:

```go
// Sync provider type mappings from current state before generating SQL
r.provider.SetTypeMappings(state.TypeMappings)
sqlStr, err := op.Up(r.provider, state, state.Defaults)
```

This handles the edge case where `SetTypeMappings` and DDL operations appear in the same
migration — subsequent operations see the updated mappings immediately.

---

## Section 3: Diff Detection

### `internal/yaml/diff.go`

New `ChangeType` constant:

```go
ChangeTypeTypeMappingsModified ChangeType = "type_mappings_modified" // non-destructive: updates active type mappings
```

### `cmd/go_migrations.go`

New helper (parallel to `getDefaultsForDB`):

```go
// getTypeMappingsForDB returns the type mappings for the given database type from a schema.
func getTypeMappingsForDB(schema *yamlpkg.Schema, dbType string) map[string]string {
    if schema == nil {
        return nil
    }
    return schema.TypeMappings.ForProvider(types.DatabaseType(dbType))
}
```

Diff detection (after the Defaults block):

```go
prevMappings := getTypeMappingsForDB(prevSchema, cfg.Database.Type)
currMappings := getTypeMappingsForDB(currentSchema, cfg.Database.Type)
if !mapsEqual(prevMappings, currMappings) && len(currMappings) > 0 {
    diff.Changes = append([]yamlpkg.Change{{
        Type:        yamlpkg.ChangeTypeTypeMappingsModified,
        Description: "Update schema type mappings",
        OldValue:    prevMappings,
        NewValue:    currMappings,
    }}, diff.Changes...)
    diff.HasChanges = true
}
```

### `cmd/go_init.go`

Prepend `SetTypeMappings` change when generating the initial migration if the schema has type
mappings for the active DB:

```go
// Prepend SetTypeMappings if the schema has type mappings for this DB type
currMappings := getTypeMappingsForDB(schema, string(dbType))
if len(currMappings) > 0 {
    changes = append([]yamlpkg.Change{{
        Type:        yamlpkg.ChangeTypeTypeMappingsModified,
        Description: "Set schema type mappings",
        NewValue:    currMappings,
    }}, changes...)
}
```

### `cmd/go_migrations.go` — `schemaStateToYAMLSchema()`

Repopulate `TypeMappings` from `state.TypeMappings` (parallel to Defaults):

```go
if len(state.TypeMappings) > 0 {
    switch dbType {
    case "postgresql":
        schema.TypeMappings.PostgreSQL = state.TypeMappings
    case "mysql":
        schema.TypeMappings.MySQL = state.TypeMappings
    case "sqlserver":
        schema.TypeMappings.SQLServer = state.TypeMappings
    case "sqlite":
        schema.TypeMappings.SQLite = state.TypeMappings
    // ... other providers
    }
}
```

---

## Section 4: Codegen

### `internal/codegen/go_generator.go`

Add case in the change dispatch:

```go
case yaml.ChangeTypeTypeMappingsModified:
    return g.generateSetTypeMappings(change)
```

New generator method (sorted keys for determinism, parallel to `generateSetDefaults`):

```go
func (g *GoGenerator) generateSetTypeMappings(change yaml.Change) (string, error) {
    mappings, ok := change.NewValue.(map[string]string)
    if !ok {
        return "", fmt.Errorf("SetTypeMappings: expected map[string]string NewValue")
    }
    keys := sortedKeys(mappings)
    var b strings.Builder
    b.WriteString("\t\t\t&m.SetTypeMappings{\n\t\t\t\tTypeMappings: map[string]string{\n")
    for _, k := range keys {
        fmt.Fprintf(&b, "\t\t\t\t\t%q: %q,\n", k, mappings[k])
    }
    b.WriteString("\t\t\t\t},\n\t\t\t},\n")
    return b.String(), nil
}
```

---

## Section 5: Documentation

### `docs/schema-format.md`

Add a **Type Mappings** section documenting:
- Purpose: override the default SQL type per database provider
- Structure: per-provider maps with YAML keys
- Go template syntax for parameterised types (`DECIMAL({{.Precision}},{{.Scale}})`)
- Examples: `float → DOUBLE PRECISION` (PostgreSQL), `text → NVARCHAR(MAX)` (SQL Server)
- Note that `type_mappings` is tracked in the migration DAG via `SetTypeMappings` operations

---

## Files Changed

| File | Change |
|------|--------|
| `migrate/state.go` | Add `TypeMappings` field + `SetTypeMappings()` method |
| `migrate/operations.go` | Add `SetTypeMappings` operation struct |
| `migrate/runner.go` | Sync provider TypeMappings before each op |
| `internal/yaml/diff.go` | Add `ChangeTypeTypeMappingsModified` constant |
| `cmd/go_migrations.go` | Add `getTypeMappingsForDB()`, diff detection, `schemaStateToYAMLSchema()` update |
| `cmd/go_init.go` | Prepend SetTypeMappings change for initial migration |
| `cmd/yaml_common.go` | Same diff detection for SQL-mode migrations (if applicable) |
| `internal/codegen/go_generator.go` | Add `generateSetTypeMappings()` + dispatch case |
| `docs/schema-format.md` | Add Type Mappings section |

## Tests Required

- `migrate/operations_test.go` — `TestSetTypeMappings_Mutate`, `TestSetTypeMappings_UpDown`
- `migrate/state_test.go` — `TestSchemaState_SetTypeMappings`
- `cmd/go_migrations_test.go` — `TestSchemaStateToYAMLSchema_TypeMappings`, diff detection tests
- `internal/codegen/go_generator_test.go` — `TestGoGenerator_SetTypeMappings`
