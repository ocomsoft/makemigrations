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

func TestMergeSchemas(t *testing.T) {
	merger := NewMerger(false)

	// Create two schemas with overlapping tables
	schema1 := &Schema{
		Database: Database{Name: "app", Version: "1.0.0"},
		Defaults: Defaults{
			PostgreSQL: map[string]string{"Now": "CURRENT_TIMESTAMP"},
		},
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

	schema2 := &Schema{
		Database: Database{Name: "app", Version: "1.0.0"},
		Defaults: Defaults{
			PostgreSQL: map[string]string{"Today": "CURRENT_DATE"},
		},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "name", Type: "varchar", Length: 100},
				},
			},
			{
				Name: "posts",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "title", Type: "varchar", Length: 255},
				},
			},
		},
	}

	merged, err := merger.MergeSchemas([]*Schema{schema1, schema2})
	if err != nil {
		t.Fatalf("Failed to merge schemas: %v", err)
	}

	// Validate merged result
	if len(merged.Tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(merged.Tables))
	}

	// Find users table
	var usersTable *Table
	for i := range merged.Tables {
		if merged.Tables[i].Name == "users" {
			usersTable = &merged.Tables[i]
			break
		}
	}

	if usersTable == nil {
		t.Fatal("Users table not found in merged schema")
	}

	// Should have 3 fields: id, email, name
	if len(usersTable.Fields) != 3 {
		t.Errorf("Expected 3 fields in users table, got %d", len(usersTable.Fields))
	}

	// Validate defaults were merged
	if len(merged.Defaults.PostgreSQL) != 2 {
		t.Errorf("Expected 2 PostgreSQL defaults, got %d", len(merged.Defaults.PostgreSQL))
	}
}

func TestMergeFieldsWithConflictResolution(t *testing.T) {
	merger := NewMerger(false)

	// Test VARCHAR length conflict resolution (larger wins)
	field1 := Field{Name: "name", Type: "varchar", Length: 100}
	field2 := Field{Name: "name", Type: "varchar", Length: 255}

	merged, err := merger.mergeFields("users", "name", []Field{field1, field2})
	if err != nil {
		t.Fatalf("Failed to merge fields: %v", err)
	}

	if merged.Length != 255 {
		t.Errorf("Expected length 255, got %d", merged.Length)
	}

	// Test nullable conflict resolution (NOT NULL wins)
	nullable := true
	notNullable := false
	field1 = Field{Name: "email", Type: "varchar", Length: 255, Nullable: &nullable}
	field2 = Field{Name: "email", Type: "varchar", Length: 255, Nullable: &notNullable}

	merged, err = merger.mergeFields("users", "email", []Field{field1, field2})
	if err != nil {
		t.Fatalf("Failed to merge fields: %v", err)
	}

	if merged.IsNullable() {
		t.Error("Expected NOT NULL to win in conflict resolution")
	}

	// Test type promotion (integer to bigint)
	field1 = Field{Name: "count", Type: "integer"}
	field2 = Field{Name: "count", Type: "bigint"}

	merged, err = merger.mergeFields("users", "count", []Field{field1, field2})
	if err != nil {
		t.Fatalf("Failed to merge fields: %v", err)
	}

	if merged.Type != "bigint" {
		t.Errorf("Expected type 'bigint', got '%s'", merged.Type)
	}
}

func TestIncompatibleTypeConflict(t *testing.T) {
	merger := NewMerger(false)

	// Test incompatible types
	field1 := Field{Name: "data", Type: "varchar", Length: 255}
	field2 := Field{Name: "data", Type: "integer"}

	_, err := merger.mergeFields("users", "data", []Field{field1, field2})
	if err == nil {
		t.Fatal("Expected error for incompatible field types")
	}
}

func TestForeignKeyConflictResolution(t *testing.T) {
	merger := NewMerger(false)

	// Test compatible foreign key definitions
	field1 := Field{
		Name: "user_id",
		Type: "foreign_key",
		ForeignKey: &ForeignKey{
			Table:    "users",
			OnDelete: "CASCADE",
		},
	}
	field2 := Field{
		Name: "user_id",
		Type: "foreign_key",
		ForeignKey: &ForeignKey{
			Table:    "users",
			OnDelete: "CASCADE",
		},
	}

	merged, err := merger.mergeFields("posts", "user_id", []Field{field1, field2})
	if err != nil {
		t.Fatalf("Failed to merge foreign key fields: %v", err)
	}

	if merged.ForeignKey.Table != "users" {
		t.Errorf("Expected foreign key table 'users', got '%s'", merged.ForeignKey.Table)
	}

	// Test incompatible foreign key definitions
	field3 := Field{
		Name: "user_id",
		Type: "foreign_key",
		ForeignKey: &ForeignKey{
			Table:    "accounts",
			OnDelete: "CASCADE",
		},
	}

	_, err = merger.mergeFields("posts", "user_id", []Field{field1, field3})
	if err == nil {
		t.Fatal("Expected error for incompatible foreign key definitions")
	}
}

func TestValidateMergedSchema(t *testing.T) {
	merger := NewMerger(false)

	// Create a valid merged schema
	schema := &Schema{
		Database: Database{Name: "app", Version: "1.0.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
			{
				Name: "posts",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "users"}},
				},
			},
		},
	}

	err := merger.ValidateMergedSchema(schema)
	if err != nil {
		t.Fatalf("Valid schema failed validation: %v", err)
	}

	// Test schema with multiple primary keys (should fail)
	invalidSchema := &Schema{
		Database: Database{Name: "app", Version: "1.0.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "uuid", Type: "uuid", PrimaryKey: true},
				},
			},
		},
	}

	err = merger.ValidateMergedSchema(invalidSchema)
	if err == nil {
		t.Fatal("Expected validation error for multiple primary keys")
	}
}
