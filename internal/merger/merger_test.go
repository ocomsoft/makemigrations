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
package merger

import (
	"testing"

	"github.com/ocomsoft/makemigrations/internal/parser"
)

func TestMerger_MergeSchemas(t *testing.T) {
	merger := New(false)

	// Create test statements
	statements := []parser.Statement{
		{
			Type:       parser.CreateTable,
			ObjectName: "users",
			Columns: []parser.Column{
				{Name: "id", DataType: "SERIAL", IsPrimaryKey: true, IsNullable: false},
				{Name: "email", DataType: "VARCHAR", Size: 255, IsNullable: false},
			},
		},
		{
			Type:           parser.CreateIndex,
			ObjectName:     "idx_users_email",
			IndexedTable:   "users",
			IndexedColumns: []string{"email"},
		},
	}

	schema, err := merger.MergeSchemas(statements, "test_module")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check tables
	if len(schema.Tables) != 1 {
		t.Fatalf("Expected 1 table, got %d", len(schema.Tables))
	}

	usersTable := schema.Tables["users"]
	if usersTable == nil {
		t.Fatal("Users table not found")
	}

	if len(usersTable.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(usersTable.Columns))
	}

	// Check indexes
	if len(schema.Indexes) != 1 {
		t.Fatalf("Expected 1 index, got %d", len(schema.Indexes))
	}
}

func TestMerger_MergeColumn(t *testing.T) {
	merger := New(false)

	// Test VARCHAR size conflict resolution
	existing := parser.Column{
		Name:       "email",
		DataType:   "VARCHAR",
		Size:       100,
		IsNullable: true,
	}

	new := parser.Column{
		Name:       "email",
		DataType:   "VARCHAR",
		Size:       255,
		IsNullable: false,
	}

	merged := merger.mergeColumn(existing, new, "test")

	if merged.Size != 255 {
		t.Errorf("Expected size 255 (larger wins), got %d", merged.Size)
	}

	if merged.IsNullable {
		t.Error("Expected NOT NULL to win over nullable")
	}
}

func TestMerger_MergeColumn_PrimaryKey(t *testing.T) {
	merger := New(false)

	existing := parser.Column{
		Name:       "id",
		DataType:   "INTEGER",
		IsNullable: true,
	}

	new := parser.Column{
		Name:         "id",
		DataType:     "INTEGER",
		IsPrimaryKey: true,
		IsNullable:   false,
	}

	merged := merger.mergeColumn(existing, new, "test")

	if !merged.IsPrimaryKey {
		t.Error("Expected primary key to be preserved")
	}

	if merged.IsNullable {
		t.Error("Expected primary key to be NOT NULL")
	}
}

func TestMerger_MergeColumn_DefaultValue(t *testing.T) {
	merger := New(false)

	// Test existing default wins
	existing := parser.Column{
		Name:         "status",
		DataType:     "VARCHAR",
		DefaultValue: "active",
	}

	new := parser.Column{
		Name:         "status",
		DataType:     "VARCHAR",
		DefaultValue: "pending",
	}

	merged := merger.mergeColumn(existing, new, "test")

	if merged.DefaultValue != "active" {
		t.Errorf("Expected existing default 'active', got '%s'", merged.DefaultValue)
	}

	// Test new default when existing is empty
	existing.DefaultValue = ""
	merged = merger.mergeColumn(existing, new, "test")

	if merged.DefaultValue != "pending" {
		t.Errorf("Expected new default 'pending', got '%s'", merged.DefaultValue)
	}
}

func TestMerger_MergeIndex_Identical(t *testing.T) {
	merger := New(false)

	schema := &MergedSchema{
		Tables:  make(map[string]*MergedTable),
		Indexes: make(map[string]*MergedIndex),
	}

	// Add first index
	stmt1 := parser.Statement{
		Type:           parser.CreateIndex,
		ObjectName:     "idx_test",
		IndexedTable:   "users",
		IndexedColumns: []string{"email"},
		IsUnique:       false,
	}

	err := merger.mergeIndex(schema, stmt1, "module1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Add identical index
	stmt2 := parser.Statement{
		Type:           parser.CreateIndex,
		ObjectName:     "idx_test",
		IndexedTable:   "users",
		IndexedColumns: []string{"email"},
		IsUnique:       false,
	}

	err = merger.mergeIndex(schema, stmt2, "module2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still have only one index
	if len(schema.Indexes) != 1 {
		t.Errorf("Expected 1 index after merging identical, got %d", len(schema.Indexes))
	}
}

func TestMerger_MergeIndex_Conflict(t *testing.T) {
	merger := New(false)

	schema := &MergedSchema{
		Tables:  make(map[string]*MergedTable),
		Indexes: make(map[string]*MergedIndex),
	}

	// Add first index
	stmt1 := parser.Statement{
		Type:           parser.CreateIndex,
		ObjectName:     "idx_test",
		IndexedTable:   "users",
		IndexedColumns: []string{"email"},
		IsUnique:       false,
	}

	err := merger.mergeIndex(schema, stmt1, "module1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Add conflicting index (different table)
	stmt2 := parser.Statement{
		Type:           parser.CreateIndex,
		ObjectName:     "idx_test",
		IndexedTable:   "posts",
		IndexedColumns: []string{"title"},
		IsUnique:       false,
	}

	err = merger.mergeIndex(schema, stmt2, "module2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have two indexes (original + renamed)
	if len(schema.Indexes) != 2 {
		t.Errorf("Expected 2 indexes after conflict resolution, got %d", len(schema.Indexes))
	}

	// Check that one was renamed
	hasOriginal := false
	hasRenamed := false
	for name := range schema.Indexes {
		if name == "idx_test" {
			hasOriginal = true
		}
		if name == "idx_test_2" {
			hasRenamed = true
		}
	}

	if !hasOriginal {
		t.Error("Expected original index name to be preserved")
	}
	if !hasRenamed {
		t.Error("Expected conflicting index to be renamed")
	}
}

func TestMerger_ForeignKeySignature(t *testing.T) {
	merger := New(false)

	fk := parser.ForeignKey{
		Columns:           []string{"user_id", "post_id"},
		ReferencedTable:   "users",
		ReferencedColumns: []string{"id", "alt_id"},
	}

	sig := merger.foreignKeySignature(fk)
	expected := "user_id,post_id->users(id,alt_id)"

	if sig != expected {
		t.Errorf("Expected signature '%s', got '%s'", expected, sig)
	}
}
