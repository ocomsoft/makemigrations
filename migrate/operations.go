/*
MIT License

# Copyright (c) 2025 OcomSoft

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// operations.go defines the Operation interface and all 10 concrete migration
// operation types used by the makemigrations Go migration framework.
package migrate

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers"
	"github.com/ocomsoft/makemigrations/internal/types"
)

// Operation is the interface all migration operations must implement.
// Each operation can generate forward (Up) and reverse (Down) SQL, mutate the
// in-memory SchemaState for graph traversal, and describe itself for display.
// Up/Down method names align with the CLI commands `./migrate up` / `./migrate down`.
type Operation interface {
	// Up generates the forward SQL for this operation.
	Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error)
	// Down generates the reverse SQL to undo this operation.
	Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error)
	// Mutate applies this operation to the given SchemaState, updating the
	// in-memory schema representation during migration graph traversal.
	Mutate(state *SchemaState) error
	// Describe returns a human-readable description of the operation.
	Describe() string
	// TypeName returns the operation type identifier (e.g. "create_table").
	TypeName() string
	// TableName returns the primary table this operation acts on, or "" for RunSQL.
	TableName() string
	// IsDestructive returns true if this operation may cause data loss.
	IsDestructive() bool
}

// boolPtr converts a bool value to a *bool pointer for use with types.Field.Nullable.
func boolPtr(b bool) *bool { return &b }

// toTypesField converts a migrate.Field (with bool Nullable) to a *types.Field
// (with *bool Nullable) as required by the providers.Provider interface.
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
		// types.ForeignKey only has Table and OnDelete (no OnUpdate).
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

// stateToSchema builds a minimal types.Schema from a SchemaState for provider
// calls that need the full schema (e.g. GenerateCreateTable for FK resolution).
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

// joinFields joins a slice of field names into a comma-separated string.
func joinFields(fields []string) string {
	return strings.Join(fields, ", ")
}

// --- CreateTable ---

// CreateTable is a migration operation that creates a new database table
// with the specified fields and indexes.
// When SchemaOnly is true the operation advances the in-memory schema state
// (via Mutate) but does not execute any SQL, allowing the schema state to be
// seeded from an existing database without re-running CREATE TABLE.
type CreateTable struct {
	Name       string
	Fields     []Field
	Indexes    []Index
	SchemaOnly bool // when true, Up/Down return no SQL; Mutate still runs
}

// TypeName returns the operation type identifier.
func (op *CreateTable) TypeName() string { return "create_table" }

// TableName returns the name of the table being created.
func (op *CreateTable) TableName() string { return op.Name }

// IsDestructive returns false — creating a table is not destructive.
func (op *CreateTable) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *CreateTable) Describe() string {
	return fmt.Sprintf("Create table %s (%d fields)", op.Name, len(op.Fields))
}

// Up generates the CREATE TABLE SQL statement, or returns empty string when SchemaOnly is set.
func (op *CreateTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
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

// Down generates the DROP TABLE SQL to reverse the creation.
// Returns empty string when SchemaOnly is set.
func (op *CreateTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
	return p.GenerateDropTable(op.Name), nil
}

// Mutate adds the new table to the SchemaState.
func (op *CreateTable) Mutate(state *SchemaState) error {
	return state.AddTable(op.Name, op.Fields, op.Indexes)
}

// --- DropTable ---

// DropTable is a migration operation that drops an existing database table.
// When SchemaOnly is true the operation advances the in-memory schema state
// (via Mutate) but does not execute any SQL against the database, allowing the
// developer to acknowledge a removal in the schema definition without
// immediately dropping the live table.
type DropTable struct {
	Name       string
	SchemaOnly bool // when true, Up/Down return no SQL; Mutate still runs
}

// TypeName returns the operation type identifier.
func (op *DropTable) TypeName() string { return "drop_table" }

// TableName returns the name of the table being dropped.
func (op *DropTable) TableName() string { return op.Name }

// IsDestructive returns true — dropping a table causes data loss.
func (op *DropTable) IsDestructive() bool { return true }

// Describe returns a human-readable description of this operation.
func (op *DropTable) Describe() string { return fmt.Sprintf("Drop table %s", op.Name) }

// Up generates the DROP TABLE SQL statement, or returns empty string when SchemaOnly is set.
func (op *DropTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
	return p.GenerateDropTable(op.Name), nil
}

// Down reconstructs the CREATE TABLE SQL by reading the table's pre-drop state.
// Returns empty string when SchemaOnly is set.
func (op *DropTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
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

// Mutate removes the table from the SchemaState.
func (op *DropTable) Mutate(state *SchemaState) error { return state.DropTable(op.Name) }

// --- RenameTable ---

// RenameTable is a migration operation that renames an existing database table.
type RenameTable struct{ OldName, NewName string }

// TypeName returns the operation type identifier.
func (op *RenameTable) TypeName() string { return "rename_table" }

// TableName returns the old (pre-rename) table name.
func (op *RenameTable) TableName() string { return op.OldName }

// IsDestructive returns false — renaming a table is not destructive.
func (op *RenameTable) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *RenameTable) Describe() string {
	return fmt.Sprintf("Rename table %s to %s", op.OldName, op.NewName)
}

// Up generates the RENAME TABLE SQL statement.
func (op *RenameTable) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameTable(op.OldName, op.NewName), nil
}

// Down generates the reverse RENAME TABLE SQL to restore the original name.
func (op *RenameTable) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameTable(op.NewName, op.OldName), nil
}

// Mutate updates the SchemaState to reflect the renamed table.
func (op *RenameTable) Mutate(state *SchemaState) error {
	return state.RenameTable(op.OldName, op.NewName)
}

// --- AddField ---

// AddField is a migration operation that adds a new column to an existing table.
// When SchemaOnly is true the operation advances the in-memory schema state
// (via Mutate) but does not execute any SQL, allowing the schema state to be
// seeded from an existing database without running ALTER TABLE ADD COLUMN.
type AddField struct {
	Table      string
	Field      Field
	SchemaOnly bool // when true, Up/Down return no SQL; Mutate still runs
}

// TypeName returns the operation type identifier.
func (op *AddField) TypeName() string { return "add_field" }

// TableName returns the name of the table being altered.
func (op *AddField) TableName() string { return op.Table }

// IsDestructive returns false — adding a column is not destructive.
func (op *AddField) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *AddField) Describe() string {
	return fmt.Sprintf("Add field %s.%s %s", op.Table, op.Field.Name, op.Field.Type)
}

// Up generates the ADD COLUMN SQL statement, or returns empty string when SchemaOnly is set.
func (op *AddField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
	return p.GenerateAddColumn(op.Table, toTypesField(op.Field)), nil
}

// Down generates the DROP COLUMN SQL to reverse the addition.
// Returns empty string when SchemaOnly is set.
func (op *AddField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
	return p.GenerateDropColumn(op.Table, op.Field.Name), nil
}

// Mutate adds the new field to the table's entry in SchemaState.
func (op *AddField) Mutate(state *SchemaState) error {
	return state.AddField(op.Table, op.Field)
}

// --- DropField ---

// DropField is a migration operation that removes a column from an existing table.
// When SchemaOnly is true the operation advances the in-memory schema state
// (via Mutate) but does not execute any SQL against the database, allowing the
// developer to acknowledge a removal in the schema definition without
// immediately dropping the live column.
type DropField struct {
	Table      string
	Field      string
	SchemaOnly bool // when true, Up/Down return no SQL; Mutate still runs
}

// TypeName returns the operation type identifier.
func (op *DropField) TypeName() string { return "drop_field" }

// TableName returns the name of the table being altered.
func (op *DropField) TableName() string { return op.Table }

// IsDestructive returns true — dropping a column causes data loss.
func (op *DropField) IsDestructive() bool { return true }

// Describe returns a human-readable description of this operation.
func (op *DropField) Describe() string {
	return fmt.Sprintf("Drop field %s.%s", op.Table, op.Field)
}

// Up generates the DROP COLUMN SQL statement, or returns empty string when SchemaOnly is set.
func (op *DropField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
	return p.GenerateDropColumn(op.Table, op.Field), nil
}

// Down reconstructs the ADD COLUMN SQL by reading the field's pre-drop state.
// Returns empty string when SchemaOnly is set.
func (op *DropField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	if op.SchemaOnly {
		return "", nil
	}
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

// Mutate removes the field from the table's entry in SchemaState.
func (op *DropField) Mutate(state *SchemaState) error {
	return state.DropField(op.Table, op.Field)
}

// --- AlterField ---

// AlterField is a migration operation that modifies an existing column's definition.
type AlterField struct {
	Table    string
	OldField Field
	NewField Field
}

// TypeName returns the operation type identifier.
func (op *AlterField) TypeName() string { return "alter_field" }

// TableName returns the name of the table being altered.
func (op *AlterField) TableName() string { return op.Table }

// IsDestructive returns false — altering a column is not considered destructive
// (though data conversion may fail at the database level for incompatible types).
func (op *AlterField) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *AlterField) Describe() string {
	return fmt.Sprintf("Alter field %s.%s", op.Table, op.NewField.Name)
}

// Up generates the ALTER COLUMN SQL to apply the new field definition.
func (op *AlterField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateAlterColumn(op.Table, toTypesField(op.OldField), toTypesField(op.NewField))
}

// Down generates the ALTER COLUMN SQL to restore the original field definition.
func (op *AlterField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateAlterColumn(op.Table, toTypesField(op.NewField), toTypesField(op.OldField))
}

// Mutate replaces the field in the table's entry in SchemaState.
func (op *AlterField) Mutate(state *SchemaState) error {
	return state.AlterField(op.Table, op.NewField)
}

// --- RenameField ---

// RenameField is a migration operation that renames a column in an existing table.
type RenameField struct {
	Table   string
	OldName string
	NewName string
}

// TypeName returns the operation type identifier.
func (op *RenameField) TypeName() string { return "rename_field" }

// TableName returns the name of the table being altered.
func (op *RenameField) TableName() string { return op.Table }

// IsDestructive returns false — renaming a column is not destructive.
func (op *RenameField) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *RenameField) Describe() string {
	return fmt.Sprintf("Rename field %s.%s to %s", op.Table, op.OldName, op.NewName)
}

// Up generates the RENAME COLUMN SQL statement.
func (op *RenameField) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameColumn(op.Table, op.OldName, op.NewName), nil
}

// Down generates the reverse RENAME COLUMN SQL to restore the original name.
func (op *RenameField) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateRenameColumn(op.Table, op.NewName, op.OldName), nil
}

// Mutate updates the field name in the table's entry in SchemaState.
func (op *RenameField) Mutate(state *SchemaState) error {
	return state.RenameField(op.Table, op.OldName, op.NewName)
}

// --- AddIndex ---

// AddIndex is a migration operation that adds an index to an existing table.
type AddIndex struct {
	Table string
	Index Index
}

// TypeName returns the operation type identifier.
func (op *AddIndex) TypeName() string { return "add_index" }

// TableName returns the name of the table being indexed.
func (op *AddIndex) TableName() string { return op.Table }

// IsDestructive returns false — adding an index is not destructive.
func (op *AddIndex) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *AddIndex) Describe() string {
	return fmt.Sprintf("Add index %s on %s(%s)", op.Index.Name, op.Table, joinFields(op.Index.Fields))
}

// Up generates the CREATE INDEX SQL statement.
func (op *AddIndex) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	ti := &types.Index{Name: op.Index.Name, Unique: op.Index.Unique, Fields: op.Index.Fields}
	return p.GenerateCreateIndex(ti, op.Table), nil
}

// Down generates the DROP INDEX SQL to reverse the index creation.
func (op *AddIndex) Down(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropIndex(op.Index.Name, op.Table), nil
}

// Mutate adds the index to the table's entry in SchemaState.
func (op *AddIndex) Mutate(state *SchemaState) error {
	return state.AddIndex(op.Table, op.Index)
}

// --- DropIndex ---

// DropIndex is a migration operation that removes an index from an existing table.
type DropIndex struct {
	Table string
	Index string
}

// TypeName returns the operation type identifier.
func (op *DropIndex) TypeName() string { return "drop_index" }

// TableName returns the name of the table whose index is being dropped.
func (op *DropIndex) TableName() string { return op.Table }

// IsDestructive returns false — dropping an index is not considered destructive.
func (op *DropIndex) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *DropIndex) Describe() string {
	return fmt.Sprintf("Drop index %s on %s", op.Index, op.Table)
}

// Up generates the DROP INDEX SQL statement.
func (op *DropIndex) Up(p providers.Provider, state *SchemaState, defaults map[string]string) (string, error) {
	return p.GenerateDropIndex(op.Index, op.Table), nil
}

// Down reconstructs the CREATE INDEX SQL by reading the index's pre-drop state.
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

// Mutate removes the index from the table's entry in SchemaState.
func (op *DropIndex) Mutate(state *SchemaState) error {
	return state.DropIndex(op.Table, op.Index)
}

// --- RunSQL ---

// RunSQL is a migration operation that executes raw SQL for forward and reverse
// migrations. ForwardSQL and BackwardSQL field names are used (not Forward/Backward)
// to avoid conflict with the Up/Down method names on the Operation interface.
type RunSQL struct {
	// ForwardSQL is the raw SQL to execute on Up (forward migration).
	ForwardSQL string
	// BackwardSQL is the raw SQL to execute on Down (reverse migration).
	BackwardSQL string
}

// TypeName returns the operation type identifier.
func (op *RunSQL) TypeName() string { return "run_sql" }

// TableName returns an empty string — RunSQL does not target a specific table.
func (op *RunSQL) TableName() string { return "" }

// IsDestructive returns false — RunSQL destructiveness depends on the SQL content.
func (op *RunSQL) IsDestructive() bool { return false }

// Describe returns a human-readable description of this operation.
func (op *RunSQL) Describe() string { return "Run SQL" }

// Up returns the ForwardSQL string directly; no provider is needed.
func (op *RunSQL) Up(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return op.ForwardSQL, nil
}

// Down returns the BackwardSQL string directly; no provider is needed.
func (op *RunSQL) Down(_ providers.Provider, _ *SchemaState, _ map[string]string) (string, error) {
	return op.BackwardSQL, nil
}

// Mutate is a no-op for RunSQL — raw SQL does not alter the SchemaState.
func (op *RunSQL) Mutate(_ *SchemaState) error { return nil }
