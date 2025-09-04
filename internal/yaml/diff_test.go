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
package yaml

import (
	"testing"
)

func TestCompareSchemas_InitialMigration(t *testing.T) {
	de := NewDiffEngine(false)

	// Test initial migration (old schema is nil)
	newSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}

	diff, err := de.CompareSchemas(nil, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if !diff.HasChanges {
		t.Error("Expected changes for initial migration")
	}

	if len(diff.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(diff.Changes))
	}

	change := diff.Changes[0]
	if change.Type != ChangeTypeTableAdded {
		t.Errorf("Expected table added change, got %s", change.Type)
	}

	if change.TableName != "users" {
		t.Errorf("Expected table name 'users', got '%s'", change.TableName)
	}
}

func TestCompareSchemas_TableChanges(t *testing.T) {
	de := NewDiffEngine(false)

	oldSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
		},
	}

	newSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
			{
				Name: "posts",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
		},
	}

	diff, err := de.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if !diff.HasChanges {
		t.Error("Expected changes for new table")
	}

	if len(diff.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(diff.Changes))
	}

	change := diff.Changes[0]
	if change.Type != ChangeTypeTableAdded {
		t.Errorf("Expected table added change, got %s", change.Type)
	}

	if change.TableName != "posts" {
		t.Errorf("Expected table name 'posts', got '%s'", change.TableName)
	}
}

func TestCompareSchemas_FieldChanges(t *testing.T) {
	de := NewDiffEngine(false)

	oldSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}

	newSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
					{Name: "name", Type: "varchar", Length: 100},
				},
			},
		},
	}

	diff, err := de.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if !diff.HasChanges {
		t.Error("Expected changes for new field")
	}

	if len(diff.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(diff.Changes))
	}

	change := diff.Changes[0]
	if change.Type != ChangeTypeFieldAdded {
		t.Errorf("Expected field added change, got %s", change.Type)
	}

	if change.FieldName != "name" {
		t.Errorf("Expected field name 'name', got '%s'", change.FieldName)
	}
}

func TestCompareSchemas_FieldModification(t *testing.T) {
	de := NewDiffEngine(false)

	nullable := true
	notNullable := false

	oldSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "email", Type: "varchar", Length: 255, Nullable: &nullable},
				},
			},
		},
	}

	newSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "email", Type: "varchar", Length: 500, Nullable: &notNullable},
				},
			},
		},
	}

	diff, err := de.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if !diff.HasChanges {
		t.Error("Expected changes for field modification")
	}

	// Should have 2 changes: length and nullable
	if len(diff.Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(diff.Changes))
	}

	// Check that one change is destructive (nullable -> not nullable)
	destructiveFound := false
	for _, change := range diff.Changes {
		if change.Destructive {
			destructiveFound = true
			break
		}
	}

	if !destructiveFound {
		t.Error("Expected at least one destructive change")
	}

	if !diff.IsDestructive {
		t.Error("Expected diff to be marked as destructive")
	}
}

func TestCompareSchemas_TypeChange(t *testing.T) {
	de := NewDiffEngine(false)

	oldSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "count", Type: "integer"},
				},
			},
		},
	}

	// Test safe type promotion (integer -> bigint)
	newSchema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "count", Type: "bigint"},
				},
			},
		},
	}

	diff, err := de.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if len(diff.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(diff.Changes))
	}

	// Safe type promotion should not be destructive
	if diff.Changes[0].Destructive {
		t.Error("Safe type promotion should not be destructive")
	}

	// Test destructive type change
	newSchema.Tables[0].Fields[0].Type = "varchar"
	diff, err = de.CompareSchemas(oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	if !diff.Changes[0].Destructive {
		t.Error("Incompatible type change should be destructive")
	}
}

func TestGenerateMigrationName(t *testing.T) {
	de := NewDiffEngine(false)

	// Test single table addition
	diff := &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeTableAdded, TableName: "users"},
		},
		HasChanges: true,
	}

	name := de.GenerateMigrationName(diff)
	expected := "add_users_table"
	if name != expected {
		t.Errorf("Expected migration name '%s', got '%s'", expected, name)
	}

	// Test single field addition
	diff = &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeFieldAdded, TableName: "users", FieldName: "email"},
		},
		HasChanges: true,
	}

	name = de.GenerateMigrationName(diff)
	expected = "add_email_to_users"
	if name != expected {
		t.Errorf("Expected migration name '%s', got '%s'", expected, name)
	}

	// Test multiple changes
	diff = &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeTableAdded, TableName: "users"},
			{Type: ChangeTypeFieldAdded, TableName: "posts", FieldName: "title"},
		},
		HasChanges: true,
	}

	name = de.GenerateMigrationName(diff)
	expected = "update_schema"
	if name != expected {
		t.Errorf("Expected migration name '%s', got '%s'", expected, name)
	}

	// Test no changes
	diff = &SchemaDiff{
		Changes:    []Change{},
		HasChanges: false,
	}

	name = de.GenerateMigrationName(diff)
	expected = "no_changes"
	if name != expected {
		t.Errorf("Expected migration name '%s', got '%s'", expected, name)
	}
}

func TestGetChangesByType(t *testing.T) {
	de := NewDiffEngine(false)

	diff := &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeTableAdded, TableName: "users"},
			{Type: ChangeTypeFieldAdded, TableName: "users", FieldName: "email"},
			{Type: ChangeTypeTableAdded, TableName: "posts"},
		},
	}

	tableChanges := de.GetChangesByType(diff, ChangeTypeTableAdded)
	if len(tableChanges) != 2 {
		t.Errorf("Expected 2 table changes, got %d", len(tableChanges))
	}

	fieldChanges := de.GetChangesByType(diff, ChangeTypeFieldAdded)
	if len(fieldChanges) != 1 {
		t.Errorf("Expected 1 field change, got %d", len(fieldChanges))
	}
}

func TestGetDestructiveChanges(t *testing.T) {
	de := NewDiffEngine(false)

	diff := &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeTableAdded, TableName: "users", Destructive: false},
			{Type: ChangeTypeTableRemoved, TableName: "posts", Destructive: true},
			{Type: ChangeTypeFieldRemoved, TableName: "users", FieldName: "old_field", Destructive: true},
		},
	}

	destructiveChanges := de.GetDestructiveChanges(diff)
	if len(destructiveChanges) != 2 {
		t.Errorf("Expected 2 destructive changes, got %d", len(destructiveChanges))
	}

	for _, change := range destructiveChanges {
		if !change.Destructive {
			t.Error("All returned changes should be destructive")
		}
	}
}
