# Type Mappings DAG Integration — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire the existing `type_mappings` schema.yaml section into the Go migration DAG so changes are detected, a `SetTypeMappings` operation is generated, and the provider uses the correct type overrides at runtime.

**Architecture:** Mirrors the `Defaults`/`SetDefaults` pattern exactly. `SchemaState` gains a `TypeMappings` field updated by a new `SetTypeMappings` DAG operation. The diff engine detects changes and codegen emits the operation. `schemaStateToYAMLSchema()` repopulates `TypeMappings` for subsequent diffs. The runner syncs the provider before each `op.Up()`/`op.Down()`.

**Tech Stack:** Go, Cobra, `internal/providers` (Provider interface), `migrate` package, `internal/codegen`, `internal/yaml`

**Design doc:** `docs/plans/2026-03-05-type-mappings-dag-design.md`

---

### Task 1: Add `TypeMappings` to `SchemaState`

**Files:**
- Modify: `migrate/state.go`
- Test: `migrate/state_test.go`

**Step 1: Write the failing test**

Open `migrate/state_test.go`. Add after the existing `SetDefaults` tests:

```go
// TestSchemaState_SetTypeMappings verifies that SetTypeMappings updates state.TypeMappings.
func TestSchemaState_SetTypeMappings(t *testing.T) {
	state := migrate.NewSchemaState()
	if state.TypeMappings != nil {
		t.Errorf("expected nil TypeMappings, got %v", state.TypeMappings)
	}
	m := map[string]string{"float": "DOUBLE PRECISION"}
	state.SetTypeMappings(m)
	if state.TypeMappings["float"] != "DOUBLE PRECISION" {
		t.Errorf("expected DOUBLE PRECISION, got %q", state.TypeMappings["float"])
	}
	// Overwrite
	state.SetTypeMappings(map[string]string{"text": "NVARCHAR(MAX)"})
	if _, ok := state.TypeMappings["float"]; ok {
		t.Error("expected float key to be gone after overwrite")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./migrate/ -run TestSchemaState_SetTypeMappings -v
```

Expected: `FAIL` — `state.TypeMappings undefined` or field not found.

**Step 3: Implement**

In `migrate/state.go`, add to `SchemaState`:

```go
type SchemaState struct {
	Tables       map[string]*TableState `json:"tables"`
	Defaults     map[string]string      `json:"defaults,omitempty"`
	TypeMappings map[string]string      `json:"type_mappings,omitempty"` // active provider's type mappings from SetTypeMappings operations
}
```

Add method after `SetDefaults`:

```go
// SetTypeMappings updates the active type mappings on the state.
// Called by SetTypeMappings operations during migration traversal.
func (s *SchemaState) SetTypeMappings(m map[string]string) {
	s.TypeMappings = m
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./migrate/ -run TestSchemaState_SetTypeMappings -v
```

Expected: `PASS`

**Step 5: Run full migrate package tests**

```bash
go test ./migrate/... -v 2>&1 | tail -20
```

Expected: all pass.

**Step 6: Commit**

```bash
git add migrate/state.go migrate/state_test.go
git commit -m "feat: add TypeMappings field and SetTypeMappings method to SchemaState"
```

---

### Task 2: Add `SetTypeMappings` DAG Operation

**Files:**
- Modify: `migrate/operations.go`
- Test: `migrate/operations_test.go`

**Step 1: Write the failing tests**

Open `migrate/operations_test.go`. Add after the `TestSetDefaults_*` tests:

```go
// TestSetTypeMappings_Mutate verifies that SetTypeMappings.Mutate updates state.TypeMappings.
func TestSetTypeMappings_Mutate(t *testing.T) {
	state := migrate.NewSchemaState()
	op := &migrate.SetTypeMappings{
		TypeMappings: map[string]string{"float": "DOUBLE PRECISION", "text": "NVARCHAR(MAX)"},
	}
	if err := op.Mutate(state); err != nil {
		t.Fatalf("Mutate returned error: %v", err)
	}
	if state.TypeMappings["float"] != "DOUBLE PRECISION" {
		t.Errorf("expected DOUBLE PRECISION, got %q", state.TypeMappings["float"])
	}
	if state.TypeMappings["text"] != "NVARCHAR(MAX)" {
		t.Errorf("expected NVARCHAR(MAX), got %q", state.TypeMappings["text"])
	}
}

// TestSetTypeMappings_UpDown verifies that SetTypeMappings.Up and Down return empty SQL.
func TestSetTypeMappings_UpDown(t *testing.T) {
	state := migrate.NewSchemaState()
	op := &migrate.SetTypeMappings{TypeMappings: map[string]string{"float": "DOUBLE PRECISION"}}
	upSQL, err := op.Up(nil, state, nil)
	if err != nil || upSQL != "" {
		t.Errorf("SetTypeMappings.Up should return empty SQL, got %q err=%v", upSQL, err)
	}
	downSQL, err := op.Down(nil, state, nil)
	if err != nil || downSQL != "" {
		t.Errorf("SetTypeMappings.Down should return empty SQL, got %q err=%v", downSQL, err)
	}
}

// TestSetTypeMappings_Metadata verifies TypeName, TableName, IsDestructive, Describe.
func TestSetTypeMappings_Metadata(t *testing.T) {
	op := &migrate.SetTypeMappings{TypeMappings: map[string]string{}}
	if op.TypeName() != "set_type_mappings" {
		t.Errorf("TypeName = %q, want set_type_mappings", op.TypeName())
	}
	if op.TableName() != "" {
		t.Errorf("TableName = %q, want empty", op.TableName())
	}
	if op.IsDestructive() {
		t.Error("IsDestructive should be false")
	}
	if op.Describe() == "" {
		t.Error("Describe should be non-empty")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./migrate/ -run "TestSetTypeMappings" -v
```

Expected: `FAIL` — `migrate.SetTypeMappings undefined`.

**Step 3: Implement the operation**

In `migrate/operations.go`, add after the `SetDefaults` block (around line 650):

```go
// --- SetTypeMappings ---

// SetTypeMappings is a migration operation that records the active schema type mappings
// for the target database provider. It emits no SQL — its only effect is updating
// SchemaState.TypeMappings so that subsequent operations and the runner use the correct
// type overrides when generating SQL via ConvertFieldType.
type SetTypeMappings struct {
	TypeMappings map[string]string
}

// TypeName returns the operation type identifier.
func (op *SetTypeMappings) TypeName() string { return "set_type_mappings" }

// TableName returns "" — SetTypeMappings does not target a specific table.
func (op *SetTypeMappings) TableName() string { return "" }

// IsDestructive returns false — SetTypeMappings is always non-destructive.
func (op *SetTypeMappings) IsDestructive() bool { return false }

// Describe returns a human-readable description.
func (op *SetTypeMappings) Describe() string { return "Set schema type mappings" }

// Up returns empty string — SetTypeMappings emits no SQL.
func (op *SetTypeMappings) Up(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return "", nil
}

// Down returns empty string — SetTypeMappings emits no SQL.
func (op *SetTypeMappings) Down(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return "", nil
}

// Mutate applies the type mappings to the schema state.
func (op *SetTypeMappings) Mutate(state *SchemaState) error {
	state.SetTypeMappings(op.TypeMappings)
	return nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./migrate/ -run "TestSetTypeMappings" -v
```

Expected: all 3 tests `PASS`.

**Step 5: Run full migrate tests**

```bash
go test ./migrate/... 2>&1 | tail -5
```

Expected: `ok  	github.com/ocomsoft/makemigrations/migrate`

**Step 6: Commit**

```bash
git add migrate/operations.go migrate/operations_test.go
git commit -m "feat: add SetTypeMappings DAG operation"
```

---

### Task 3: Sync Provider TypeMappings in Runner

**Files:**
- Modify: `migrate/runner.go`
- Test: `migrate/runner_test.go` (if it exists, otherwise note it's covered by integration tests)

**Context:** The runner must call `provider.SetTypeMappings(state.TypeMappings)` before each `op.Up()`/`op.Down()` so that when a `SetTypeMappings` operation mutates state mid-migration, the next DDL operation sees the updated mappings on the provider.

**Step 1: Locate the three call sites in `runner.go`**

Find the three loops:
1. `ShowSQL` — `op.Up(r.provider, state, state.Defaults)` around line 194
2. `applyMigration` — `op.Up(r.provider, state, state.Defaults)` around line 213
3. `rollbackMigration` — `op.Down(r.provider, state, state.Defaults)` around line 245

**Step 2: Update `ShowSQL`**

Before `op.Up(r.provider, state, state.Defaults)` in `ShowSQL`, add:

```go
r.provider.SetTypeMappings(state.TypeMappings)
```

**Step 3: Update `applyMigration`**

Before `op.Up(r.provider, state, state.Defaults)` in `applyMigration`, add:

```go
r.provider.SetTypeMappings(state.TypeMappings)
```

**Step 4: Update `rollbackMigration`**

Before `op.Down(r.provider, state, state.Defaults)` in `rollbackMigration`, add:

```go
r.provider.SetTypeMappings(state.TypeMappings)
```

**Step 5: Run tests**

```bash
go test ./migrate/... -v 2>&1 | tail -20
```

Expected: all pass (no existing tests should break — `SetTypeMappings(nil)` on providers is safe as they already handle nil maps).

**Step 6: Commit**

```bash
git add migrate/runner.go
git commit -m "feat: sync provider TypeMappings from state before each operation"
```

---

### Task 4: Add `ChangeTypeTypeMappingsModified` to Diff Engine

**Files:**
- Modify: `internal/yaml/diff.go`
- Test: (verified by codegen tests in Task 6)

**Step 1: Add the constant**

In `internal/yaml/diff.go`, find the `ChangeType` constants block (around line 67). Add after `ChangeTypeDefaultsModified`:

```go
ChangeTypeTypeMappingsModified ChangeType = "type_mappings_modified" // non-destructive: updates active provider type mappings
```

**Step 2: Verify it compiles**

```bash
go build ./internal/yaml/...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add internal/yaml/diff.go
git commit -m "feat: add ChangeTypeTypeMappingsModified change type"
```

---

### Task 5: Diff Detection and `schemaStateToYAMLSchema` Update

**Files:**
- Modify: `cmd/go_migrations.go`
- Modify: `cmd/go_init.go`
- Test: `cmd/go_migrations_test.go`

**Step 1: Write failing tests for `schemaStateToYAMLSchema`**

Open `cmd/go_migrations_test.go`. Add:

```go
// TestSchemaStateToYAMLSchema_TypeMappings verifies that TypeMappings are repopulated
// from state into the correct provider field of the returned schema.
func TestSchemaStateToYAMLSchema_TypeMappings(t *testing.T) {
	state := migrate.NewSchemaState()
	state.SetTypeMappings(map[string]string{"float": "DOUBLE PRECISION"})

	result := schemaStateToYAMLSchema(state, "postgresql")
	if result == nil {
		t.Fatal("expected non-nil schema")
	}
	if result.TypeMappings.PostgreSQL["float"] != "DOUBLE PRECISION" {
		t.Errorf("expected DOUBLE PRECISION in PostgreSQL TypeMappings, got %q",
			result.TypeMappings.PostgreSQL["float"])
	}
	// Other providers should be nil
	if len(result.TypeMappings.MySQL) > 0 {
		t.Error("MySQL TypeMappings should be empty")
	}
}

// TestSchemaStateToYAMLSchema_TypeMappings_MySQL verifies MySQL provider mapping.
func TestSchemaStateToYAMLSchema_TypeMappings_MySQL(t *testing.T) {
	state := migrate.NewSchemaState()
	state.SetTypeMappings(map[string]string{"text": "LONGTEXT"})

	result := schemaStateToYAMLSchema(state, "mysql")
	if result.TypeMappings.MySQL["text"] != "LONGTEXT" {
		t.Errorf("expected LONGTEXT in MySQL TypeMappings, got %q", result.TypeMappings.MySQL["text"])
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./cmd/ -run "TestSchemaStateToYAMLSchema_TypeMappings" -v
```

Expected: `FAIL` — TypeMappings fields are empty.

**Step 3: Update `schemaStateToYAMLSchema` in `cmd/go_migrations.go`**

Find the Defaults repopulation block (around line 444). Add after it:

```go
// Populate TypeMappings so that type mapping changes are detected on subsequent diff runs.
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
	case "redshift":
		schema.TypeMappings.Redshift = state.TypeMappings
	case "clickhouse":
		schema.TypeMappings.ClickHouse = state.TypeMappings
	case "tidb":
		schema.TypeMappings.TiDB = state.TypeMappings
	case "vertica":
		schema.TypeMappings.Vertica = state.TypeMappings
	case "ydb":
		schema.TypeMappings.YDB = state.TypeMappings
	case "turso":
		schema.TypeMappings.Turso = state.TypeMappings
	case "starrocks":
		schema.TypeMappings.StarRocks = state.TypeMappings
	case "auroradsql":
		schema.TypeMappings.AuroraDSQL = state.TypeMappings
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./cmd/ -run "TestSchemaStateToYAMLSchema_TypeMappings" -v
```

Expected: `PASS`.

**Step 5: Add `getTypeMappingsForDB` helper**

In `cmd/go_migrations.go`, after `getDefaultsForDB`, add:

```go
// getTypeMappingsForDB returns the type mappings for the given database type from a schema.
// Returns nil when the schema or the relevant DB type mappings are empty.
func getTypeMappingsForDB(schema *yamlpkg.Schema, dbType string) map[string]string {
	if schema == nil {
		return nil
	}
	return schema.TypeMappings.ForProvider(types.DatabaseType(dbType))
}
```

Note: add `"github.com/ocomsoft/makemigrations/internal/types"` to the import if not already present.

**Step 6: Write failing test for diff detection**

In `cmd/go_migrations_test.go`, add:

```go
// TestGetTypeMappingsForDB verifies the helper returns the correct provider map.
func TestGetTypeMappingsForDB(t *testing.T) {
	schema := &yamlpkg.Schema{}
	schema.TypeMappings.PostgreSQL = map[string]string{"float": "DOUBLE PRECISION"}
	schema.TypeMappings.MySQL = map[string]string{"text": "LONGTEXT"}

	pg := getTypeMappingsForDB(schema, "postgresql")
	if pg["float"] != "DOUBLE PRECISION" {
		t.Errorf("pg: expected DOUBLE PRECISION, got %q", pg["float"])
	}
	my := getTypeMappingsForDB(schema, "mysql")
	if my["text"] != "LONGTEXT" {
		t.Errorf("mysql: expected LONGTEXT, got %q", my["text"])
	}
	if got := getTypeMappingsForDB(nil, "postgresql"); got != nil {
		t.Errorf("nil schema should return nil, got %v", got)
	}
}
```

**Step 7: Run test and verify pass**

```bash
go test ./cmd/ -run "TestGetTypeMappingsForDB" -v
```

Expected: `PASS`.

**Step 8: Add diff detection to `runGoMigrations` in `cmd/go_migrations.go`**

Find the Defaults diff detection block (around line 155). Add after it:

```go
// Prepend a SetTypeMappings operation when the active DB type mappings have changed.
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

**Step 9: Add SetTypeMappings prepend to `cmd/go_init.go`**

In `cmd/go_init.go`, find the `schemaToInitialDiff` function. Find the Defaults prepend block (around line 132). Add after it:

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

Note: `getTypeMappingsForDB` is in `cmd/go_migrations.go` (same package `cmd`), so no import needed.

**Step 10: Build to verify no compile errors**

```bash
go build ./cmd/...
```

Expected: no errors.

**Step 11: Run all cmd tests**

```bash
go test ./cmd/... 2>&1 | tail -10
```

Expected: all pass.

**Step 12: Commit**

```bash
git add cmd/go_migrations.go cmd/go_init.go cmd/go_migrations_test.go
git commit -m "feat: detect TypeMappings changes in diff and repopulate in schemaStateToYAMLSchema"
```

---

### Task 6: Codegen for `SetTypeMappings`

**Files:**
- Modify: `internal/codegen/go_generator.go`
- Test: `internal/codegen/go_generator_test.go`

**Step 1: Write the failing test**

Open `internal/codegen/go_generator_test.go`. Add after the `TestGoGenerator_SetDefaults` test:

```go
// TestGoGenerator_SetTypeMappings verifies that a ChangeTypeTypeMappingsModified change
// generates a valid &m.SetTypeMappings{...} literal with sorted keys.
func TestGoGenerator_SetTypeMappings(t *testing.T) {
	g := codegen.NewGoGenerator("testpkg", "test_migration", []string{})
	change := yaml.Change{
		Type:        yaml.ChangeTypeTypeMappingsModified,
		Description: "Update schema type mappings",
		NewValue: map[string]string{
			"float": "DOUBLE PRECISION",
			"text":  "NVARCHAR(MAX)",
		},
	}
	src, err := g.GenerateOperations([]yaml.Change{change}, map[yaml.Change]yaml.ChangeDecision{
		change: yaml.DecisionGenerate,
	})
	if err != nil {
		t.Fatalf("GenerateOperations error: %v", err)
	}
	if !strings.Contains(src, "SetTypeMappings") {
		t.Error("expected SetTypeMappings in output")
	}
	if !strings.Contains(src, `"float": "DOUBLE PRECISION"`) {
		t.Error("expected float mapping in output")
	}
	if !strings.Contains(src, `"text": "NVARCHAR(MAX)"`) {
		t.Error("expected text mapping in output")
	}
	// Verify sorted order: float before text
	floatIdx := strings.Index(src, `"float"`)
	textIdx := strings.Index(src, `"text"`)
	if floatIdx > textIdx {
		t.Error("expected float before text (sorted keys)")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/codegen/ -run "TestGoGenerator_SetTypeMappings" -v
```

Expected: `FAIL` — unknown change type or missing SetTypeMappings in output.

**Step 3: Implement the dispatch case**

In `internal/codegen/go_generator.go`, find the change type dispatch switch (around line 160). Add after the `ChangeTypeDefaultsModified` case:

```go
case yaml.ChangeTypeTypeMappingsModified:
	return g.generateSetTypeMappings(change)
```

**Step 4: Implement `generateSetTypeMappings`**

Add after `generateSetDefaults` (around line 390):

```go
// generateSetTypeMappings emits a &m.SetTypeMappings{...} literal.
// Keys are sorted for deterministic output.
func (g *GoGenerator) generateSetTypeMappings(change yaml.Change) (string, error) {
	mappings, ok := change.NewValue.(map[string]string)
	if !ok {
		return "", fmt.Errorf("SetTypeMappings: expected map[string]string NewValue, got %T", change.NewValue)
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

Note: `sortedKeys` already exists in the file (used by `generateSetDefaults`) — do not duplicate it.

**Step 5: Run test to verify it passes**

```bash
go test ./internal/codegen/ -run "TestGoGenerator_SetTypeMappings" -v
```

Expected: `PASS`.

**Step 6: Run all codegen tests**

```bash
go test ./internal/codegen/... 2>&1 | tail -10
```

Expected: all pass.

**Step 7: Commit**

```bash
git add internal/codegen/go_generator.go internal/codegen/go_generator_test.go
git commit -m "feat: add SetTypeMappings codegen for type_mappings_modified change type"
```

---

### Task 7: Update `docs/schema-format.md`

**Files:**
- Modify: `docs/schema-format.md`

**Step 1: Locate the `Defaults Section` heading**

Find the `## Defaults Section` heading. Add a new `## Type Mappings Section` **before** it (between the `Include Section` and `Defaults Section`).

**Step 2: Add the Type Mappings section**

Insert this content:

````markdown
## Type Mappings Section

The `type_mappings` section lets you override the SQL type that makemigrations generates for a
given schema field type on a per-database basis. This is useful when the built-in type mapping
is not suitable for your target database.

```yaml
type_mappings:
  postgresql:
    float: "DOUBLE PRECISION"   # override float → DOUBLE PRECISION instead of REAL
    text: "CITEXT"              # use case-insensitive text extension
  sqlserver:
    text: "NVARCHAR(MAX)"       # use unicode text for SQL Server
  mysql:
    uuid: "CHAR(36)"            # explicit char length for UUIDs
```

### Supported Providers

Type mappings can be defined for any supported database:

| Key | Database |
|-----|----------|
| `postgresql` | PostgreSQL |
| `mysql` | MySQL |
| `sqlserver` | SQL Server |
| `sqlite` | SQLite |
| `redshift` | Amazon Redshift |
| `clickhouse` | ClickHouse |
| `tidb` | TiDB |
| `vertica` | Vertica |
| `ydb` | YDB |
| `turso` | Turso |
| `starrocks` | StarRocks |
| `auroradsql` | Aurora DSQL |

### Parameterised Types

For types that take parameters (length, precision, scale), use Go template syntax:

```yaml
type_mappings:
  postgresql:
    decimal: "NUMERIC({{.Precision}},{{.Scale}})"
    varchar: "CHARACTER VARYING({{.Length}})"
```

The available template variables are: `.Length`, `.Precision`, `.Scale`.

### DAG Integration

When `type_mappings` change between runs, makemigrations automatically generates a
`SetTypeMappings` operation in the migration file. This operation has no SQL effect — it
records the type mapping configuration in the migration DAG so that:

- Subsequent migrations use the correct type overrides
- `db-diff` compares the live database against the correct expected types
- The migration history accurately reflects when type mappings changed

Example generated migration operation:

```go
&m.SetTypeMappings{
    TypeMappings: map[string]string{
        "float": "DOUBLE PRECISION",
    },
},
```

### Built-in Type Mappings

The following table shows the default SQL types used when no `type_mappings` override is set.
Use `type_mappings` to override any of these:

| Schema Type | PostgreSQL | MySQL | SQLite | SQL Server |
|-------------|------------|-------|--------|------------|
| `float` | `REAL` | `FLOAT` | `REAL` | `FLOAT` |
| `text` | `TEXT` | `TEXT` | `TEXT` | `NVARCHAR(MAX)` |
| `boolean` | `BOOLEAN` | `TINYINT(1)` | `INTEGER` | `BIT` |
| `uuid` | `UUID` | `CHAR(36)` | `TEXT` | `UNIQUEIDENTIFIER` |
| `jsonb` | `JSONB` | `JSON` | `TEXT` | `NVARCHAR(MAX)` |
````

**Step 3: Verify the docs build (no broken links)**

```bash
grep -n "type_mappings\|Type Mappings" docs/schema-format.md | head -20
```

Expected: the new section headings and references appear.

**Step 4: Commit**

```bash
git add docs/schema-format.md
git commit -m "docs: add type_mappings section to schema-format guide"
```

---

### Task 8: Full Test Suite + Lint

**Step 1: Run all tests**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./... 2>&1 | tail -30
```

Expected: all packages pass, no failures.

**Step 2: Run goimports on changed files**

```bash
goimports -w migrate/state.go migrate/operations.go migrate/runner.go \
  cmd/go_migrations.go cmd/go_init.go \
  internal/yaml/diff.go internal/codegen/go_generator.go
```

**Step 3: Run golangci-lint**

```bash
golangci-lint run --no-config ./... 2>&1 | grep -v "^#" | head -40
```

Expected: no new issues beyond pre-existing ones in `db2schema.go`, `goose.go`, `schema2diagram.go`.

**Step 4: Fix any lint issues**

Address any new issues reported. Do not touch pre-existing issues in unrelated files.

**Step 5: Final commit if lint required changes**

```bash
git add -u
git commit -m "fix: address golangci-lint issues in type-mappings DAG integration"
```

---

### Task 9: Verification

**Step 1: Confirm all files touched**

```bash
git log --oneline -10
```

Expected: see all commits from Tasks 1–8.

**Step 2: Smoke test with example schema**

Create a temp schema with type_mappings and verify no panics:

```bash
cat > /tmp/test_schema.yaml << 'EOF'
database:
  name: testdb
  version: 1.0.0
type_mappings:
  postgresql:
    float: "DOUBLE PRECISION"
tables:
  - name: items
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: weight
        type: float
        nullable: true
EOF
```

Then confirm `go build ./...` succeeds:

```bash
go build ./...
```

**Step 3: Update memory**

Update `/home/ocom/.claude/projects/-workspaces-ocom-go-makemigrations/memory/MEMORY.md` to document the `TypeMappings` DAG pattern alongside `Defaults`.
