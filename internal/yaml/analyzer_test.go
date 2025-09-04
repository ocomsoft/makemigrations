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

func TestAnalyzeDependencies(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
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
					{Name: "title", Type: "varchar", Length: 255},
				},
			},
			{
				Name: "tags",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "name", Type: "varchar", Length: 100},
					{Name: "posts", Type: "many_to_many", ManyToMany: &ManyToMany{Table: "posts"}},
				},
			},
		},
	}

	dependencies, err := analyzer.AnalyzeDependencies(schema)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}

	if len(dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(dependencies))
	}

	// Check foreign key dependency
	fkFound := false
	m2mFound := false
	for _, dep := range dependencies {
		if dep.Type == "foreign_key" && dep.FromTable == "posts" && dep.ToTable == "users" {
			fkFound = true
		}
		if dep.Type == "many_to_many" && dep.FromTable == "tags" && dep.ToTable == "posts" {
			m2mFound = true
		}
	}

	if !fkFound {
		t.Error("Foreign key dependency not found")
	}
	if !m2mFound {
		t.Error("Many-to-many dependency not found")
	}
}

func TestTopologicalSort(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	schema := &Schema{
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
					{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "users"}},
				},
			},
			{
				Name: "comments",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "post_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "posts"}},
					{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "users"}},
				},
			},
		},
	}

	sortedTables, err := analyzer.TopologicalSort(schema)
	if err != nil {
		t.Fatalf("Failed to sort tables: %v", err)
	}

	if len(sortedTables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(sortedTables))
	}

	// Users should come first (no dependencies)
	if sortedTables[0] != "users" {
		t.Errorf("Expected 'users' first, got '%s'", sortedTables[0])
	}

	// Posts should come before comments (comments depends on posts)
	postsIndex := -1
	commentsIndex := -1
	for i, table := range sortedTables {
		if table == "posts" {
			postsIndex = i
		}
		if table == "comments" {
			commentsIndex = i
		}
	}

	if postsIndex == -1 || commentsIndex == -1 {
		t.Fatal("Posts or comments table not found in sorted result")
	}

	if postsIndex >= commentsIndex {
		t.Error("Posts should come before comments in topological sort")
	}
}

func TestCircularDependencyDetection(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	// Create a schema with circular dependency
	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "table_a",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "b_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "table_b"}},
				},
			},
			{
				Name: "table_b",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "a_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "table_a"}},
				},
			},
		},
	}

	_, err := analyzer.TopologicalSort(schema)
	if err == nil {
		t.Fatal("Expected error for circular dependency")
	}

	err = analyzer.ValidateCircularDependencies(schema)
	if err == nil {
		t.Fatal("Expected validation error for circular dependency")
	}
}

func TestGenerateJunctionTables(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "roles", Type: "many_to_many", ManyToMany: &ManyToMany{Table: "roles"}},
				},
			},
			{
				Name: "roles",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "name", Type: "varchar", Length: 100},
				},
			},
		},
	}

	junctionTables, err := analyzer.GenerateJunctionTables(schema)
	if err != nil {
		t.Fatalf("Failed to generate junction tables: %v", err)
	}

	if len(junctionTables) != 1 {
		t.Errorf("Expected 1 junction table, got %d", len(junctionTables))
	}

	junctionTable := junctionTables[0]
	expectedName := "users_roles"
	if junctionTable.Name != expectedName {
		t.Errorf("Expected junction table name '%s', got '%s'", expectedName, junctionTable.Name)
	}

	// Should have 3 fields: id, users_id, roles_id
	if len(junctionTable.Fields) != 3 {
		t.Errorf("Expected 3 fields in junction table, got %d", len(junctionTable.Fields))
	}

	// Validate field names and types
	expectedFields := map[string]string{
		"id":       "serial",
		"users_id": "integer",
		"roles_id": "integer",
	}

	for _, field := range junctionTable.Fields {
		expectedType, exists := expectedFields[field.Name]
		if !exists {
			t.Errorf("Unexpected field in junction table: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Expected field %s to have type %s, got %s", field.Name, expectedType, field.Type)
		}
	}
}

func TestGetDependentTables(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	schema := &Schema{
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
					{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "users"}},
				},
			},
			{
				Name: "comments",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "users"}},
				},
			},
		},
	}

	dependents, err := analyzer.GetDependentTables(schema, "users")
	if err != nil {
		t.Fatalf("Failed to get dependent tables: %v", err)
	}

	if len(dependents) != 2 {
		t.Errorf("Expected 2 dependent tables, got %d", len(dependents))
	}

	// Check that both posts and comments are in the dependents list
	dependentMap := make(map[string]bool)
	for _, dep := range dependents {
		dependentMap[dep] = true
	}

	if !dependentMap["posts"] || !dependentMap["comments"] {
		t.Error("Expected posts and comments to be dependent on users")
	}
}

func TestGetManyToManyRelationships(t *testing.T) {
	analyzer := NewDependencyAnalyzer(false)

	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "roles", Type: "many_to_many", ManyToMany: &ManyToMany{Table: "roles"}},
					{Name: "projects", Type: "many_to_many", ManyToMany: &ManyToMany{Table: "projects"}},
				},
			},
			{
				Name: "roles",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
			{
				Name: "projects",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
		},
	}

	relationships := analyzer.GetManyToManyRelationships(schema)

	if len(relationships) != 2 {
		t.Errorf("Expected 2 many-to-many relationships, got %d", len(relationships))
	}

	// Check that we have the expected relationships
	expectedRelationships := map[string]string{
		"roles":    "roles",
		"projects": "projects",
	}

	for _, rel := range relationships {
		if rel.FromTable != "users" {
			t.Errorf("Expected relationship from 'users', got from '%s'", rel.FromTable)
		}
		if expectedTable, exists := expectedRelationships[rel.FieldName]; !exists || expectedTable != rel.ToTable {
			t.Errorf("Unexpected relationship: %s -> %s", rel.FieldName, rel.ToTable)
		}
	}
}
