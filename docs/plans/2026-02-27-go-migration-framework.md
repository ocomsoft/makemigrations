# Go Migration Framework Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the SQL-file generation pipeline with a Django-style Go migration framework that generates compiled Go migration files with a dependency DAG, typed operations, merge migrations, squash, and state reconstruction.

**Architecture:** Outside-in: `migrate/` types/operations/registry → code generator → graph → DAG CLI + binary-query loop → branch/merge → runner/recorder → squash → backward compat rename. All code lives in `github.com/ocomsoft/makemigrations` (single module). The `migrate/` directory is a package, not a separate module. Generated user `migrations/go.mod` files import `github.com/ocomsoft/makemigrations/migrate`.

**Tech Stack:** Go 1.24, Cobra, `text/template`, `go/format`, `database/sql`, `os/exec`, existing `internal/providers` interface (untouched).

**Key data structures to understand before starting:**
- `internal/yaml/diff.go`: `Change{Type, TableName, FieldName, OldValue interface{}, NewValue interface{}, Destructive}` — `OldValue`/`NewValue` are concrete `types.Table`, `types.Field`, or `types.Index` values
- `internal/types/types.go`: `Field{Name, Type, PrimaryKey, Nullable *bool, Default, Length, Precision, Scale, AutoCreate, AutoUpdate, ForeignKey, ManyToMany}`, `Table{Name, Fields, Indexes}`, `Schema{Database, Defaults, Tables}`
- `internal/yaml/types.go`: all types are aliases to `internal/types`
- `internal/providers/provider.go`: the `Provider` interface (never modify)

---

## Task 1: `migrate/types.go` and `migrate/state.go`

**Files:**
- Create: `migrate/types.go`
- Create: `migrate/state.go`
- Create: `migrate/state_test.go`

### Step 1: Create `migrate/types.go`

```go
// Package migrate provides the runtime library for the makemigrations Go migration framework.
// Generated migration files import this package and call Register() in their init() functions.
package migrate

// Migration represents a single database migration with its name, dependencies, and operations.
type Migration struct {
	Name         string      // Unique identifier e.g. "0001_initial"
	Dependencies []string    // Names of migrations this depends on
	Operations   []Operation // Ordered list of schema operations to apply
	Replaces     []string    // For squashed migrations: names of migrations this replaces
}

// Field represents a database column definition used in migration operations.
type Field struct {
	Name       string
	Type       string // varchar, text, integer, uuid, boolean, timestamp, foreign_key, etc.
	PrimaryKey bool
	Nullable   bool
	Default    string // default reference name e.g. "new_uuid", "now", "true"
	Length     int    // for varchar
	Precision  int    // for decimal/numeric
	Scale      int    // for decimal/numeric
	AutoCreate bool   // auto-set on row creation (created_at)
	AutoUpdate bool   // auto-set on row update (updated_at)
	ForeignKey *ForeignKey
	ManyToMany *ManyToMany
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Table    string
	OnDelete string
	OnUpdate string
}

// ManyToMany represents a many-to-many relationship via junction table.
type ManyToMany struct {
	Table string
}

// Index represents a database index definition.
type Index struct {
	Name   string
	Fields []string
	Unique bool
}
```

### Step 2: Create `migrate/state.go`

```go
package migrate

import "fmt"

// SchemaState holds the in-memory representation of the database schema at a
// specific point in the migration graph. Operations call Mutate() to update
// it as they are applied during graph traversal.
type SchemaState struct {
	Tables map[string]*TableState
}

// TableState holds the state of a single table.
type TableState struct {
	Name    string
	Fields  []Field
	Indexes []Index
}

// NewSchemaState returns an empty SchemaState.
func NewSchemaState() *SchemaState {
	return &SchemaState{Tables: make(map[string]*TableState)}
}

// AddTable adds a new table. Returns error if the table already exists.
func (s *SchemaState) AddTable(name string, fields []Field, indexes []Index) error {
	if _, exists := s.Tables[name]; exists {
		return fmt.Errorf("table %q already exists in schema state", name)
	}
	if indexes == nil {
		indexes = []Index{}
	}
	s.Tables[name] = &TableState{Name: name, Fields: fields, Indexes: indexes}
	return nil
}

// DropTable removes a table. Returns error if the table does not exist.
func (s *SchemaState) DropTable(name string) error {
	if _, exists := s.Tables[name]; !exists {
		return fmt.Errorf("table %q does not exist in schema state", name)
	}
	delete(s.Tables, name)
	return nil
}

// RenameTable renames a table. Returns error if old name does not exist.
func (s *SchemaState) RenameTable(oldName, newName string) error {
	t, exists := s.Tables[oldName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", oldName)
	}
	t.Name = newName
	s.Tables[newName] = t
	delete(s.Tables, oldName)
	return nil
}

// AddField appends a field to an existing table.
func (s *SchemaState) AddField(tableName string, field Field) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	t.Fields = append(t.Fields, field)
	return nil
}

// DropField removes a named field from an existing table.
func (s *SchemaState) DropField(tableName, fieldName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == fieldName {
			t.Fields = append(t.Fields[:i], t.Fields[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", fieldName, tableName)
}

// AlterField replaces a field (matched by name) in an existing table.
func (s *SchemaState) AlterField(tableName string, newField Field) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == newField.Name {
			t.Fields[i] = newField
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", newField.Name, tableName)
}

// RenameField renames a field within an existing table.
func (s *SchemaState) RenameField(tableName, oldName, newName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == oldName {
			t.Fields[i].Name = newName
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", oldName, tableName)
}

// AddIndex appends an index to an existing table.
func (s *SchemaState) AddIndex(tableName string, index Index) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	t.Indexes = append(t.Indexes, index)
	return nil
}

// DropIndex removes a named index from an existing table.
func (s *SchemaState) DropIndex(tableName, indexName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, idx := range t.Indexes {
		if idx.Name == indexName {
			t.Indexes = append(t.Indexes[:i], t.Indexes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("index %q does not exist in table %q", indexName, tableName)
}
```

### Step 3: Create `migrate/state_test.go`

```go
package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestSchemaState_AddTable(t *testing.T) {
	s := migrate.NewSchemaState()
	err := s.AddTable("users", []migrate.Field{
		{Name: "id", Type: "uuid", PrimaryKey: true},
		{Name: "email", Type: "varchar", Length: 255},
	}, nil)
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if _, ok := s.Tables["users"]; !ok {
		t.Fatal("expected 'users' in Tables")
	}
	if len(s.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.Tables["users"].Fields))
	}
}

func TestSchemaState_AddTable_Duplicate(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.AddTable("users", nil, nil); err == nil {
		t.Fatal("expected error for duplicate table")
	}
}

func TestSchemaState_DropTable(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.DropTable("users"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	if _, ok := s.Tables["users"]; ok {
		t.Fatal("expected 'users' to be removed")
	}
}

func TestSchemaState_DropTable_NotExists(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.DropTable("missing"); err == nil {
		t.Fatal("expected error for missing table")
	}
}

func TestSchemaState_RenameTable(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("old", nil, nil)
	if err := s.RenameTable("old", "new"); err != nil {
		t.Fatalf("RenameTable: %v", err)
	}
	if _, ok := s.Tables["new"]; !ok {
		t.Fatal("expected 'new' table")
	}
	if _, ok := s.Tables["old"]; ok {
		t.Fatal("expected 'old' to be removed")
	}
}

func TestSchemaState_AddDropField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "id", Type: "uuid"}}, nil)
	if err := s.AddField("users", migrate.Field{Name: "email", Type: "varchar"}); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if len(s.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.Tables["users"].Fields))
	}
	if err := s.DropField("users", "email"); err != nil {
		t.Fatalf("DropField: %v", err)
	}
	if len(s.Tables["users"].Fields) != 1 {
		t.Fatalf("expected 1 field after drop, got %d", len(s.Tables["users"].Fields))
	}
}

func TestSchemaState_AlterField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "email", Type: "varchar", Length: 100}}, nil)
	if err := s.AlterField("users", migrate.Field{Name: "email", Type: "varchar", Length: 255}); err != nil {
		t.Fatalf("AlterField: %v", err)
	}
	if s.Tables["users"].Fields[0].Length != 255 {
		t.Fatalf("expected length 255, got %d", s.Tables["users"].Fields[0].Length)
	}
}

func TestSchemaState_RenameField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "old_col", Type: "varchar"}}, nil)
	if err := s.RenameField("users", "old_col", "new_col"); err != nil {
		t.Fatalf("RenameField: %v", err)
	}
	if s.Tables["users"].Fields[0].Name != "new_col" {
		t.Fatalf("expected 'new_col', got %q", s.Tables["users"].Fields[0].Name)
	}
}

func TestSchemaState_AddDropIndex(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.AddIndex("users", migrate.Index{Name: "idx_email", Fields: []string{"email"}, Unique: true}); err != nil {
		t.Fatalf("AddIndex: %v", err)
	}
	if len(s.Tables["users"].Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(s.Tables["users"].Indexes))
	}
	if err := s.DropIndex("users", "idx_email"); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}
	if len(s.Tables["users"].Indexes) != 0 {
		t.Fatalf("expected 0 indexes after drop, got %d", len(s.Tables["users"].Indexes))
	}
}
```

### Step 4: Run test (expect compile error — types exist but no operations yet)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... 2>&1 | head -20
```

Expected: tests compile and pass (state_test.go only depends on types.go and state.go).

### Step 5: Commit

```bash
git add migrate/types.go migrate/state.go migrate/state_test.go
git commit -m "feat(migrate): add types, SchemaState and tests"
```

---

## Task 2: `migrate/operations.go`

**Files:**
- Create: `migrate/operations.go`
- Create: `migrate/operations_test.go`

### Step 1: Write failing test first

```go
// migrate/operations_test.go
package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/migrate"
)

func TestCreateTable_Forward(t *testing.T) {
	p := sqlite.NewProvider()
	state := migrate.NewSchemaState()
	op := &migrate.CreateTable{
		Name: "users",
		Fields: []migrate.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "email", Type: "varchar", Length: 255, Nullable: false},
		},
	}
	sql, err := op.Up(p, state, nil)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if err := op.Mutate(state); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if _, ok := state.Tables["users"]; !ok {
		t.Fatal("expected 'users' in state after Mutate")
	}
}

func TestCreateTable_Backward(t *testing.T) {
	p := sqlite.NewProvider()
	state := migrate.NewSchemaState()
	op := &migrate.CreateTable{Name: "users", Fields: []migrate.Field{{Name: "id", Type: "uuid"}}}
	sql, err := op.Down(p, state, nil)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty backward SQL")
	}
}

func TestAddField_ForwardBackward(t *testing.T) {
	p := sqlite.NewProvider()
	state := migrate.NewSchemaState()
	_ = state.AddTable("users", []migrate.Field{{Name: "id", Type: "uuid"}}, nil)
	op := &migrate.AddField{
		Table: "users",
		Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
	}
	upSQL, err := op.Up(p, state, nil)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if upSQL == "" {
		t.Fatal("expected non-empty SQL")
	}
	downSQL, err := op.Down(p, state, nil)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if downSQL == "" {
		t.Fatal("expected non-empty down SQL")
	}
	if err := op.Mutate(state); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(state.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields after Mutate, got %d", len(state.Tables["users"].Fields))
	}
}

func TestDropTable_IsDestructive(t *testing.T) {
	op := &migrate.DropTable{Name: "users"}
	if !op.IsDestructive() {
		t.Fatal("expected DropTable to be destructive")
	}
}

func TestDropField_IsDestructive(t *testing.T) {
	op := &migrate.DropField{Table: "users", Field: "email"}
	if !op.IsDestructive() {
		t.Fatal("expected DropField to be destructive")
	}
}

func TestRunSQL_ForwardBackward(t *testing.T) {
	op := &migrate.RunSQL{
		ForwardSQL:  "UPDATE posts SET slug = 'x'",
		BackwardSQL: "UPDATE posts SET slug = NULL",
	}
	sql, _ := op.Up(nil, nil, nil)
	if sql != "UPDATE posts SET slug = 'x'" {
		t.Fatalf("expected forward SQL, got %q", sql)
	}
	back, _ := op.Down(nil, nil, nil)
	if back != "UPDATE posts SET slug = NULL" {
		t.Fatalf("expected backward SQL, got %q", back)
	}
	if op.Mutate(migrate.NewSchemaState()) != nil {
		t.Fatal("RunSQL.Mutate should not error")
	}
}
```

### Step 2: Run test to confirm it fails

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... 2>&1 | head -20
```

Expected: compile error — `CreateTable`, `AddField`, etc. not defined.

### Step 3: Create `migrate/operations.go`

**IMPORTANT:** The `Operation` interface uses `Up`/`Down` method names (not `Forward`/`Backward`) to avoid conflict with `RunSQL` struct fields named `ForwardSQL`/`BackwardSQL`. The `Up`/`Down` naming also aligns with the CLI commands.

**IMPORTANT:** `internal/types.Field.Nullable` is `*bool`. Convert with `boolPtr(f.Nullable)` helper.

```go
// Package migrate provides the runtime library for the makemigrations Go migration framework.
package migrate

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers"
	"github.com/ocomsoft/makemigrations/internal/types"
)

// Operation is the interface all migration operations must implement.
// Up generates SQL to apply the operation; Down generates SQL to reverse it.
// Mutate updates the in-memory SchemaState after the operation is applied.
type Operation interface {
	Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error)
	Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error)
	Mutate(state *SchemaState) error
	Describe() string
	TypeName() string
	TableName() string
	IsDestructive() bool
}

// boolPtr converts a bool to *bool for internal/types.Field.Nullable.
func boolPtr(b bool) *bool { return &b }

// toTypesField converts a migrate.Field to an internal/types.Field.
func toTypesField(f Field) *types.Field {
	tf := &types.Field{
		Name:       f.Name,
		Type:       f.Type,
		PrimaryKey: f.PrimaryKey,
		Nullable:   boolPtr(f.Nullable),
		Default:    f.Default,
		Length:     f.Length,
		Precision:  f.Precision,
		Scale:      f.Scale,
		AutoCreate: f.AutoCreate,
		AutoUpdate: f.AutoUpdate,
	}
	if f.ForeignKey != nil {
		tf.ForeignKey = &types.ForeignKey{
			Table:    f.ForeignKey.Table,
			OnDelete: f.ForeignKey.OnDelete,
		}
	}
	if f.ManyToMany != nil {
		tf.ManyToMany = &types.ManyToMany{Table: f.ManyToMany.Table}
	}
	return tf
}

// stateToSchema builds a minimal types.Schema from a SchemaState for provider calls
// that require the full schema context (e.g. GenerateCreateTable needs FK resolution).
func stateToSchema(state *SchemaState) *types.Schema {
	if state == nil {
		return &types.Schema{}
	}
	s := &types.Schema{}
	for _, ts := range state.Tables {
		t := &types.Table{Name: ts.Name}
		for _, f := range ts.Fields {
			t.Fields = append(t.Fields, *toTypesField(f))
		}
		for _, idx := range ts.Indexes {
			t.Indexes = append(t.Indexes, types.Index{
				Name:   idx.Name,
				Fields: idx.Fields,
				Unique: idx.Unique,
			})
		}
		s.Tables = append(s.Tables, *t)
	}
	return s
}

// joinFields joins field name strings with ", ".
func joinFields(fields []string) string {
	return strings.Join(fields, ", ")
}

// --- CreateTable ---

// CreateTable creates a new database table.
type CreateTable struct {
	Name    string
	Fields  []Field
	Indexes []Index
}

func (op *CreateTable) TypeName() string    { return "create_table" }
func (op *CreateTable) TableName() string   { return op.Name }
func (op *CreateTable) IsDestructive() bool { return false }
func (op *CreateTable) Describe() string {
	return fmt.Sprintf("Create table %s (%d fields)", op.Name, len(op.Fields))
}

func (op *CreateTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	schema := stateToSchema(state)
	table := &types.Table{Name: op.Name}
	for _, f := range op.Fields {
		table.Fields = append(table.Fields, *toTypesField(f))
	}
	for _, idx := range op.Indexes {
		table.Indexes = append(table.Indexes, types.Index{Name: idx.Name, Fields: idx.Fields, Unique: idx.Unique})
	}
	return p.GenerateCreateTable(schema, table)
}

func (op *CreateTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropTable(op.Name), nil
}

func (op *CreateTable) Mutate(state *SchemaState) error {
	return state.AddTable(op.Name, op.Fields, op.Indexes)
}

// --- DropTable ---

// DropTable drops an existing table. Destructive operation.
type DropTable struct {
	Name string
}

func (op *DropTable) TypeName() string    { return "drop_table" }
func (op *DropTable) TableName() string   { return op.Name }
func (op *DropTable) IsDestructive() bool { return true }
func (op *DropTable) Describe() string    { return fmt.Sprintf("Drop table %s", op.Name) }

func (op *DropTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropTable(op.Name), nil
}

func (op *DropTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	ts, exists := state.Tables[op.Name]
	if !exists {
		return "", fmt.Errorf("table %q not found in state for Down generation", op.Name)
	}
	schema := stateToSchema(state)
	t := &types.Table{Name: ts.Name}
	for _, f := range ts.Fields {
		t.Fields = append(t.Fields, *toTypesField(f))
	}
	for _, idx := range ts.Indexes {
		t.Indexes = append(t.Indexes, types.Index{Name: idx.Name, Fields: idx.Fields, Unique: idx.Unique})
	}
	return p.GenerateCreateTable(schema, t)
}

func (op *DropTable) Mutate(state *SchemaState) error {
	return state.DropTable(op.Name)
}

// --- RenameTable ---

// RenameTable renames an existing table.
type RenameTable struct {
	OldName string
	NewName string
}

func (op *RenameTable) TypeName() string    { return "rename_table" }
func (op *RenameTable) TableName() string   { return op.OldName }
func (op *RenameTable) IsDestructive() bool { return false }
func (op *RenameTable) Describe() string {
	return fmt.Sprintf("Rename table %s to %s", op.OldName, op.NewName)
}

func (op *RenameTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameTable(op.OldName, op.NewName), nil
}

func (op *RenameTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameTable(op.NewName, op.OldName), nil
}

func (op *RenameTable) Mutate(state *SchemaState) error {
	return state.RenameTable(op.OldName, op.NewName)
}

// --- AddField ---

// AddField adds a column to an existing table.
type AddField struct {
	Table string
	Field Field
}

func (op *AddField) TypeName() string    { return "add_field" }
func (op *AddField) TableName() string   { return op.Table }
func (op *AddField) IsDestructive() bool { return false }
func (op *AddField) Describe() string {
	return fmt.Sprintf("Add field %s.%s %s", op.Table, op.Field.Name, op.Field.Type)
}

func (op *AddField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateAddColumn(op.Table, toTypesField(op.Field)), nil
}

func (op *AddField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropColumn(op.Table, op.Field.Name), nil
}

func (op *AddField) Mutate(state *SchemaState) error {
	return state.AddField(op.Table, op.Field)
}

// --- DropField ---

// DropField removes a column from an existing table. Destructive operation.
type DropField struct {
	Table string
	Field string
}

func (op *DropField) TypeName() string    { return "drop_field" }
func (op *DropField) TableName() string   { return op.Table }
func (op *DropField) IsDestructive() bool { return true }
func (op *DropField) Describe() string    { return fmt.Sprintf("Drop field %s.%s", op.Table, op.Field) }

func (op *DropField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropColumn(op.Table, op.Field), nil
}

func (op *DropField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	ts, exists := state.Tables[op.Table]
	if !exists {
		return "", fmt.Errorf("table %q not found in state", op.Table)
	}
	for _, f := range ts.Fields {
		if f.Name == op.Field {
			return p.GenerateAddColumn(op.Table, toTypesField(f)), nil
		}
	}
	return "", fmt.Errorf("field %q not found in table %q state", op.Field, op.Table)
}

func (op *DropField) Mutate(state *SchemaState) error {
	return state.DropField(op.Table, op.Field)
}

// --- AlterField ---

// AlterField modifies an existing column definition.
type AlterField struct {
	Table    string
	OldField Field
	NewField Field
}

func (op *AlterField) TypeName() string    { return "alter_field" }
func (op *AlterField) TableName() string   { return op.Table }
func (op *AlterField) IsDestructive() bool { return false }
func (op *AlterField) Describe() string {
	return fmt.Sprintf("Alter field %s.%s", op.Table, op.NewField.Name)
}

func (op *AlterField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateAlterColumn(op.Table, toTypesField(op.OldField), toTypesField(op.NewField))
}

func (op *AlterField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateAlterColumn(op.Table, toTypesField(op.NewField), toTypesField(op.OldField))
}

func (op *AlterField) Mutate(state *SchemaState) error {
	return state.AlterField(op.Table, op.NewField)
}

// --- RenameField ---

// RenameField renames an existing column.
type RenameField struct {
	Table   string
	OldName string
	NewName string
}

func (op *RenameField) TypeName() string    { return "rename_field" }
func (op *RenameField) TableName() string   { return op.Table }
func (op *RenameField) IsDestructive() bool { return false }
func (op *RenameField) Describe() string {
	return fmt.Sprintf("Rename field %s.%s to %s", op.Table, op.OldName, op.NewName)
}

func (op *RenameField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameColumn(op.Table, op.OldName, op.NewName), nil
}

func (op *RenameField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameColumn(op.Table, op.NewName, op.OldName), nil
}

func (op *RenameField) Mutate(state *SchemaState) error {
	return state.RenameField(op.Table, op.OldName, op.NewName)
}

// --- AddIndex ---

// AddIndex adds an index to an existing table.
type AddIndex struct {
	Table string
	Index Index
}

func (op *AddIndex) TypeName() string    { return "add_index" }
func (op *AddIndex) TableName() string   { return op.Table }
func (op *AddIndex) IsDestructive() bool { return false }
func (op *AddIndex) Describe() string {
	return fmt.Sprintf("Add index %s on %s(%s)", op.Index.Name, op.Table, joinFields(op.Index.Fields))
}

func (op *AddIndex) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	ti := &types.Index{Name: op.Index.Name, Unique: op.Index.Unique, Fields: op.Index.Fields}
	return p.GenerateCreateIndex(ti, op.Table), nil
}

func (op *AddIndex) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropIndex(op.Index.Name, op.Table), nil
}

func (op *AddIndex) Mutate(state *SchemaState) error {
	return state.AddIndex(op.Table, op.Index)
}

// --- DropIndex ---

// DropIndex removes an index from a table.
type DropIndex struct {
	Table string
	Index string
}

func (op *DropIndex) TypeName() string    { return "drop_index" }
func (op *DropIndex) TableName() string   { return op.Table }
func (op *DropIndex) IsDestructive() bool { return false }
func (op *DropIndex) Describe() string    { return fmt.Sprintf("Drop index %s on %s", op.Index, op.Table) }

func (op *DropIndex) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropIndex(op.Index, op.Table), nil
}

func (op *DropIndex) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	ts, exists := state.Tables[op.Table]
	if !exists {
		return "", fmt.Errorf("table %q not found in state", op.Table)
	}
	for _, idx := range ts.Indexes {
		if idx.Name == op.Index {
			ti := &types.Index{Name: idx.Name, Unique: idx.Unique, Fields: idx.Fields}
			return p.GenerateCreateIndex(ti, op.Table), nil
		}
	}
	return "", fmt.Errorf("index %q not found in table %q state", op.Index, op.Table)
}

func (op *DropIndex) Mutate(state *SchemaState) error {
	return state.DropIndex(op.Table, op.Index)
}

// --- RunSQL ---

// RunSQL executes raw SQL with manually provided forward and backward statements.
// Use this as an escape hatch for data migrations or unsupported schema operations.
type RunSQL struct {
	ForwardSQL  string
	BackwardSQL string
}

func (op *RunSQL) TypeName() string    { return "run_sql" }
func (op *RunSQL) TableName() string   { return "" }
func (op *RunSQL) IsDestructive() bool { return false }
func (op *RunSQL) Describe() string    { return "Run SQL" }

func (op *RunSQL) Up(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return op.ForwardSQL, nil
}

func (op *RunSQL) Down(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return op.BackwardSQL, nil
}

func (op *RunSQL) Mutate(_ *SchemaState) error { return nil }
```

### Step 4: Run tests

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... -v 2>&1 | tail -30
```

Expected: all tests pass.

### Step 5: Lint

```bash
cd /workspaces/ocom/go/makemigrations && golangci-lint run --no-config ./migrate/... 2>&1
```

Expected: no issues.

### Step 6: Commit

```bash
git add migrate/operations.go migrate/operations_test.go
git commit -m "feat(migrate): add Operation interface and all concrete types"
```

---

## Task 3: `migrate/registry.go`

**Files:**
- Create: `migrate/registry.go`
- Create: `migrate/registry_test.go`

### Step 1: Write failing test

```go
// migrate/registry_test.go
package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestRegistry_Register(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	all := reg.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(all))
	}
	if all[0].Name != "0001_initial" {
		t.Fatalf("expected '0001_initial', got %q", all[0].Name)
	}
}

func TestRegistry_Register_Duplicate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate migration name")
		}
	}()
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	reg.Register(&migrate.Migration{Name: "0001_initial"}) // should panic
}

func TestRegistry_Get(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	m, ok := reg.Get("0001_initial")
	if !ok {
		t.Fatal("expected to find '0001_initial'")
	}
	if m.Name != "0001_initial" {
		t.Fatalf("expected '0001_initial', got %q", m.Name)
	}
	_, ok = reg.Get("missing")
	if ok {
		t.Fatal("expected false for missing migration")
	}
}

func TestRegistry_InsertionOrder(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0002_second"})
	reg.Register(&migrate.Migration{Name: "0001_first"})
	all := reg.All()
	if all[0].Name != "0002_second" || all[1].Name != "0001_first" {
		t.Fatal("expected insertion order preserved")
	}
}
```

### Step 2: Run test (expect compile error)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... 2>&1 | grep -E "undefined|FAIL|PASS"
```

### Step 3: Create `migrate/registry.go`

**NOTE:** The global `Register()` function (called from generated `init()` functions) uses the package-level global registry. `NewRegistry()` is for tests. These must be separate.

```go
package migrate

import (
	"fmt"
	"sync"
)

// Registry stores all registered migrations, preserving insertion order.
type Registry struct {
	mu         sync.RWMutex
	migrations map[string]*Migration
	order      []string
}

// NewRegistry creates an empty Registry. Used for testing; generated code uses the global registry.
func NewRegistry() *Registry {
	return &Registry{migrations: make(map[string]*Migration)}
}

// Register adds a migration to this registry. Panics on duplicate names.
func (r *Registry) Register(m *Migration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.migrations[m.Name]; exists {
		panic(fmt.Sprintf("migration registration error: duplicate migration name %q", m.Name))
	}
	r.migrations[m.Name] = m
	r.order = append(r.order, m.Name)
}

// All returns all registered migrations in insertion order.
func (r *Registry) All() []*Migration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Migration, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.migrations[name])
	}
	return result
}

// Get returns a migration by name.
func (r *Registry) Get(name string) (*Migration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.migrations[name]
	return m, ok
}

// globalRegistry is populated by init() calls in generated migration files.
var globalRegistry = NewRegistry()

// Register adds a migration to the global registry.
// Called by each generated migration file's init() function. Panics on duplicates.
func Register(m *Migration) {
	globalRegistry.Register(m)
}

// GlobalRegistry returns the global registry. Used by app.go to build the graph.
func GlobalRegistry() *Registry {
	return globalRegistry
}
```

### Step 4: Run tests, then lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... -v 2>&1 | tail -20
golangci-lint run --no-config ./migrate/... 2>&1
```

Expected: all pass, no lint issues.

### Step 5: Commit

```bash
git add migrate/registry.go migrate/registry_test.go
git commit -m "feat(migrate): add Registry with global Register() function"
```

---

## Task 4: `migrate/graph.go`

**Files:**
- Create: `migrate/graph.go`
- Create: `migrate/graph_test.go`

### Step 1: Write failing test

```go
// migrate/graph_test.go
package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

// buildTestRegistry creates a registry with a simple linear chain for testing.
func buildLinearRegistry() *migrate.Registry {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	reg.Register(&migrate.Migration{Name: "0002_add_phone", Dependencies: []string{"0001_initial"}})
	reg.Register(&migrate.Migration{Name: "0003_add_slug", Dependencies: []string{"0002_add_phone"}})
	return reg
}

func TestGraph_Linearize_Simple(t *testing.T) {
	g, err := migrate.BuildGraph(buildLinearRegistry())
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	order, err := g.Linearize()
	if err != nil {
		t.Fatalf("Linearize: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(order))
	}
	if order[0].Name != "0001_initial" {
		t.Fatalf("expected first to be 0001_initial, got %q", order[0].Name)
	}
}

func TestGraph_Leaves_Linear(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	leaves := g.Leaves()
	if len(leaves) != 1 || leaves[0] != "0003_add_slug" {
		t.Fatalf("expected single leaf '0003_add_slug', got %v", leaves)
	}
}

func TestGraph_Roots(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "0001_initial" {
		t.Fatalf("expected single root '0001_initial', got %v", roots)
	}
}

func TestGraph_DetectBranches_Linear(t *testing.T) {
	g, _ := migrate.BuildGraph(buildLinearRegistry())
	branches := g.DetectBranches()
	if len(branches) != 0 {
		t.Fatalf("expected no branches, got %v", branches)
	}
}

func TestGraph_DetectBranches_WithBranch(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	reg.Register(&migrate.Migration{Name: "0002_base", Dependencies: []string{"0001_initial"}})
	reg.Register(&migrate.Migration{Name: "0003_feature_a", Dependencies: []string{"0002_base"}})
	reg.Register(&migrate.Migration{Name: "0003_feature_b", Dependencies: []string{"0002_base"}})

	g, _ := migrate.BuildGraph(reg)
	branches := g.DetectBranches()
	if len(branches) == 0 {
		t.Fatal("expected branches to be detected")
	}
}

func TestGraph_CycleDetection(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001", Dependencies: []string{"0002"}})
	reg.Register(&migrate.Migration{Name: "0002", Dependencies: []string{"0001"}})

	_, err := migrate.BuildGraph(reg)
	if err == nil {
		t.Fatal("expected error for cyclic dependency")
	}
}

func TestGraph_MissingDependency(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0002_add_phone", Dependencies: []string{"0001_missing"}})

	_, err := migrate.BuildGraph(reg)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestGraph_ReconstructState(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})
	reg.Register(&migrate.Migration{
		Name:         "0002_add_phone",
		Dependencies: []string{"0001_initial"},
		Operations: []migrate.Operation{
			&migrate.AddField{
				Table: "users",
				Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
			},
		},
	})

	g, err := migrate.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	state, err := g.ReconstructState()
	if err != nil {
		t.Fatalf("ReconstructState: %v", err)
	}
	users, ok := state.Tables["users"]
	if !ok {
		t.Fatal("expected 'users' table in reconstructed state")
	}
	if len(users.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(users.Fields))
	}
}
```

### Step 2: Run test (expect compile error)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... 2>&1 | grep "undefined\|cannot"
```

### Step 3: Create `migrate/graph.go`

```go
package migrate

import (
	"fmt"
	"sort"
)

// Graph is a directed acyclic graph (DAG) of migrations.
// Each node represents one migration; edges represent dependencies.
type Graph struct {
	nodes map[string]*graphNode
}

type graphNode struct {
	migration *Migration
	parents   []*graphNode // migrations this depends on
	children  []*graphNode // migrations that depend on this
}

// BuildGraph constructs a Graph from a Registry.
// Returns an error if any dependency is missing or if a cycle is detected.
func BuildGraph(reg *Registry) (*Graph, error) {
	g := &Graph{nodes: make(map[string]*graphNode)}

	// Create all nodes first
	for _, m := range reg.All() {
		g.nodes[m.Name] = &graphNode{migration: m}
	}

	// Wire edges and detect missing dependencies
	for _, node := range g.nodes {
		for _, dep := range node.migration.Dependencies {
			parent, exists := g.nodes[dep]
			if !exists {
				return nil, fmt.Errorf("migration %q depends on %q which is not registered", node.migration.Name, dep)
			}
			node.parents = append(node.parents, parent)
			parent.children = append(parent.children, node)
		}
	}

	// Detect cycles via DFS
	if err := g.detectCycles(); err != nil {
		return nil, err
	}

	return g, nil
}

// detectCycles uses DFS coloring (white=0, grey=1, black=2) to find cycles.
func (g *Graph) detectCycles() error {
	color := make(map[string]int)
	var visit func(name string) error
	visit = func(name string) error {
		color[name] = 1 // grey: in progress
		for _, child := range g.nodes[name].children {
			if color[child.migration.Name] == 1 {
				return fmt.Errorf("cycle detected involving migration %q", child.migration.Name)
			}
			if color[child.migration.Name] == 0 {
				if err := visit(child.migration.Name); err != nil {
					return err
				}
			}
		}
		color[name] = 2 // black: done
		return nil
	}
	for name := range g.nodes {
		if color[name] == 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// Linearize returns all migrations in topological order using Kahn's algorithm.
// Deterministic: nodes at the same level are sorted by name.
func (g *Graph) Linearize() ([]*Migration, error) {
	inDegree := make(map[string]int)
	for name, node := range g.nodes {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, child := range node.children {
			inDegree[child.migration.Name]++
		}
	}

	// Collect nodes with no incoming edges (roots)
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic ordering

	var result []*Migration
	for len(queue) > 0 {
		sort.Strings(queue) // sort at each step for determinism
		name := queue[0]
		queue = queue[1:]
		result = append(result, g.nodes[name].migration)
		var nextBatch []string
		for _, child := range g.nodes[name].children {
			inDegree[child.migration.Name]--
			if inDegree[child.migration.Name] == 0 {
				nextBatch = append(nextBatch, child.migration.Name)
			}
		}
		queue = append(queue, nextBatch...)
	}

	if len(result) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected during linearization (processed %d of %d nodes)", len(result), len(g.nodes))
	}
	return result, nil
}

// Roots returns names of migrations with no dependencies (no parents).
func (g *Graph) Roots() []string {
	var roots []string
	for name, node := range g.nodes {
		if len(node.parents) == 0 {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)
	return roots
}

// Leaves returns names of migrations that no other migration depends on (no children).
func (g *Graph) Leaves() []string {
	var leaves []string
	for name, node := range g.nodes {
		if len(node.children) == 0 {
			leaves = append(leaves, name)
		}
	}
	sort.Strings(leaves)
	return leaves
}

// DetectBranches returns groups of leaf migration names when there are multiple leaves
// (indicating concurrent development branches). Returns empty slice if the graph is linear.
func (g *Graph) DetectBranches() [][]string {
	leaves := g.Leaves()
	if len(leaves) <= 1 {
		return nil
	}
	return [][]string{leaves}
}

// HasBranches returns true if there are multiple leaf nodes.
func (g *Graph) HasBranches() bool {
	return len(g.Leaves()) > 1
}

// ReconstructState replays all operations in topological order to produce the
// full schema state as it would exist after all registered migrations have run.
func (g *Graph) ReconstructState() (*SchemaState, error) {
	order, err := g.Linearize()
	if err != nil {
		return nil, fmt.Errorf("linearizing graph for state reconstruction: %w", err)
	}
	state := NewSchemaState()
	for _, mig := range order {
		for _, op := range mig.Operations {
			if err := op.Mutate(state); err != nil {
				return nil, fmt.Errorf("mutating state for migration %q operation %q: %w", mig.Name, op.Describe(), err)
			}
		}
	}
	return state, nil
}

// DAGOutput is the JSON-serialisable representation of the full migration graph.
// This is what the compiled migration binary emits via the `dag --format json` command.
type DAGOutput struct {
	Migrations  []MigrationSummary `json:"migrations"`
	Roots       []string           `json:"roots"`
	Leaves      []string           `json:"leaves"`
	HasBranches bool               `json:"has_branches"`
	SchemaState *SchemaState       `json:"schema_state"`
}

// MigrationSummary is a JSON-serialisable summary of a single migration.
type MigrationSummary struct {
	Name         string             `json:"name"`
	Dependencies []string           `json:"dependencies"`
	Operations   []OperationSummary `json:"operations"`
}

// OperationSummary is a JSON-serialisable summary of a single operation.
type OperationSummary struct {
	Type        string `json:"type"`
	Table       string `json:"table,omitempty"`
	Description string `json:"description"`
}

// ToDAGOutput builds a DAGOutput from this graph, including the reconstructed schema state.
func (g *Graph) ToDAGOutput() (*DAGOutput, error) {
	order, err := g.Linearize()
	if err != nil {
		return nil, err
	}
	state, err := g.ReconstructState()
	if err != nil {
		return nil, err
	}

	out := &DAGOutput{
		Roots:       g.Roots(),
		Leaves:      g.Leaves(),
		HasBranches: g.HasBranches(),
		SchemaState: state,
	}

	for _, mig := range order {
		ms := MigrationSummary{
			Name:         mig.Name,
			Dependencies: mig.Dependencies,
		}
		if ms.Dependencies == nil {
			ms.Dependencies = []string{}
		}
		for _, op := range mig.Operations {
			ms.Operations = append(ms.Operations, OperationSummary{
				Type:        op.TypeName(),
				Table:       op.TableName(),
				Description: op.Describe(),
			})
		}
		out.Migrations = append(out.Migrations, ms)
	}
	return out, nil
}
```

### Step 4: Run tests, then lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... -v 2>&1 | tail -30
golangci-lint run --no-config ./migrate/... 2>&1
```

Expected: all pass.

### Step 5: Commit

```bash
git add migrate/graph.go migrate/graph_test.go
git commit -m "feat(migrate): add DAG graph with topological sort and state reconstruction"
```

---

## Task 5: `internal/codegen/go_generator.go`

Generates `gofmt`-compatible `.go` migration files from a `SchemaDiff`. Also generates `main.go` and `go.mod` for `init`.

**Files:**
- Create: `internal/codegen/go_generator.go`
- Create: `internal/codegen/go_generator_test.go`

**Key design:** `Change.OldValue`/`NewValue` are `interface{}` containing concrete `yaml.Table`, `yaml.Field`, or `yaml.Index` values. Use type assertions. For `field_modified` changes, pass the full `currentSchema` and `previousSchema` so the generator can look up complete field definitions.

### Step 1: Write failing tests

```go
// internal/codegen/go_generator_test.go
package codegen_test

import (
	"go/format"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/yaml"
)

func TestGoGenerator_GenerateMigration_CreateTable(t *testing.T) {
	g := codegen.NewGoGenerator()

	// Build a diff with a table_added change
	table := yaml.Table{
		Name: "users",
		Fields: []yaml.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "email", Type: "varchar", Length: 255},
		},
	}
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeTableAdded,
				TableName: "users",
				NewValue:  table,
			},
		},
	}

	src, err := g.GenerateMigration("0001_initial", []string{}, diff, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if src == "" {
		t.Fatal("expected non-empty source")
	}
	// Must be valid Go
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "0001_initial") {
		t.Error("expected migration name in output")
	}
	if !strings.Contains(src, "CreateTable") {
		t.Error("expected CreateTable in output")
	}
	if !strings.Contains(src, `"users"`) {
		t.Error("expected table name 'users' in output")
	}
}

func TestGoGenerator_GenerateMigration_AddField(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "phone",
				NewValue:  yaml.Field{Name: "phone", Type: "varchar", Length: 20},
			},
		},
	}
	src, err := g.GenerateMigration("0002_add_phone", []string{"0001_initial"}, diff, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "AddField") {
		t.Error("expected AddField in output")
	}
	if !strings.Contains(src, `"0001_initial"`) {
		t.Error("expected dependency in output")
	}
}

func TestGoGenerator_GenerateMigration_ValidGoFormat(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableRemoved, TableName: "old_table", OldValue: yaml.Table{Name: "old_table"}},
		},
	}
	src, err := g.GenerateMigration("0003_drop_table", []string{"0002_add_phone"}, diff, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
}

func TestGoGenerator_GenerateMainGo(t *testing.T) {
	g := codegen.NewGoGenerator()
	src := g.GenerateMainGo()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("main.go is not valid Go: %v", err)
	}
	if !strings.Contains(src, "func main()") {
		t.Error("expected func main() in output")
	}
	if !strings.Contains(src, "m.NewApp") {
		t.Error("expected m.NewApp in output")
	}
}

func TestGoGenerator_GenerateGoMod(t *testing.T) {
	g := codegen.NewGoGenerator()
	src := g.GenerateGoMod("myproject/migrations", "v0.3.0")
	if !strings.Contains(src, "module myproject/migrations") {
		t.Error("expected module declaration")
	}
	if !strings.Contains(src, "github.com/ocomsoft/makemigrations") {
		t.Error("expected makemigrations dependency")
	}
}
```

### Step 2: Run test (expect compile error)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... 2>&1 | head -10
```

### Step 3: Create `internal/codegen/go_generator.go`

**NOTE:** Use `go/format` to ensure output is `gofmt`-compatible. Use `fmt.Fprintf` to a `bytes.Buffer` for building source. Field type assertions: `NewValue.(yaml.Table)` for table changes, `NewValue.(yaml.Field)` for field changes, `NewValue.(yaml.Index)` for index changes.

**NOTE on field_modified:** The diff only stores the changed attribute in `OldValue`/`NewValue` (e.g. just the type string for a type change, or just the length int for a length change). To generate a complete `AlterField` operation we need the full old and new field definitions. Accept `currentSchema` and `previousSchema *yaml.Schema` parameters and look up complete field definitions from them when handling `field_modified`.

```go
// Package codegen provides code generators for the Go migration framework.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/yaml"
)

// GoGenerator generates gofmt-compatible Go migration source files from schema diffs.
type GoGenerator struct{}

// NewGoGenerator creates a new GoGenerator.
func NewGoGenerator() *GoGenerator {
	return &GoGenerator{}
}

// GenerateMigration generates the source code for a new migration .go file.
// name is the migration name (e.g. "0002_add_phone").
// deps is the list of dependency migration names.
// diff contains the schema changes to encode as operations.
// currentSchema is the new (target) schema — used for field_modified lookups.
// previousSchema is the old schema — used for field_modified lookups.
func (g *GoGenerator) GenerateMigration(name string, deps []string, diff *yaml.SchemaDiff, currentSchema, previousSchema *yaml.Schema) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("package main\n\n")
	buf.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	buf.WriteString("func init() {\n")
	fmt.Fprintf(&buf, "\tm.Register(&m.Migration{\n")
	fmt.Fprintf(&buf, "\t\tName:         %q,\n", name)

	// Dependencies
	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("%q", d)
	}
	fmt.Fprintf(&buf, "\t\tDependencies: []string{%s},\n", strings.Join(depStrs, ", "))

	// Operations
	buf.WriteString("\t\tOperations: []m.Operation{\n")
	for _, change := range diff.Changes {
		opStr, err := g.generateOperation(change, currentSchema, previousSchema)
		if err != nil {
			return "", fmt.Errorf("generating operation for %q: %w", change.Type, err)
		}
		if opStr != "" {
			buf.WriteString(opStr)
		}
	}
	buf.WriteString("\t\t},\n")
	buf.WriteString("\t})\n")
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("formatting generated code: %w\nRaw source:\n%s", err, buf.String())
	}
	return string(formatted), nil
}

// generateOperation converts a single Change into a Go operation literal string.
func (g *GoGenerator) generateOperation(change yaml.Change, currentSchema, previousSchema *yaml.Schema) (string, error) {
	switch change.Type {
	case yaml.ChangeTypeTableAdded:
		table, ok := change.NewValue.(yaml.Table)
		if !ok {
			return "", fmt.Errorf("table_added: expected yaml.Table in NewValue, got %T", change.NewValue)
		}
		return g.generateCreateTable(table), nil

	case yaml.ChangeTypeTableRemoved:
		return fmt.Sprintf("\t\t\t&m.DropTable{Name: %q},\n", change.TableName), nil

	case yaml.ChangeTypeTableRenamed:
		newName, ok := change.NewValue.(string)
		if !ok {
			return "", fmt.Errorf("table_renamed: expected string in NewValue, got %T", change.NewValue)
		}
		return fmt.Sprintf("\t\t\t&m.RenameTable{OldName: %q, NewName: %q},\n", change.TableName, newName), nil

	case yaml.ChangeTypeFieldAdded:
		field, ok := change.NewValue.(yaml.Field)
		if !ok {
			return "", fmt.Errorf("field_added: expected yaml.Field in NewValue, got %T", change.NewValue)
		}
		return g.generateAddField(change.TableName, field), nil

	case yaml.ChangeTypeFieldRemoved:
		return fmt.Sprintf("\t\t\t&m.DropField{Table: %q, Field: %q},\n", change.TableName, change.FieldName), nil

	case yaml.ChangeTypeFieldRenamed:
		newName, ok := change.NewValue.(string)
		if !ok {
			return "", fmt.Errorf("field_renamed: expected string in NewValue, got %T", change.NewValue)
		}
		return fmt.Sprintf("\t\t\t&m.RenameField{Table: %q, OldName: %q, NewName: %q},\n",
			change.TableName, change.FieldName, newName), nil

	case yaml.ChangeTypeFieldModified:
		return g.generateAlterField(change, currentSchema, previousSchema)

	case yaml.ChangeTypeIndexAdded:
		index, ok := change.NewValue.(yaml.Index)
		if !ok {
			return "", fmt.Errorf("index_added: expected yaml.Index in NewValue, got %T", change.NewValue)
		}
		return g.generateAddIndex(change.TableName, index), nil

	case yaml.ChangeTypeIndexRemoved:
		return fmt.Sprintf("\t\t\t&m.DropIndex{Table: %q, Index: %q},\n", change.TableName, change.FieldName), nil

	default:
		return "", fmt.Errorf("unknown change type: %q", change.Type)
	}
}

// generateCreateTable generates a CreateTable operation literal.
func (g *GoGenerator) generateCreateTable(table yaml.Table) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\t\t\t&m.CreateTable{\n")
	fmt.Fprintf(&buf, "\t\t\t\tName: %q,\n", table.Name)
	buf.WriteString("\t\t\t\tFields: []m.Field{\n")
	for _, f := range table.Fields {
		buf.WriteString(g.generateFieldLiteral(f))
	}
	buf.WriteString("\t\t\t\t},\n")
	if len(table.Indexes) > 0 {
		buf.WriteString("\t\t\t\tIndexes: []m.Index{\n")
		for _, idx := range table.Indexes {
			buf.WriteString(g.generateIndexLiteral(idx))
		}
		buf.WriteString("\t\t\t\t},\n")
	}
	buf.WriteString("\t\t\t},\n")
	return buf.String()
}

// generateAddField generates an AddField operation literal.
func (g *GoGenerator) generateAddField(tableName string, field yaml.Field) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\t\t\t&m.AddField{\n")
	fmt.Fprintf(&buf, "\t\t\t\tTable: %q,\n", tableName)
	fmt.Fprintf(&buf, "\t\t\t\tField: %s,\n", strings.TrimRight(g.generateFieldLiteral(field), "\n"))
	buf.WriteString("\t\t\t},\n")
	return buf.String()
}

// generateAlterField generates an AlterField operation, looking up full field defs from schemas.
func (g *GoGenerator) generateAlterField(change yaml.Change, currentSchema, previousSchema *yaml.Schema) (string, error) {
	// Find the new field definition from currentSchema
	var newField *yaml.Field
	if currentSchema != nil {
		for i := range currentSchema.Tables {
			if currentSchema.Tables[i].Name == change.TableName {
				for j := range currentSchema.Tables[i].Fields {
					if currentSchema.Tables[i].Fields[j].Name == change.FieldName {
						f := currentSchema.Tables[i].Fields[j]
						newField = &f
						break
					}
				}
			}
		}
	}
	// Find the old field definition from previousSchema
	var oldField *yaml.Field
	if previousSchema != nil {
		for i := range previousSchema.Tables {
			if previousSchema.Tables[i].Name == change.TableName {
				for j := range previousSchema.Tables[i].Fields {
					if previousSchema.Tables[i].Fields[j].Name == change.FieldName {
						f := previousSchema.Tables[i].Fields[j]
						oldField = &f
						break
					}
				}
			}
		}
	}

	// Fallback: use what we have from OldValue/NewValue for simple type/length changes
	if newField == nil {
		f := yaml.Field{Name: change.FieldName, Type: fmt.Sprintf("%v", change.NewValue)}
		newField = &f
	}
	if oldField == nil {
		f := yaml.Field{Name: change.FieldName, Type: fmt.Sprintf("%v", change.OldValue)}
		oldField = &f
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\t\t\t&m.AlterField{\n")
	fmt.Fprintf(&buf, "\t\t\t\tTable:    %q,\n", change.TableName)
	fmt.Fprintf(&buf, "\t\t\t\tOldField: %s,\n", strings.TrimRight(g.generateFieldLiteral(*oldField), "\n"))
	fmt.Fprintf(&buf, "\t\t\t\tNewField: %s,\n", strings.TrimRight(g.generateFieldLiteral(*newField), "\n"))
	buf.WriteString("\t\t\t},\n")
	return buf.String(), nil
}

// generateFieldLiteral generates a m.Field{...} struct literal string.
func (g *GoGenerator) generateFieldLiteral(f yaml.Field) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Name: %q", f.Name))
	parts = append(parts, fmt.Sprintf("Type: %q", f.Type))
	if f.PrimaryKey {
		parts = append(parts, "PrimaryKey: true")
	}
	nullable := false
	if f.Nullable != nil {
		nullable = *f.Nullable
	}
	if nullable {
		parts = append(parts, "Nullable: true")
	}
	if f.Default != "" {
		parts = append(parts, fmt.Sprintf("Default: %q", f.Default))
	}
	if f.Length > 0 {
		parts = append(parts, fmt.Sprintf("Length: %d", f.Length))
	}
	if f.Precision > 0 {
		parts = append(parts, fmt.Sprintf("Precision: %d", f.Precision))
	}
	if f.Scale > 0 {
		parts = append(parts, fmt.Sprintf("Scale: %d", f.Scale))
	}
	if f.AutoCreate {
		parts = append(parts, "AutoCreate: true")
	}
	if f.AutoUpdate {
		parts = append(parts, "AutoUpdate: true")
	}
	if f.ForeignKey != nil {
		fk := fmt.Sprintf("ForeignKey: &m.ForeignKey{Table: %q, OnDelete: %q}", f.ForeignKey.Table, f.ForeignKey.OnDelete)
		parts = append(parts, fk)
	}
	return fmt.Sprintf("\t\t\t\t\tm.Field{%s}", strings.Join(parts, ", "))
}

// generateIndexLiteral generates a m.Index{...} struct literal string.
func (g *GoGenerator) generateIndexLiteral(idx yaml.Index) string {
	fieldStrs := make([]string, len(idx.Fields))
	for i, f := range idx.Fields {
		fieldStrs[i] = fmt.Sprintf("%q", f)
	}
	unique := ""
	if idx.Unique {
		unique = ", Unique: true"
	}
	return fmt.Sprintf("\t\t\t\t\t{Name: %q, Fields: []string{%s}%s},\n",
		idx.Name, strings.Join(fieldStrs, ", "), unique)
}

// generateAddIndex generates an AddIndex operation literal.
func (g *GoGenerator) generateAddIndex(tableName string, idx yaml.Index) string {
	fieldStrs := make([]string, len(idx.Fields))
	for i, f := range idx.Fields {
		fieldStrs[i] = fmt.Sprintf("%q", f)
	}
	unique := ""
	if idx.Unique {
		unique = ", Unique: true"
	}
	return fmt.Sprintf("\t\t\t&m.AddIndex{\n\t\t\t\tTable: %q,\n\t\t\t\tIndex: m.Index{Name: %q, Fields: []string{%s}%s},\n\t\t\t},\n",
		tableName, idx.Name, strings.Join(fieldStrs, ", "), unique)
}

// GenerateMainGo generates the migrations/main.go file content.
// This file is created once during `makemigrations init` and never regenerated.
func (g *GoGenerator) GenerateMainGo() string {
	src := `package main

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
`
	return src
}

// GenerateGoMod generates the migrations/go.mod file content.
// moduleName is e.g. "myproject/migrations"; version is the makemigrations version to depend on.
func (g *GoGenerator) GenerateGoMod(moduleName, version string) string {
	return fmt.Sprintf(`module %s

go 1.24

require (
	github.com/ocomsoft/makemigrations %s
)
`, moduleName, version)
}

// MigrationFileName returns the file name for a migration given its name.
// e.g. "0001_initial" -> "0001_initial.go"
func MigrationFileName(name string) string {
	return name + ".go"
}

// NextMigrationNumber returns a zero-padded 4-digit migration number string
// given the current count of migrations.
func NextMigrationNumber(count int) string {
	return fmt.Sprintf("%04d", count+1)
}
```

### Step 4: Run tests, then lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... -v 2>&1 | tail -20
golangci-lint run --no-config ./internal/codegen/... 2>&1
```

Expected: all tests pass, no lint issues.

### Step 5: Commit

```bash
git add internal/codegen/go_generator.go internal/codegen/go_generator_test.go
git commit -m "feat(codegen): add Go migration file generator"
```

---

## Task 6: `migrate/app.go`, `migrate/dag_ascii.go`, and `migrate/config.go`

The compiled binary's CLI app. Implements `dag`, `up`, `down`, `status`, `showsql`, and `fake` commands. `dag` is implemented here; runner commands are wired in Task 9.

**Files:**
- Create: `migrate/config.go`
- Create: `migrate/app.go`
- Create: `migrate/dag_ascii.go`
- Create: `migrate/app_test.go`

### Step 1: Create `migrate/config.go`

```go
package migrate

import "os"

// Config holds database connection configuration for the migration binary.
// All fields default to environment variables if not set explicitly.
type Config struct {
	DatabaseType string // postgresql, mysql, sqlserver, sqlite
	DatabaseURL  string // full DSN — overrides individual fields if set
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
}

// EnvOr returns the value of the named environment variable,
// or defaultVal if the variable is not set or empty.
func EnvOr(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}
```

### Step 2: Create `migrate/dag_ascii.go`

```go
package migrate

import (
	"fmt"
	"strings"
)

// RenderDAGASCII produces a human-readable ASCII tree of the migration graph.
// It uses box-drawing characters to show parent→child relationships.
func RenderDAGASCII(out *DAGOutput) string {
	if out == nil || len(out.Migrations) == 0 {
		return "No migrations registered.\n"
	}

	var sb strings.Builder
	sb.WriteString("Migration Graph\n")
	sb.WriteString("===============\n\n")

	// Build a quick lookup: name -> summary
	byName := make(map[string]MigrationSummary, len(out.Migrations))
	for _, m := range out.Migrations {
		byName[m.Name] = m
	}

	// Build reverse lookup: name -> children (names that depend on this one)
	children := make(map[string][]string)
	for _, m := range out.Migrations {
		for _, dep := range m.Dependencies {
			children[dep] = append(children[dep], m.Name)
		}
	}

	// Render roots first, then recurse into children
	rendered := make(map[string]bool)
	var render func(name, prefix string, isLast bool)
	render = func(name, prefix string, isLast bool) {
		if rendered[name] {
			return
		}
		rendered[name] = true
		m := byName[name]

		connector := "├─►"
		childPrefix := prefix + "│  "
		if isLast {
			connector = "└─►"
			childPrefix = prefix + "   "
		}

		if prefix == "" {
			fmt.Fprintf(&sb, "  %s\n", name)
		} else {
			fmt.Fprintf(&sb, "%s %s %s\n", prefix, connector, name)
		}

		// Print operations
		opPrefix := childPrefix
		if prefix == "" {
			opPrefix = "  │  "
		}
		for _, op := range m.Operations {
			fmt.Fprintf(&sb, "%s%s\n", opPrefix, op.Description)
		}

		// Recurse into children
		ch := children[name]
		for i, child := range ch {
			render(child, childPrefix, i == len(ch)-1)
		}
		if len(ch) > 0 {
			fmt.Fprintf(&sb, "%s│\n", opPrefix)
		}
	}

	for _, root := range out.Roots {
		render(root, "", true)
	}

	sb.WriteString(fmt.Sprintf("\nRoots:  %s\n", strings.Join(out.Roots, ", ")))
	sb.WriteString(fmt.Sprintf("Leaves: %s\n", strings.Join(out.Leaves, ", ")))
	if out.HasBranches {
		sb.WriteString("⚠ Branches detected — run makemigrations --merge\n")
	} else {
		sb.WriteString("✓ No branches — graph is linear\n")
	}
	return sb.String()
}
```

### Step 3: Create `migrate/app.go`

This is the Cobra CLI app embedded in the generated migration binary. Implement `dag` fully; stub out `up`, `down`, `status`, `showsql`, `fake` with "not yet implemented" returns (they are implemented in Task 9).

```go
package migrate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// App is the CLI application embedded in each compiled migration binary.
type App struct {
	config Config
	root   *cobra.Command
}

// NewApp creates a new App with the given configuration.
func NewApp(cfg Config) *App {
	app := &App{config: cfg}
	app.root = app.buildRootCommand()
	return app
}

// Run executes the CLI with the given arguments.
func (a *App) Run(args []string) error {
	a.root.SetArgs(args)
	return a.root.Execute()
}

func (a *App) buildRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "makemigrations migration runner",
		Long:  "Compiled migration binary — apply, rollback, and inspect database migrations.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(a.buildDAGCommand())
	root.AddCommand(a.buildUpCommand())
	root.AddCommand(a.buildDownCommand())
	root.AddCommand(a.buildStatusCommand())
	root.AddCommand(a.buildShowSQLCommand())
	root.AddCommand(a.buildFakeCommand())

	return root
}

func (a *App) buildDAGCommand() *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Show the migration dependency graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := GlobalRegistry()
			g, err := BuildGraph(reg)
			if err != nil {
				return fmt.Errorf("building graph: %w", err)
			}
			dagOut, err := g.ToDAGOutput()
			if err != nil {
				return fmt.Errorf("building DAG output: %w", err)
			}
			if outputFormat == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(dagOut)
			}
			fmt.Print(RenderDAGASCII(dagOut))
			return nil
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", "ascii", "Output format: ascii or json")
	return cmd
}

func (a *App) buildUpCommand() *cobra.Command {
	var toMigration string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runUp(toMigration)
		},
	}
	cmd.Flags().StringVar(&toMigration, "to", "", "Apply up to this migration name")
	return cmd
}

func (a *App) buildDownCommand() *cobra.Command {
	var steps int
	var toMigration string
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runDown(steps, toMigration)
		},
	}
	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to roll back")
	cmd.Flags().StringVar(&toMigration, "to", "", "Roll back to this migration name")
	return cmd
}

func (a *App) buildStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runStatus()
		},
	}
}

func (a *App) buildShowSQLCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "showsql",
		Short: "Print SQL for pending migrations without executing",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runShowSQL()
		},
	}
}

func (a *App) buildFakeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fake [migration-name]",
		Short: "Mark a migration as applied without running its SQL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runFake(args[0])
		},
	}
}

// runUp, runDown, runStatus, runShowSQL, runFake are implemented in runner.go (Task 9).
// These stubs prevent compilation errors until then.
func (a *App) runUp(to string) error       { return fmt.Errorf("runner not yet implemented") }
func (a *App) runDown(steps int, to string) error { return fmt.Errorf("runner not yet implemented") }
func (a *App) runStatus() error            { return fmt.Errorf("runner not yet implemented") }
func (a *App) runShowSQL() error           { return fmt.Errorf("runner not yet implemented") }
func (a *App) runFake(name string) error   { return fmt.Errorf("runner not yet implemented") }
```

### Step 4: Write test for dag ASCII rendering

```go
// migrate/app_test.go
package migrate_test

import (
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestRenderDAGASCII_Linear(t *testing.T) {
	out := &migrate.DAGOutput{
		Roots:       []string{"0001_initial"},
		Leaves:      []string{"0002_add_phone"},
		HasBranches: false,
		Migrations: []migrate.MigrationSummary{
			{Name: "0001_initial", Dependencies: []string{}, Operations: []migrate.OperationSummary{
				{Type: "create_table", Table: "users", Description: "Create table users (2 fields)"},
			}},
			{Name: "0002_add_phone", Dependencies: []string{"0001_initial"}, Operations: []migrate.OperationSummary{
				{Type: "add_field", Table: "users", Description: "Add field users.phone varchar"},
			}},
		},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "0001_initial") {
		t.Error("expected 0001_initial in output")
	}
	if !strings.Contains(result, "0002_add_phone") {
		t.Error("expected 0002_add_phone in output")
	}
	if !strings.Contains(result, "No branches") {
		t.Error("expected 'No branches' message")
	}
}

func TestRenderDAGASCII_WithBranches(t *testing.T) {
	out := &migrate.DAGOutput{
		Roots:       []string{"0001_initial"},
		Leaves:      []string{"0002_feature_a", "0002_feature_b"},
		HasBranches: true,
		Migrations: []migrate.MigrationSummary{
			{Name: "0001_initial", Dependencies: []string{}},
			{Name: "0002_feature_a", Dependencies: []string{"0001_initial"}},
			{Name: "0002_feature_b", Dependencies: []string{"0001_initial"}},
		},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "Branches detected") {
		t.Error("expected 'Branches detected' warning")
	}
}

func TestRenderDAGASCII_Empty(t *testing.T) {
	result := migrate.RenderDAGASCII(nil)
	if !strings.Contains(result, "No migrations") {
		t.Error("expected 'No migrations' message for nil output")
	}
}
```

### Step 5: Run tests and lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... -v 2>&1 | tail -20
golangci-lint run --no-config ./migrate/... 2>&1
```

### Step 6: Commit

```bash
git add migrate/config.go migrate/app.go migrate/dag_ascii.go migrate/app_test.go
git commit -m "feat(migrate): add App CLI, dag command, and ASCII renderer"
```

---

## Task 7: `cmd/go_migrations.go` — the `makemigrations` CLI command

This is the main CLI entry point that wires everything together: builds the migrations binary, queries the DAG, diffs the schema, and generates the next `.go` file.

**Files:**
- Create: `cmd/go_migrations.go`
- Create: `cmd/go_migrations_test.go`

### Step 1: Create `cmd/go_migrations.go`

**Key flow:**
1. Check for `migrations/` directory
2. If `.go` files exist, build binary → query `dag --format json` → parse `DAGOutput`
3. If no `.go` files yet, start with empty schema state
4. Parse current YAML schema
5. Diff current schema against reconstructed state from DAG
6. If no changes: print "No changes" and return
7. If `--check`: return error if changes exist
8. If `--merge`: generate merge migration if branches detected
9. Generate next `.go` file
10. Write to migrations directory

```go
// Package cmd contains all CLI commands for makemigrations.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/yaml"
	"github.com/ocomsoft/makemigrations/migrate"
)

var (
	goMigDryRun    bool
	goMigCheck     bool
	goMigMerge     bool
	goMigName      string
	goMigVerbose   bool
	goMigConfigFile string
)

var goMigrationsCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Generate Go migration files from YAML schema changes",
	Long: `Compares the current YAML schema against the reconstructed state from existing
Go migration files and generates a new migration .go file for any changes detected.`,
	RunE: runGoMakeMigrations,
}

func init() {
	rootCmd.AddCommand(goMigrationsCmd)
	goMigrationsCmd.Flags().BoolVar(&goMigDryRun, "dry-run", false, "Print generated migration without writing")
	goMigrationsCmd.Flags().BoolVar(&goMigCheck, "check", false, "Exit with error if migrations are needed (for CI/CD)")
	goMigrationsCmd.Flags().BoolVar(&goMigMerge, "merge", false, "Generate merge migration for detected branches")
	goMigrationsCmd.Flags().StringVar(&goMigName, "name", "", "Custom migration name suffix")
	goMigrationsCmd.Flags().BoolVar(&goMigVerbose, "verbose", false, "Show detailed output")
	goMigrationsCmd.Flags().StringVar(&goMigConfigFile, "config", "", "Config file path")
}

func runGoMakeMigrations(cmd *cobra.Command, args []string) error {
	cfg := config.DefaultConfig()
	migrationsDir := cfg.Migration.Directory
	gen := codegen.NewGoGenerator()

	// 1. Query existing DAG (if any .go files exist)
	var dagOut *migrate.DAGOutput
	var prevSchema *yaml.Schema

	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}
	// Filter to migration files only (not main.go)
	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	if len(migFiles) > 0 {
		dagOut, err = queryDAG(migrationsDir, goMigVerbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		prevSchema = schemaStateToYAMLSchema(dagOut.SchemaState)
	}

	// 2. Parse current YAML schema
	dbType := yaml.DatabaseType(cfg.Database.Type)
	components := InitializeYAMLComponents(dbType, goMigVerbose, false)
	currentSchema, err := ScanAndParseSchemas(components, goMigVerbose)
	if err != nil {
		return fmt.Errorf("parsing YAML schema: %w", err)
	}

	// 3. Diff
	diffEngine := yaml.NewDiffEngine(goMigVerbose)
	diff, err := diffEngine.CompareSchemas(prevSchema, currentSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	// 4. Handle merge if requested
	if goMigMerge && dagOut != nil && dagOut.HasBranches {
		return generateMerge(migrationsDir, dagOut, gen, goMigDryRun, goMigVerbose)
	}

	// 5. Check for branches (warn if present and not doing merge)
	if dagOut != nil && dagOut.HasBranches && !goMigMerge {
		fmt.Printf("⚠ Branches detected: %s\n", strings.Join(dagOut.Leaves, ", "))
		fmt.Println("Run 'makemigrations makemigrations --merge' to generate a merge migration.")
	}

	if !diff.HasChanges {
		fmt.Println("No changes detected.")
		return nil
	}

	if goMigCheck {
		return fmt.Errorf("migrations needed: %d changes detected", len(diff.Changes))
	}

	// 6. Determine next migration name
	deps := []string{}
	if dagOut != nil {
		deps = dagOut.Leaves
	}
	count := len(migFiles)
	name := buildMigrationName(count, goMigName, diffEngine.GenerateMigrationName(diff))

	// 7. Generate Go source
	src, err := gen.GenerateMigration(name, deps, diff, currentSchema, prevSchema)
	if err != nil {
		return fmt.Errorf("generating migration source: %w", err)
	}

	if goMigDryRun {
		fmt.Println(src)
		return nil
	}

	// 8. Write file
	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0644); err != nil {
		return fmt.Errorf("writing migration file: %w", err)
	}
	fmt.Printf("Created %s\n", outPath)
	return nil
}

// queryDAG builds the migrations binary and runs `dag --format json` to get the current graph state.
func queryDAG(migrationsDir string, verbose bool) (*migrate.DAGOutput, error) {
	// Build binary to a temp file
	tmpBin, err := os.CreateTemp("", "migrate-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpBin.Close()
	defer os.Remove(tmpBin.Name())

	if verbose {
		fmt.Printf("Building migration binary from %s...\n", migrationsDir)
	}

	buildCmd := exec.Command("go", "build", "-o", tmpBin.Name(), ".")
	buildCmd.Dir = migrationsDir
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("building migration binary: %w\nOutput: %s", err, string(out))
	}

	// Query DAG
	dagCmd := exec.Command(tmpBin.Name(), "dag", "--format", "json")
	dagOut, err := dagCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running dag command: %w", err)
	}

	var result migrate.DAGOutput
	if err := json.Unmarshal(dagOut, &result); err != nil {
		return nil, fmt.Errorf("parsing DAG output: %w", err)
	}
	return &result, nil
}

// schemaStateToYAMLSchema converts a migrate.SchemaState to a yaml.Schema for diffing.
func schemaStateToYAMLSchema(state *migrate.SchemaState) *yaml.Schema {
	if state == nil {
		return nil
	}
	schema := &yaml.Schema{}
	for _, ts := range state.Tables {
		t := yaml.Table{Name: ts.Name}
		for _, f := range ts.Fields {
			nullable := f.Nullable
			yf := yaml.Field{
				Name:       f.Name,
				Type:       f.Type,
				PrimaryKey: f.PrimaryKey,
				Nullable:   &nullable,
				Default:    f.Default,
				Length:     f.Length,
				Precision:  f.Precision,
				Scale:      f.Scale,
				AutoCreate: f.AutoCreate,
				AutoUpdate: f.AutoUpdate,
			}
			if f.ForeignKey != nil {
				yf.ForeignKey = &types.ForeignKey{
					Table:    f.ForeignKey.Table,
					OnDelete: f.ForeignKey.OnDelete,
				}
			}
			t.Fields = append(t.Fields, yf)
		}
		for _, idx := range ts.Indexes {
			t.Indexes = append(t.Indexes, yaml.Index{Name: idx.Name, Fields: idx.Fields, Unique: idx.Unique})
		}
		schema.Tables = append(schema.Tables, t)
	}
	// Sort tables for determinism
	sort.Slice(schema.Tables, func(i, j int) bool {
		return schema.Tables[i].Name < schema.Tables[j].Name
	})
	return schema
}

// buildMigrationName builds the migration name from sequence number and optional custom name.
// Format: 0002_add_user_phone or 0002_20060102150405
func buildMigrationName(currentCount int, customName, autoName string) string {
	num := codegen.NextMigrationNumber(currentCount)
	if customName != "" {
		return fmt.Sprintf("%s_%s", num, strings.ToLower(strings.ReplaceAll(customName, " ", "_")))
	}
	if autoName != "" {
		return fmt.Sprintf("%s_%s", num, autoName)
	}
	return fmt.Sprintf("%s_%s", num, time.Now().Format("20060102150405"))
}

// generateMerge generates a merge migration for detected branches.
func generateMerge(migrationsDir string, dagOut *migrate.DAGOutput, gen *codegen.GoGenerator, dryRun, verbose bool) error {
	mergeGen := codegen.NewMergeGenerator()
	count := 0 // will be calculated from existing files
	goFiles, _ := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			count++
		}
	}

	name := fmt.Sprintf("%s_merge_%s", codegen.NextMigrationNumber(count),
		strings.Join(dagOut.Leaves, "_and_"))
	// truncate if too long
	if len(name) > 80 {
		name = fmt.Sprintf("%s_merge", codegen.NextMigrationNumber(count))
	}

	src, err := mergeGen.GenerateMerge(name, dagOut.Leaves)
	if err != nil {
		return fmt.Errorf("generating merge migration: %w", err)
	}

	if dryRun {
		fmt.Println(src)
		return nil
	}

	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0644); err != nil {
		return fmt.Errorf("writing merge migration: %w", err)
	}
	fmt.Printf("Created merge migration: %s\n", outPath)
	fmt.Printf("Dependencies: %s\n", strings.Join(dagOut.Leaves, ", "))
	return nil
}
```

### Step 2: Create `cmd/go_migrations_test.go`

```go
package cmd_test

import (
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/cmd"
)

func TestBuildMigrationName_WithCustomName(t *testing.T) {
	name := cmd.BuildMigrationName(0, "add user phone", "")
	if !strings.HasPrefix(name, "0001_") {
		t.Fatalf("expected prefix '0001_', got %q", name)
	}
	if !strings.Contains(name, "add_user_phone") {
		t.Fatalf("expected 'add_user_phone' in name, got %q", name)
	}
}

func TestBuildMigrationName_WithAutoName(t *testing.T) {
	name := cmd.BuildMigrationName(2, "", "add_email")
	if !strings.HasPrefix(name, "0003_") {
		t.Fatalf("expected prefix '0003_', got %q", name)
	}
}
```

**NOTE:** Export `buildMigrationName` as `BuildMigrationName` for testing, or move to a separate testable package. Make it an exported function in `cmd/go_migrations.go`.

### Step 3: Run build check

```bash
cd /workspaces/ocom/go/makemigrations && go build ./... 2>&1
```

Expected: compiles (MergeGenerator will be a stub until Task 8).

### Step 4: Lint

```bash
golangci-lint run --no-config ./cmd/... 2>&1
```

### Step 5: Commit

```bash
git add cmd/go_migrations.go cmd/go_migrations_test.go
git commit -m "feat(cmd): add makemigrations command with binary-query loop"
```

---

## Task 8: Branch Detection & Merge Migrations

**Files:**
- Create: `internal/codegen/merge_generator.go`
- Create: `internal/codegen/merge_generator_test.go`

### Step 1: Write failing test

```go
// internal/codegen/merge_generator_test.go
package codegen_test

import (
	"go/format"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/codegen"
)

func TestMergeGenerator_GenerateMerge(t *testing.T) {
	g := codegen.NewMergeGenerator()
	src, err := g.GenerateMerge("0004_merge_feature_a_and_b",
		[]string{"0003_feature_a", "0003_feature_b"})
	if err != nil {
		t.Fatalf("GenerateMerge: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "0004_merge_feature_a_and_b") {
		t.Error("expected migration name in output")
	}
	if !strings.Contains(src, "0003_feature_a") {
		t.Error("expected first dependency in output")
	}
	if !strings.Contains(src, "0003_feature_b") {
		t.Error("expected second dependency in output")
	}
	// Merge migrations have no operations
	if strings.Contains(src, "CreateTable") || strings.Contains(src, "AddField") {
		t.Error("merge migration should have no operations")
	}
}
```

### Step 2: Run test (expect compile error)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... 2>&1 | grep "undefined\|FAIL\|PASS"
```

### Step 3: Create `internal/codegen/merge_generator.go`

```go
// Package codegen provides code generators for the Go migration framework.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
)

// MergeGenerator generates merge migration .go files.
// Merge migrations have no operations — they exist only to establish a
// common ancestor for two divergent branches of the migration DAG.
type MergeGenerator struct{}

// NewMergeGenerator creates a new MergeGenerator.
func NewMergeGenerator() *MergeGenerator {
	return &MergeGenerator{}
}

// GenerateMerge generates the source code for a merge migration .go file.
// name is the migration name (e.g. "0004_merge_feature_a_and_b").
// deps is the list of branch leaf names to merge.
func (g *MergeGenerator) GenerateMerge(name string, deps []string) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("package main\n\n")
	buf.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	buf.WriteString("func init() {\n")
	fmt.Fprintf(&buf, "\tm.Register(&m.Migration{\n")
	fmt.Fprintf(&buf, "\t\tName:         %q,\n", name)

	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("%q", d)
	}
	fmt.Fprintf(&buf, "\t\tDependencies: []string{%s},\n", strings.Join(depStrs, ", "))
	buf.WriteString("\t\tOperations:   []m.Operation{},\n")
	buf.WriteString("\t})\n")
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("formatting merge migration: %w\nRaw:\n%s", err, buf.String())
	}
	return string(formatted), nil
}
```

### Step 4: Run tests, then lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... -v 2>&1 | tail -20
golangci-lint run --no-config ./internal/codegen/... 2>&1
```

### Step 5: Commit

```bash
git add internal/codegen/merge_generator.go internal/codegen/merge_generator_test.go
git commit -m "feat(codegen): add merge migration generator"
```

---

## Task 9: `migrate/recorder.go` and `migrate/runner.go`

Implements the database interaction: `makemigrations_history` table management and migration execution.

**Files:**
- Create: `migrate/recorder.go`
- Create: `migrate/runner.go`
- Create: `migrate/runner_test.go`
- Modify: `migrate/app.go` — replace stub `runUp`/`runDown`/`runStatus`/`runShowSQL`/`runFake` with real implementations

### Step 1: Create `migrate/recorder.go`

```go
package migrate

import (
	"database/sql"
	"fmt"
)

const createHistoryTableSQL = `CREATE TABLE IF NOT EXISTS makemigrations_history (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)`

// MigrationRecorder manages the makemigrations_history table.
// It records which migrations have been applied to the database.
type MigrationRecorder struct {
	db *sql.DB
}

// NewMigrationRecorder creates a new MigrationRecorder using the given db connection.
func NewMigrationRecorder(db *sql.DB) *MigrationRecorder {
	return &MigrationRecorder{db: db}
}

// EnsureTable creates the makemigrations_history table if it does not exist.
func (r *MigrationRecorder) EnsureTable() error {
	_, err := r.db.Exec(createHistoryTableSQL)
	if err != nil {
		return fmt.Errorf("creating makemigrations_history table: %w", err)
	}
	return nil
}

// GetApplied returns a set of migration names that have been applied.
func (r *MigrationRecorder) GetApplied() (map[string]bool, error) {
	rows, err := r.db.Query("SELECT name FROM makemigrations_history")
	if err != nil {
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning migration name: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// RecordApplied inserts a migration name into the history table.
func (r *MigrationRecorder) RecordApplied(name string) error {
	_, err := r.db.Exec("INSERT INTO makemigrations_history (name) VALUES ($1)", name)
	if err != nil {
		return fmt.Errorf("recording migration %q as applied: %w", name, err)
	}
	return nil
}

// RecordRolledBack removes a migration name from the history table.
func (r *MigrationRecorder) RecordRolledBack(name string) error {
	_, err := r.db.Exec("DELETE FROM makemigrations_history WHERE name = $1", name)
	if err != nil {
		return fmt.Errorf("recording migration %q as rolled back: %w", name, err)
	}
	return nil
}

// Fake inserts a migration name without executing any SQL.
// Used to mark migrations as applied when the database already has the schema.
func (r *MigrationRecorder) Fake(name string) error {
	return r.RecordApplied(name)
}
```

### Step 2: Create `migrate/runner.go`

```go
package migrate

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers"
)

// Runner executes migrations against a database in topological order.
type Runner struct {
	graph    *Graph
	provider providers.Provider
	db       *sql.DB
	recorder *MigrationRecorder
}

// NewRunner creates a Runner using the given graph, provider, db, and recorder.
func NewRunner(graph *Graph, provider providers.Provider, db *sql.DB, recorder *MigrationRecorder) *Runner {
	return &Runner{
		graph:    graph,
		provider: provider,
		db:       db,
		recorder: recorder,
	}
}

// Up applies all pending migrations in topological order.
// If to is non-empty, stops after applying the named migration.
func (r *Runner) Up(to string) error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return fmt.Errorf("linearizing graph: %w", err)
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}
	state := NewSchemaState()

	// Replay already-applied migrations to build state
	for _, mig := range plan {
		if !applied[mig.Name] {
			break
		}
		for _, op := range mig.Operations {
			if err := op.Mutate(state); err != nil {
				return fmt.Errorf("replaying state for %q: %w", mig.Name, err)
			}
		}
	}

	for _, mig := range plan {
		if applied[mig.Name] {
			continue
		}
		fmt.Printf("Applying %s...", mig.Name)
		if err := r.applyMigration(mig, state); err != nil {
			fmt.Println(" FAILED")
			return fmt.Errorf("applying migration %q: %w", mig.Name, err)
		}
		fmt.Println(" done")
		if to != "" && mig.Name == to {
			break
		}
	}
	return nil
}

// Down rolls back migrations. If steps > 0, rolls back that many. If to is set, rolls back to that name.
func (r *Runner) Down(steps int, to string) error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return fmt.Errorf("linearizing graph: %w", err)
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Collect applied migrations in reverse order
	var toRollback []*Migration
	for i := len(plan) - 1; i >= 0; i-- {
		if applied[plan[i].Name] {
			toRollback = append(toRollback, plan[i])
		}
	}

	// Build state up to each rollback point by replaying from the start
	for i, mig := range toRollback {
		if steps > 0 && i >= steps {
			break
		}
		if to != "" && mig.Name == to {
			break
		}
		// Reconstruct state just before this migration
		state := NewSchemaState()
		for _, m := range plan {
			if m.Name == mig.Name {
				break
			}
			if applied[m.Name] {
				for _, op := range m.Operations {
					_ = op.Mutate(state)
				}
			}
		}
		fmt.Printf("Rolling back %s...", mig.Name)
		if err := r.rollbackMigration(mig, state); err != nil {
			fmt.Println(" FAILED")
			return fmt.Errorf("rolling back migration %q: %w", mig.Name, err)
		}
		fmt.Println(" done")
	}
	return nil
}

// Status prints migration status: applied vs pending.
func (r *Runner) Status() error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return err
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return err
	}
	fmt.Printf("%-50s %s\n", "Migration", "Status")
	fmt.Println(strings.Repeat("-", 60))
	for _, mig := range plan {
		status := "Pending"
		if applied[mig.Name] {
			status = "Applied"
		}
		fmt.Printf("%-50s %s\n", mig.Name, status)
	}
	return nil
}

// ShowSQL prints all pending migration SQL without executing it.
func (r *Runner) ShowSQL() error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return err
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return err
	}
	state := NewSchemaState()
	for _, mig := range plan {
		if applied[mig.Name] {
			for _, op := range mig.Operations {
				_ = op.Mutate(state)
			}
			continue
		}
		fmt.Printf("-- %s\n", mig.Name)
		for _, op := range mig.Operations {
			sqlStr, err := op.Up(r.provider, state, nil)
			if err != nil {
				return fmt.Errorf("generating SQL for %q: %w", mig.Name, err)
			}
			if sqlStr != "" {
				fmt.Println(sqlStr)
				fmt.Println()
			}
			_ = op.Mutate(state)
		}
	}
	return nil
}

// applyMigration executes all operations in a migration and records it as applied.
func (r *Runner) applyMigration(mig *Migration, state *SchemaState) error {
	for _, op := range mig.Operations {
		sqlStr, err := op.Up(r.provider, state, nil)
		if err != nil {
			return fmt.Errorf("generating SQL for operation %q: %w", op.Describe(), err)
		}
		if sqlStr != "" {
			if _, err := r.db.Exec(sqlStr); err != nil {
				return fmt.Errorf("executing SQL %q: %w", sqlStr, err)
			}
		}
		if err := op.Mutate(state); err != nil {
			return fmt.Errorf("mutating state: %w", err)
		}
	}
	return r.recorder.RecordApplied(mig.Name)
}

// rollbackMigration reverses all operations in a migration and removes it from history.
func (r *Runner) rollbackMigration(mig *Migration, state *SchemaState) error {
	// Roll back in reverse operation order
	for i := len(mig.Operations) - 1; i >= 0; i-- {
		op := mig.Operations[i]
		sqlStr, err := op.Down(r.provider, state, nil)
		if err != nil {
			return fmt.Errorf("generating down SQL for %q: %w", op.Describe(), err)
		}
		if sqlStr != "" {
			if _, err := r.db.Exec(sqlStr); err != nil {
				return fmt.Errorf("executing down SQL %q: %w", sqlStr, err)
			}
		}
	}
	return r.recorder.RecordRolledBack(mig.Name)
}
```

### Step 3: Update `migrate/app.go` — replace stubs with real runner calls

Replace the `runUp`, `runDown`, `runStatus`, `runShowSQL`, `runFake` stub methods in `app.go` with real implementations. Add `openDB()` and `buildProvider()` helpers.

Find and replace the stub implementations at the bottom of `migrate/app.go`:

```go
// openDB creates a *sql.DB from the app config.
func (a *App) openDB() (*sql.DB, error) {
	// Import is needed: "database/sql" and database drivers.
	// Drivers are registered by importing their packages in main.go.
	// Use DATABASE_URL if set, otherwise build DSN from individual fields.
	dsn := a.config.DatabaseURL
	if dsn == "" {
		dsn = buildDSN(a.config)
	}
	driver := driverName(a.config.DatabaseType)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return db, nil
}

// buildDSN constructs a DSN from individual config fields.
func buildDSN(cfg Config) string {
	switch cfg.DatabaseType {
	case "postgresql", "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
	case "sqlite":
		return cfg.DBName
	default:
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	}
}

// driverName maps DatabaseType to SQL driver name.
func driverName(dbType string) string {
	switch dbType {
	case "mysql", "tidb":
		return "mysql"
	case "sqlserver":
		return "sqlserver"
	case "sqlite":
		return "sqlite3"
	default:
		return "postgres"
	}
}

func (a *App) buildRunner() (*Runner, error) {
	reg := GlobalRegistry()
	g, err := BuildGraph(reg)
	if err != nil {
		return nil, fmt.Errorf("building graph: %w", err)
	}
	db, err := a.openDB()
	if err != nil {
		return nil, err
	}
	recorder := NewMigrationRecorder(db)
	if err := recorder.EnsureTable(); err != nil {
		return nil, err
	}
	// Provider is constructed via the internal factory
	p, err := buildProviderFromType(a.config.DatabaseType)
	if err != nil {
		return nil, err
	}
	return NewRunner(g, p, db, recorder), nil
}

func (a *App) runUp(to string) error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.Up(to)
}

func (a *App) runDown(steps int, to string) error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.Down(steps, to)
}

func (a *App) runStatus() error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.Status()
}

func (a *App) runShowSQL() error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.ShowSQL()
}

func (a *App) runFake(name string) error {
	db, err := a.openDB()
	if err != nil {
		return err
	}
	recorder := NewMigrationRecorder(db)
	if err := recorder.EnsureTable(); err != nil {
		return err
	}
	if err := recorder.Fake(name); err != nil {
		return err
	}
	fmt.Printf("Marked %q as applied (faked).\n", name)
	return nil
}
```

Also add `migrate/provider_bridge.go` — thin wrapper that uses the existing provider factory:

```go
// migrate/provider_bridge.go
package migrate

import (
	"fmt"

	"github.com/ocomsoft/makemigrations/internal/providers"
	"github.com/ocomsoft/makemigrations/internal/providers/factory"
	"github.com/ocomsoft/makemigrations/internal/types"
)

// buildProviderFromType creates a Provider from a database type string.
func buildProviderFromType(dbType string) (providers.Provider, error) {
	p, err := factory.NewProvider(types.DatabaseType(dbType))
	if err != nil {
		return nil, fmt.Errorf("creating provider for %q: %w", dbType, err)
	}
	return p, nil
}
```

**NOTE:** Check `internal/providers/factory.go` for the actual factory function signature. Adjust `buildProviderFromType` to match.

### Step 4: Write runner integration test using SQLite

```go
// migrate/runner_test.go
package migrate_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/migrate"
)

func TestRunner_UpDown_SQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("opening SQLite: %v", err)
	}
	defer db.Close()

	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255, Nullable: false},
				},
			},
		},
	})

	g, err := migrate.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	recorder := migrate.NewMigrationRecorder(db)
	if err := recorder.EnsureTable(); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}

	p := sqlite.NewProvider()
	runner := migrate.NewRunner(g, p, db, recorder)

	// Apply
	if err := runner.Up(""); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Verify table exists
	if _, err := db.Exec("INSERT INTO users (email) VALUES ('test@example.com')"); err != nil {
		t.Fatalf("expected users table to exist after Up: %v", err)
	}

	// Verify recorded
	applied, _ := recorder.GetApplied()
	if !applied["0001_initial"] {
		t.Fatal("expected 0001_initial to be recorded as applied")
	}

	// Roll back
	if err := runner.Down(1, ""); err != nil {
		t.Fatalf("Down: %v", err)
	}

	// Verify table gone
	if _, err := db.Exec("SELECT 1 FROM users"); err == nil {
		t.Fatal("expected users table to be dropped after Down")
	}

	// Verify unrecorded
	applied, _ = recorder.GetApplied()
	if applied["0001_initial"] {
		t.Fatal("expected 0001_initial to be removed from history after Down")
	}
}

func TestRecorder_Fake(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("opening SQLite: %v", err)
	}
	defer db.Close()

	recorder := migrate.NewMigrationRecorder(db)
	_ = recorder.EnsureTable()

	if err := recorder.Fake("0001_initial"); err != nil {
		t.Fatalf("Fake: %v", err)
	}

	applied, _ := recorder.GetApplied()
	if !applied["0001_initial"] {
		t.Fatal("expected 0001_initial in history after Fake")
	}
}
```

**NOTE:** SQLite's `SERIAL` is not supported — for the SQLite recorder test, use `INTEGER PRIMARY KEY` in the history table creation. Check if `createHistoryTableSQL` needs a SQLite variant in `recorder.go`. If so, make it configurable or use `CREATE TABLE IF NOT EXISTS makemigrations_history (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)` for SQLite.

The simplest fix: use `CREATE TABLE IF NOT EXISTS makemigrations_history (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)` as the base SQL (works for SQLite). For PostgreSQL, `INTEGER` maps to `int4` which is fine. If a provider-specific history table is needed later, that can be added.

### Step 5: Run tests

```bash
cd /workspaces/ocom/go/makemigrations && go test ./migrate/... -v -run TestRunner 2>&1
go test ./migrate/... -v -run TestRecorder 2>&1
```

### Step 6: Lint

```bash
golangci-lint run --no-config ./migrate/... 2>&1
```

### Step 7: Commit

```bash
git add migrate/recorder.go migrate/runner.go migrate/runner_test.go migrate/provider_bridge.go
git add migrate/app.go  # updated stubs
git commit -m "feat(migrate): add MigrationRecorder, Runner, and update App to wire real runner"
```

---

## Task 10: `internal/codegen/squash_generator.go`

Collapses a range of migrations into a single squashed migration with a `Replaces` field.

**Files:**
- Create: `internal/codegen/squash_generator.go`
- Create: `internal/codegen/squash_generator_test.go`

### Step 1: Write failing test

```go
// internal/codegen/squash_generator_test.go
package codegen_test

import (
	"go/format"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/migrate"
)

func TestSquashGenerator_GenerateSquash(t *testing.T) {
	migrations := []*migrate.Migration{
		{
			Name:         "0001_initial",
			Dependencies: []string{},
			Operations: []migrate.Operation{
				&migrate.CreateTable{
					Name:   "users",
					Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
				},
			},
		},
		{
			Name:         "0002_add_phone",
			Dependencies: []string{"0001_initial"},
			Operations: []migrate.Operation{
				&migrate.AddField{
					Table: "users",
					Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
				},
			},
		},
	}

	g := codegen.NewSquashGenerator()
	src, err := g.GenerateSquash("0001_squashed_0002", []string{"0001_initial", "0002_add_phone"}, migrations)
	if err != nil {
		t.Fatalf("GenerateSquash: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "0001_squashed_0002") {
		t.Error("expected squash name in output")
	}
	if !strings.Contains(src, "Replaces") {
		t.Error("expected Replaces field in output")
	}
	if !strings.Contains(src, "0001_initial") {
		t.Error("expected replaced migration in Replaces list")
	}
	if !strings.Contains(src, "CreateTable") {
		t.Error("expected CreateTable operation in squashed output")
	}
}
```

### Step 2: Run test (expect compile error)

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... 2>&1 | grep "undefined\|FAIL\|PASS"
```

### Step 3: Create `internal/codegen/squash_generator.go`

The squash generator uses `GoGenerator` internally to render individual operations, then wraps them in a migration with a `Replaces` list.

```go
// Package codegen provides code generators for the Go migration framework.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"

	"github.com/ocomsoft/makemigrations/migrate"
)

// SquashGenerator generates squashed migration .go files.
// A squashed migration combines multiple migrations into one, listing the originals
// in its Replaces field so the runner can skip them if already applied.
type SquashGenerator struct {
	gen *GoGenerator
}

// NewSquashGenerator creates a new SquashGenerator.
func NewSquashGenerator() *SquashGenerator {
	return &SquashGenerator{gen: NewGoGenerator()}
}

// GenerateSquash generates the source code for a squashed migration .go file.
// name is the new squashed migration name.
// replaces is the ordered list of migration names being replaced.
// migrations is the ordered list of Migration objects to squash.
func (g *SquashGenerator) GenerateSquash(name string, replaces []string, migrations []*migrate.Migration) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("package main\n\n")
	buf.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	buf.WriteString("func init() {\n")
	fmt.Fprintf(&buf, "\tm.Register(&m.Migration{\n")
	fmt.Fprintf(&buf, "\t\tName:         %q,\n", name)
	buf.WriteString("\t\tDependencies: []string{},\n")

	// Replaces field
	replaceStrs := make([]string, len(replaces))
	for i, r := range replaces {
		replaceStrs[i] = fmt.Sprintf("%q", r)
	}
	fmt.Fprintf(&buf, "\t\tReplaces:     []string{%s},\n", strings.Join(replaceStrs, ", "))

	// Combine all operations from all migrations
	buf.WriteString("\t\tOperations: []m.Operation{\n")
	for _, mig := range migrations {
		for _, op := range mig.Operations {
			opStr, err := g.renderOperation(op)
			if err != nil {
				return "", fmt.Errorf("rendering operation from %q: %w", mig.Name, err)
			}
			if opStr != "" {
				buf.WriteString(opStr)
			}
		}
	}
	buf.WriteString("\t\t},\n")
	buf.WriteString("\t})\n")
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("formatting squash migration: %w\nRaw:\n%s", err, buf.String())
	}
	return string(formatted), nil
}

// renderOperation converts a migrate.Operation back to Go source literal.
// This is the reverse of what GoGenerator.generateOperation does.
func (g *SquashGenerator) renderOperation(op migrate.Operation) (string, error) {
	switch o := op.(type) {
	case *migrate.CreateTable:
		return g.gen.generateCreateTable(migrate2yamlTable(o)), nil
	case *migrate.DropTable:
		return fmt.Sprintf("\t\t\t&m.DropTable{Name: %q},\n", o.Name), nil
	case *migrate.RenameTable:
		return fmt.Sprintf("\t\t\t&m.RenameTable{OldName: %q, NewName: %q},\n", o.OldName, o.NewName), nil
	case *migrate.AddField:
		return g.gen.generateAddField(o.Table, migrate2yamlField(o.Field)), nil
	case *migrate.DropField:
		return fmt.Sprintf("\t\t\t&m.DropField{Table: %q, Field: %q},\n", o.Table, o.Field), nil
	case *migrate.RenameField:
		return fmt.Sprintf("\t\t\t&m.RenameField{Table: %q, OldName: %q, NewName: %q},\n", o.Table, o.OldName, o.NewName), nil
	case *migrate.AlterField:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "\t\t\t&m.AlterField{\n")
		fmt.Fprintf(&buf, "\t\t\t\tTable:    %q,\n", o.Table)
		fmt.Fprintf(&buf, "\t\t\t\tOldField: %s,\n", strings.TrimRight(g.gen.generateFieldLiteral(migrate2yamlField(o.OldField)), "\n"))
		fmt.Fprintf(&buf, "\t\t\t\tNewField: %s,\n", strings.TrimRight(g.gen.generateFieldLiteral(migrate2yamlField(o.NewField)), "\n"))
		buf.WriteString("\t\t\t},\n")
		return buf.String(), nil
	case *migrate.AddIndex:
		return g.gen.generateAddIndex(o.Table, migrate2yamlIndex(o.Index)), nil
	case *migrate.DropIndex:
		return fmt.Sprintf("\t\t\t&m.DropIndex{Table: %q, Index: %q},\n", o.Table, o.Index), nil
	case *migrate.RunSQL:
		return fmt.Sprintf("\t\t\t&m.RunSQL{ForwardSQL: %q, BackwardSQL: %q},\n", o.ForwardSQL, o.BackwardSQL), nil
	default:
		return "", fmt.Errorf("unknown operation type %T", op)
	}
}
```

Also add conversion helpers in `internal/codegen/squash_generator.go` (or a shared `codegen/convert.go`):

```go
// migrate2yamlField converts a migrate.Field to a yaml.Field for reuse in generators.
// Add these at the bottom of squash_generator.go:

import "github.com/ocomsoft/makemigrations/internal/yaml"

func migrate2yamlField(f migrate.Field) yaml.Field {
	nullable := f.Nullable
	yf := yaml.Field{
		Name:       f.Name,
		Type:       f.Type,
		PrimaryKey: f.PrimaryKey,
		Nullable:   &nullable,
		Default:    f.Default,
		Length:     f.Length,
		Precision:  f.Precision,
		Scale:      f.Scale,
		AutoCreate: f.AutoCreate,
		AutoUpdate: f.AutoUpdate,
	}
	if f.ForeignKey != nil {
		yf.ForeignKey = &yaml.ForeignKey{Table: f.ForeignKey.Table, OnDelete: f.ForeignKey.OnDelete}
	}
	return yf
}

func migrate2yamlTable(op *migrate.CreateTable) yaml.Table {
	t := yaml.Table{Name: op.Name}
	for _, f := range op.Fields {
		t.Fields = append(t.Fields, migrate2yamlField(f))
	}
	for _, idx := range op.Indexes {
		t.Indexes = append(t.Indexes, migrate2yamlIndex(idx))
	}
	return t
}

func migrate2yamlIndex(idx migrate.Index) yaml.Index {
	return yaml.Index{Name: idx.Name, Fields: idx.Fields, Unique: idx.Unique}
}
```

**NOTE:** `generateCreateTable`, `generateAddField`, `generateAddIndex`, `generateFieldLiteral`, `generateIndexLiteral` must be exported (capital letter) OR the squash generator must be in the same package as `go_generator.go`. Since both are in `package codegen`, they can stay unexported and be called directly — but the squash generator needs access to the `GoGenerator`'s unexported methods. The simplest solution: make the squash generator a method set on `GoGenerator`, or add the conversion logic inline. Check if this compiles; if not, make the helper methods exported.

### Step 4: Run tests, lint

```bash
cd /workspaces/ocom/go/makemigrations && go test ./internal/codegen/... -v 2>&1 | tail -20
golangci-lint run --no-config ./internal/codegen/... 2>&1
```

### Step 5: Commit

```bash
git add internal/codegen/squash_generator.go internal/codegen/squash_generator_test.go
git commit -m "feat(codegen): add squash migration generator"
```

---

## Task 11: Backward Compatibility — Rename `sql-migrations` + Extend `init`

**Files:**
- Rename: `cmd/makemigrations.go` → `cmd/sql_migrations.go` (command name: `sql-migrations`)
- Modify: `cmd/init.go` — detect `.schema_snapshot.yaml` and generate initial Go migration

### Step 1: Rename the SQL command

In `cmd/makemigrations.go`, change the command name from `"makemigrations"` to `"sql-migrations"` and rename the file to `cmd/sql_migrations.go`.

```bash
mv /workspaces/ocom/go/makemigrations/cmd/makemigrations.go \
   /workspaces/ocom/go/makemigrations/cmd/sql_migrations.go
```

Then edit `cmd/sql_migrations.go`: change `Use: "makemigrations"` to `Use: "sql-migrations"` and update the `Short`/`Long` description to note it is the legacy SQL workflow.

### Step 2: Verify no command name collision

Both `makemigrations` (the new Go workflow from Task 7) and `sql-migrations` (legacy) are now registered. Verify they don't collide:

```bash
cd /workspaces/ocom/go/makemigrations && go build ./... && ./makemigrations --help 2>&1 | grep -E "makemigrations|sql-migrations"
```

Expected: both commands appear in help output.

### Step 3: Extend `cmd/init.go` for Go migration bootstrap

Add a new function `ExecuteGoMigrationInit` to `cmd/yaml_common.go` (or a new file `cmd/go_init.go`) that:
1. Checks for `migrations/.schema_snapshot.yaml`
2. If found: loads it, generates `0001_initial.go` from it, generates `main.go`, generates `go.mod`
3. Prints fake instructions
4. If not found: generates empty `main.go` and `go.mod` only

Add a `--go` flag to the `init` command to trigger the Go migration setup:

```go
// cmd/go_init.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/yaml"
)

// ExecuteGoMigrationInit initializes the migrations/ directory for the Go migration framework.
// If a .schema_snapshot.yaml exists, generates an initial migration from it.
func ExecuteGoMigrationInit(databaseType string, verbose bool) error {
	cfg := config.DefaultConfig()
	migrationsDir := cfg.Migration.Directory
	gen := codegen.NewGoGenerator()

	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}

	// Detect existing snapshot
	snapshotPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml")
	sm := yaml.NewStateManager(verbose)
	existingSchema, err := sm.LoadSchemaSnapshot(migrationsDir)

	var initialMigName string

	if err == nil && existingSchema != nil {
		// Generate initial migration from snapshot
		initialMigName = "0001_initial"
		diff := schemaToInitialDiff(existingSchema)
		src, err := gen.GenerateMigration(initialMigName, []string{}, diff, existingSchema, nil)
		if err != nil {
			return fmt.Errorf("generating initial migration: %w", err)
		}
		migPath := filepath.Join(migrationsDir, codegen.MigrationFileName(initialMigName))
		if err := os.WriteFile(migPath, []byte(src), 0644); err != nil {
			return fmt.Errorf("writing initial migration: %w", err)
		}
		fmt.Printf("Created %s (from existing schema snapshot)\n", migPath)
		_ = snapshotPath // snapshot path noted for user
	}

	// Generate main.go (only if it doesn't exist)
	mainPath := filepath.Join(migrationsDir, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		if err := os.WriteFile(mainPath, []byte(gen.GenerateMainGo()), 0644); err != nil {
			return fmt.Errorf("writing main.go: %w", err)
		}
		fmt.Printf("Created %s\n", mainPath)
	}

	// Determine module name from go.mod
	moduleName := readModuleName() + "/migrations"

	// Generate go.mod (only if it doesn't exist)
	goModPath := filepath.Join(migrationsDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		version := "v0.3.0" // TODO: use current version from build info
		if err := os.WriteFile(goModPath, []byte(gen.GenerateGoMod(moduleName, version)), 0644); err != nil {
			return fmt.Errorf("writing go.mod: %w", err)
		}
		fmt.Printf("Created %s\n", goModPath)
	}

	// Print fake instructions if we generated an initial migration
	if initialMigName != "" {
		fmt.Printf(`
Your database already has these tables applied. Mark this migration as applied without re-running SQL:

  cd %s && go mod tidy && go build -o migrate .
  ./migrate fake %s

`, migrationsDir, initialMigName)
	} else {
		fmt.Printf(`
Initialization complete. No existing schema found.

To generate your first migration:
  makemigrations makemigrations --name "initial"

Then build and run:
  cd %s && go mod tidy && go build -o migrate .
  ./migrate up
`, migrationsDir)
	}

	return nil
}

// schemaToInitialDiff converts a Schema to a SchemaDiff treating all tables as added.
func schemaToInitialDiff(schema *yaml.Schema) *yaml.SchemaDiff {
	diff := &yaml.SchemaDiff{HasChanges: true}
	for _, t := range schema.Tables {
		diff.Changes = append(diff.Changes, yaml.Change{
			Type:      yaml.ChangeTypeTableAdded,
			TableName: t.Name,
			NewValue:  t,
		})
	}
	return diff
}

// readModuleName reads the module name from go.mod in the current directory.
func readModuleName() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "myproject"
	}
	for _, line := range splitLines(string(data)) {
		if len(line) > 7 && line[:7] == "module " {
			return strings.TrimSpace(line[7:])
		}
	}
	return "myproject"
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}
```

Then wire it into `cmd/init.go`: add a `--go` flag that calls `ExecuteGoMigrationInit`.

### Step 4: Write integration test for init

```go
// cmd/go_init_test.go
package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/cmd"
)

func TestExecuteGoMigrationInit_NoSnapshot(t *testing.T) {
	// Set up a temp directory as working dir
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)

	// Create go.mod so the function doesn't error
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testproject\n\ngo 1.24\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "migrations"), 0755)

	if err := cmd.ExecuteGoMigrationInit("postgresql", false); err != nil {
		t.Fatalf("ExecuteGoMigrationInit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "migrations", "main.go")); os.IsNotExist(err) {
		t.Error("expected main.go to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, "migrations", "go.mod")); os.IsNotExist(err) {
		t.Error("expected go.mod to be created")
	}
}
```

### Step 5: Build and full test suite

```bash
cd /workspaces/ocom/go/makemigrations && go build ./... && go test ./... 2>&1 | tail -30
golangci-lint run --no-config ./... 2>&1
```

Expected: all tests pass, binary builds, no lint issues.

### Step 6: Commit

```bash
git add cmd/sql_migrations.go cmd/go_init.go cmd/init.go cmd/go_init_test.go
git commit -m "feat(cmd): rename sql-migrations, add Go migration init with snapshot bootstrap"
```

---

## Task 12: Final Integration Test + Documentation

**Files:**
- Create: `migrate/integration_test.go`

### Step 1: Write full round-trip integration test

```go
// migrate/integration_test.go
package migrate_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/migrate"
)

// TestFullRoundTrip tests the complete lifecycle:
// register migrations → build graph → reconstruct state → apply → rollback → verify
func TestFullRoundTrip(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
				Indexes: []migrate.Index{
					{Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
				},
			},
		},
	})
	reg.Register(&migrate.Migration{
		Name:         "0002_add_phone",
		Dependencies: []string{"0001_initial"},
		Operations: []migrate.Operation{
			&migrate.AddField{
				Table: "users",
				Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
			},
		},
	})

	// Build graph
	g, err := migrate.BuildGraph(reg)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Reconstruct state
	state, err := g.ReconstructState()
	if err != nil {
		t.Fatalf("ReconstructState: %v", err)
	}
	if len(state.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(state.Tables))
	}
	if len(state.Tables["users"].Fields) != 3 {
		t.Fatalf("expected 3 fields (id, email, phone), got %d", len(state.Tables["users"].Fields))
	}

	// DAG output
	dagOut, err := g.ToDAGOutput()
	if err != nil {
		t.Fatalf("ToDAGOutput: %v", err)
	}
	if dagOut.HasBranches {
		t.Fatal("expected no branches")
	}
	if len(dagOut.Leaves) != 1 || dagOut.Leaves[0] != "0002_add_phone" {
		t.Fatalf("expected leaf '0002_add_phone', got %v", dagOut.Leaves)
	}

	// Run against SQLite
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite3: %v", err)
	}
	defer db.Close()

	recorder := migrate.NewMigrationRecorder(db)
	_ = recorder.EnsureTable()

	p := sqlite.NewProvider()
	runner := migrate.NewRunner(g, p, db, recorder)

	if err := runner.Up(""); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Verify both tables exist and phone column is present
	if _, err := db.Exec("INSERT INTO users (email, phone) VALUES ('a@b.com', '0412345678')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Roll back
	if err := runner.Down(2, ""); err != nil {
		t.Fatalf("Down: %v", err)
	}

	// Verify table gone
	if _, err := db.Exec("SELECT 1 FROM users"); err == nil {
		t.Fatal("expected users table to be dropped")
	}
}
```

### Step 2: Run full test suite

```bash
cd /workspaces/ocom/go/makemigrations && go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

Expected: all packages `ok`.

### Step 3: Final lint

```bash
golangci-lint run --no-config ./... 2>&1
```

Expected: no new issues (pre-existing issues in `db2schema.go`, `goose.go`, `schema2diagram.go` are acceptable per existing project state).

### Step 4: Final commit

```bash
git add migrate/integration_test.go
git commit -m "test(migrate): add full round-trip integration test"
```

---

## Summary of All New Files

| File | Purpose |
|------|---------|
| `migrate/types.go` | Migration, Field, Index, ForeignKey types |
| `migrate/state.go` | In-memory SchemaState with table/field/index mutation methods |
| `migrate/operations.go` | Operation interface + 10 concrete types |
| `migrate/registry.go` | Global registry with Register() for init() pattern |
| `migrate/graph.go` | DAG, topological sort, branch detection, ReconstructState, DAGOutput |
| `migrate/config.go` | Config struct and EnvOr helper |
| `migrate/app.go` | Cobra CLI: dag/up/down/status/showsql/fake commands |
| `migrate/dag_ascii.go` | ASCII tree renderer |
| `migrate/recorder.go` | makemigrations_history table management |
| `migrate/runner.go` | Migration execution: Up/Down/Status/ShowSQL |
| `migrate/provider_bridge.go` | Thin wrapper for internal provider factory |
| `internal/codegen/go_generator.go` | Go migration file generator from SchemaDiff |
| `internal/codegen/merge_generator.go` | Merge migration generator |
| `internal/codegen/squash_generator.go` | Squash migration generator |
| `cmd/go_migrations.go` | makemigrations CLI command (binary-query loop) |
| `cmd/go_init.go` | Go migration init with snapshot bootstrap |
| `cmd/sql_migrations.go` | Renamed from makemigrations.go (legacy SQL workflow) |

## Unchanged Packages

`internal/yaml/`, `internal/types/`, `internal/providers/`, `internal/config/`, `internal/scanner/` — zero modifications required.
