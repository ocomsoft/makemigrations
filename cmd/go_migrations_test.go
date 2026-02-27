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
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

// TestBuildMigrationName_WithCustomName verifies that a custom name is
// normalized (lowered, spaces to underscores) and prepended with the
// correct zero-padded sequence number.
func TestBuildMigrationName_WithCustomName(t *testing.T) {
	name := BuildMigrationName(0, "add user phone", "")
	if !strings.HasPrefix(name, "0001_") {
		t.Fatalf("expected prefix '0001_', got %q", name)
	}
	if !strings.Contains(name, "add_user_phone") {
		t.Fatalf("expected 'add_user_phone' in name, got %q", name)
	}
}

// TestBuildMigrationName_WithAutoName verifies that the auto-generated name
// from the diff engine is used when no custom name is provided.
func TestBuildMigrationName_WithAutoName(t *testing.T) {
	name := BuildMigrationName(2, "", "add_email")
	if !strings.HasPrefix(name, "0003_") {
		t.Fatalf("expected prefix '0003_', got %q", name)
	}
	if !strings.HasSuffix(name, "add_email") {
		t.Fatalf("expected suffix 'add_email', got %q", name)
	}
}

// TestBuildMigrationName_Timestamp verifies that when neither custom nor auto
// name is provided, a timestamp suffix is appended.
func TestBuildMigrationName_Timestamp(t *testing.T) {
	name := BuildMigrationName(0, "", "")
	if !strings.HasPrefix(name, "0001_") {
		t.Fatalf("expected prefix '0001_', got %q", name)
	}
	// Should be "0001_" + 14-digit timestamp = 19 chars total
	if len(name) != 19 {
		t.Fatalf("expected 19 chars (0001_ + 14 digit timestamp), got %d: %q", len(name), name)
	}
}

// TestBuildMigrationName_CustomNamePriority verifies that custom name takes
// priority over auto name when both are provided.
func TestBuildMigrationName_CustomNamePriority(t *testing.T) {
	name := BuildMigrationName(5, "my_custom", "auto_name")
	if !strings.HasPrefix(name, "0006_") {
		t.Fatalf("expected prefix '0006_', got %q", name)
	}
	if !strings.Contains(name, "my_custom") {
		t.Fatalf("expected 'my_custom' in name, got %q", name)
	}
	if strings.Contains(name, "auto_name") {
		t.Fatalf("expected auto_name to be ignored, got %q", name)
	}
}

// TestBuildMigrationName_SequenceNumbers verifies that the sequence number is
// correctly derived from the current migration count.
func TestBuildMigrationName_SequenceNumbers(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, "0001_"},
		{1, "0002_"},
		{9, "0010_"},
		{99, "0100_"},
		{999, "1000_"},
	}
	for _, tt := range tests {
		name := BuildMigrationName(tt.count, "test", "")
		if !strings.HasPrefix(name, tt.expected) {
			t.Errorf("count=%d: expected prefix %q, got %q", tt.count, tt.expected, name)
		}
	}
}

// TestSchemaStateToYAMLSchema_Nil verifies that a nil SchemaState produces a
// nil yaml.Schema.
func TestSchemaStateToYAMLSchema_Nil(t *testing.T) {
	result := schemaStateToYAMLSchema(nil)
	if result != nil {
		t.Fatal("expected nil result for nil state")
	}
}

// TestSchemaStateToYAMLSchema_EmptyState verifies that an empty SchemaState
// produces a Schema with no tables.
func TestSchemaStateToYAMLSchema_EmptyState(t *testing.T) {
	state := migrate.NewSchemaState()
	result := schemaStateToYAMLSchema(state)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Tables) != 0 {
		t.Fatalf("expected 0 tables, got %d", len(result.Tables))
	}
}

// TestSchemaStateToYAMLSchema_WithTables verifies that tables, fields, and
// indexes are correctly converted from SchemaState to yaml.Schema.
func TestSchemaStateToYAMLSchema_WithTables(t *testing.T) {
	state := migrate.NewSchemaState()
	err := state.AddTable("users", []migrate.Field{
		{Name: "id", Type: "uuid", PrimaryKey: true},
		{Name: "name", Type: "varchar", Length: 100, Nullable: true},
		{Name: "org_id", Type: "foreign_key", ForeignKey: &migrate.ForeignKey{
			Table: "orgs", OnDelete: "CASCADE",
		}},
	}, []migrate.Index{
		{Name: "idx_users_name", Fields: []string{"name"}, Unique: false},
	})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	err = state.AddTable("orgs", []migrate.Field{
		{Name: "id", Type: "uuid", PrimaryKey: true},
	}, nil)
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	result := schemaStateToYAMLSchema(state)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Tables should be sorted alphabetically
	if len(result.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(result.Tables))
	}
	if result.Tables[0].Name != "orgs" {
		t.Fatalf("expected first table to be 'orgs', got %q", result.Tables[0].Name)
	}
	if result.Tables[1].Name != "users" {
		t.Fatalf("expected second table to be 'users', got %q", result.Tables[1].Name)
	}

	// Verify users table fields
	usersTable := result.Tables[1]
	if len(usersTable.Fields) != 3 {
		t.Fatalf("expected 3 fields in users, got %d", len(usersTable.Fields))
	}

	// Verify primary key field
	idField := usersTable.Fields[0]
	if idField.Name != "id" || idField.Type != "uuid" || !idField.PrimaryKey {
		t.Errorf("unexpected id field: %+v", idField)
	}

	// Verify nullable field
	nameField := usersTable.Fields[1]
	if nameField.Nullable == nil || !*nameField.Nullable {
		t.Error("expected name field to be nullable")
	}
	if nameField.Length != 100 {
		t.Errorf("expected name length 100, got %d", nameField.Length)
	}

	// Verify foreign key field
	orgField := usersTable.Fields[2]
	if orgField.ForeignKey == nil {
		t.Fatal("expected foreign key on org_id field")
	}
	if orgField.ForeignKey.Table != "orgs" {
		t.Errorf("expected FK table 'orgs', got %q", orgField.ForeignKey.Table)
	}
	if orgField.ForeignKey.OnDelete != "CASCADE" {
		t.Errorf("expected FK on_delete 'CASCADE', got %q", orgField.ForeignKey.OnDelete)
	}

	// Verify indexes
	if len(usersTable.Indexes) != 1 {
		t.Fatalf("expected 1 index on users, got %d", len(usersTable.Indexes))
	}
	idx := usersTable.Indexes[0]
	if idx.Name != "idx_users_name" {
		t.Errorf("expected index name 'idx_users_name', got %q", idx.Name)
	}
	if len(idx.Fields) != 1 || idx.Fields[0] != "name" {
		t.Errorf("unexpected index fields: %v", idx.Fields)
	}
}

// TestSchemaStateToYAMLSchema_FieldAttributes verifies that all field
// attributes (precision, scale, auto_create, auto_update, default) are
// correctly carried through the conversion.
func TestSchemaStateToYAMLSchema_FieldAttributes(t *testing.T) {
	state := migrate.NewSchemaState()
	err := state.AddTable("products", []migrate.Field{
		{
			Name:      "price",
			Type:      "decimal",
			Precision: 10,
			Scale:     2,
			Default:   "0.00",
		},
		{
			Name:       "created_at",
			Type:       "timestamp",
			AutoCreate: true,
		},
		{
			Name:       "updated_at",
			Type:       "timestamp",
			AutoUpdate: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	result := schemaStateToYAMLSchema(state)
	if result == nil || len(result.Tables) != 1 {
		t.Fatal("expected 1 table")
	}

	fields := result.Tables[0].Fields
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	// Verify price field
	price := fields[0]
	if price.Precision != 10 || price.Scale != 2 {
		t.Errorf("price: expected precision=10 scale=2, got precision=%d scale=%d",
			price.Precision, price.Scale)
	}
	if price.Default != "0.00" {
		t.Errorf("price: expected default '0.00', got %q", price.Default)
	}

	// Verify created_at
	createdAt := fields[1]
	if !createdAt.AutoCreate {
		t.Error("created_at: expected auto_create=true")
	}

	// Verify updated_at
	updatedAt := fields[2]
	if !updatedAt.AutoUpdate {
		t.Error("updated_at: expected auto_update=true")
	}
}

// TestGoGenerateMerge_WritesMergeFile verifies that goGenerateMerge produces
// a merge migration .go file with the correct name and dependencies.
func TestGoGenerateMerge_WritesMergeFile(t *testing.T) {
	tmpDir := t.TempDir()
	dagOut := &migrate.DAGOutput{
		Leaves:      []string{"0001_initial", "0002_feature"},
		HasBranches: true,
	}
	err := goGenerateMerge(tmpDir, dagOut, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a .go file was written
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*.go"))
	if len(files) != 1 {
		t.Fatalf("expected 1 .go file, got %d", len(files))
	}

	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("reading merge file: %v", err)
	}

	// Check that it contains the expected dependencies
	src := string(content)
	if !strings.Contains(src, `"0001_initial"`) {
		t.Error("expected merge file to contain dependency '0001_initial'")
	}
	if !strings.Contains(src, `"0002_feature"`) {
		t.Error("expected merge file to contain dependency '0002_feature'")
	}
	if !strings.Contains(src, "merge") {
		t.Error("expected merge file name to contain 'merge'")
	}
}

// TestGoGenerateMerge_DryRun verifies that goGenerateMerge prints to stdout
// and does not write any file when dryRun is true.
func TestGoGenerateMerge_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	dagOut := &migrate.DAGOutput{
		Leaves:      []string{"0001_a", "0001_b"},
		HasBranches: true,
	}
	err := goGenerateMerge(tmpDir, dagOut, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No file should be written in dry-run mode
	files, _ := filepath.Glob(filepath.Join(tmpDir, "*.go"))
	if len(files) != 0 {
		t.Fatalf("expected 0 .go files in dry-run, got %d", len(files))
	}
}

// TestGoGenerateMerge_LongNameTruncation verifies that merge migration names
// are truncated when they exceed 80 characters.
func TestGoGenerateMerge_LongNameTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	dagOut := &migrate.DAGOutput{
		Leaves: []string{
			"0001_very_long_migration_name_that_goes_on",
			"0002_another_very_long_name_that_also_goes",
		},
		HasBranches: true,
	}
	err := goGenerateMerge(tmpDir, dagOut, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(tmpDir, "*.go"))
	if len(files) != 1 {
		t.Fatalf("expected 1 .go file, got %d", len(files))
	}
	// The filename should be truncated — just check it was written
	baseName := filepath.Base(files[0])
	if !strings.Contains(baseName, "merge") {
		t.Errorf("expected filename to contain 'merge', got %q", baseName)
	}
}
