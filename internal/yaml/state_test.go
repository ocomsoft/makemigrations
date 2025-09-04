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
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadSchemaSnapshot(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	// Create a test schema
	schema := &Schema{
		Database: Database{Name: "test_app", Version: "1.0.0"},
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

	// Save the schema
	err := sm.SaveSchemaSnapshot(schema, tempDir)
	if err != nil {
		t.Fatalf("Failed to save schema snapshot: %v", err)
	}

	// Check that the file was created
	snapshotPath := filepath.Join(tempDir, ".schema_snapshot.yaml")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Fatal("Schema snapshot file was not created")
	}

	// Load the schema back
	loadedSchema, err := sm.LoadSchemaSnapshot(tempDir)
	if err != nil {
		t.Fatalf("Failed to load schema snapshot: %v", err)
	}

	if loadedSchema == nil {
		t.Fatal("Loaded schema is nil")
	}

	// Validate the loaded schema
	if loadedSchema.Database.Name != "test_app" {
		t.Errorf("Expected database name 'test_app', got '%s'", loadedSchema.Database.Name)
	}

	if len(loadedSchema.Tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(loadedSchema.Tables))
	}

	if loadedSchema.Tables[0].Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", loadedSchema.Tables[0].Name)
	}
}

func TestLoadNonExistentSnapshot(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	// Try to load from directory with no snapshot
	schema, err := sm.LoadSchemaSnapshot(tempDir)
	if err != nil {
		t.Fatalf("Expected no error for non-existent snapshot, got: %v", err)
	}

	if schema != nil {
		t.Error("Expected nil schema for non-existent snapshot")
	}
}

func TestSchemaSnapshotExists(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	// Should not exist initially
	if sm.SchemaSnapshotExists(tempDir) {
		t.Error("Expected snapshot to not exist initially")
	}

	// Create a schema
	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables:   []Table{},
	}

	// Save it
	err := sm.SaveSchemaSnapshot(schema, tempDir)
	if err != nil {
		t.Fatalf("Failed to save schema: %v", err)
	}

	// Should exist now
	if !sm.SchemaSnapshotExists(tempDir) {
		t.Error("Expected snapshot to exist after saving")
	}
}

func TestValidateSnapshot(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	// Create valid schema
	validSchema := &Schema{
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

	err := sm.SaveSchemaSnapshot(validSchema, tempDir)
	if err != nil {
		t.Fatalf("Failed to save valid schema: %v", err)
	}

	// Validate should succeed
	err = sm.ValidateSnapshot(tempDir)
	if err != nil {
		t.Errorf("Valid snapshot failed validation: %v", err)
	}

	// Test with invalid YAML
	snapshotPath := filepath.Join(tempDir, ".schema_snapshot.yaml")
	err = os.WriteFile(snapshotPath, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	err = sm.ValidateSnapshot(tempDir)
	if err == nil {
		t.Error("Expected validation error for invalid YAML")
	}
}

func TestCreateInitialSnapshot(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	err := sm.CreateInitialSnapshot("myapp", tempDir)
	if err != nil {
		t.Fatalf("Failed to create initial snapshot: %v", err)
	}

	// Load and validate the initial snapshot
	schema, err := sm.LoadSchemaSnapshot(tempDir)
	if err != nil {
		t.Fatalf("Failed to load initial snapshot: %v", err)
	}

	if schema.Database.Name != "myapp" {
		t.Errorf("Expected database name 'myapp', got '%s'", schema.Database.Name)
	}

	if len(schema.Tables) != 0 {
		t.Errorf("Expected 0 tables in initial snapshot, got %d", len(schema.Tables))
	}

	// Should have defaults for all database types
	if len(schema.Defaults.PostgreSQL) == 0 {
		t.Error("Expected PostgreSQL defaults in initial snapshot")
	}
	if len(schema.Defaults.MySQL) == 0 {
		t.Error("Expected MySQL defaults in initial snapshot")
	}
	if len(schema.Defaults.SQLServer) == 0 {
		t.Error("Expected SQLServer defaults in initial snapshot")
	}
	if len(schema.Defaults.SQLite) == 0 {
		t.Error("Expected SQLite defaults in initial snapshot")
	}
}

func TestCompareSnapshots(t *testing.T) {
	sm := NewStateManager(false)

	// Create two identical schemas
	schema1 := &Schema{
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

	schema2 := &Schema{
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

	// Should be equal
	if !sm.CompareSnapshots(schema1, schema2) {
		t.Error("Expected identical schemas to be equal")
	}

	// Test with different table count
	schema3 := &Schema{
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

	if sm.CompareSnapshots(schema1, schema3) {
		t.Error("Expected schemas with different table counts to be different")
	}

	// Test with different field properties
	schema4 := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 100}, // Different length
				},
			},
		},
	}

	if sm.CompareSnapshots(schema1, schema4) {
		t.Error("Expected schemas with different field properties to be different")
	}
}

func TestBackupSnapshot(t *testing.T) {
	sm := NewStateManager(false)
	tempDir := t.TempDir()

	// Create and save a schema
	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Tables:   []Table{},
	}

	err := sm.SaveSchemaSnapshot(schema, tempDir)
	if err != nil {
		t.Fatalf("Failed to save schema: %v", err)
	}

	// Create backup
	err = sm.BackupSnapshot(tempDir)
	if err != nil {
		t.Fatalf("Failed to backup snapshot: %v", err)
	}

	// Check that backup file exists
	backupPath := filepath.Join(tempDir, ".schema_snapshot.yaml.backup")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file was not created")
	}

	// Test backup with no existing snapshot
	tempDir2 := t.TempDir()
	err = sm.BackupSnapshot(tempDir2)
	if err != nil {
		t.Errorf("Expected no error when backing up non-existent snapshot, got: %v", err)
	}
}
