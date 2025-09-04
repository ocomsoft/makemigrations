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
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"

	"github.com/ocomsoft/makemigrations/internal/errors"
	"github.com/ocomsoft/makemigrations/internal/version"
)

// StateManager handles YAML schema state management
type StateManager struct {
	verbose bool
}

// NewStateManager creates a new YAML state manager
func NewStateManager(verbose bool) *StateManager {
	return &StateManager{
		verbose: verbose,
	}
}

// SaveSchemaSnapshot saves a YAML schema to the .schema_snapshot.yaml file
func (sm *StateManager) SaveSchemaSnapshot(schema *Schema, migrationsDir string) error {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	snapshotPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml")

	// Serialize schema to YAML
	yamlData, err := yaml.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(snapshotPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write schema snapshot: %w", err)
	}

	if sm.verbose {
		fmt.Printf("Saved schema snapshot to %s\n", snapshotPath)
	}

	return nil
}

// LoadSchemaSnapshot loads a YAML schema from the .schema_snapshot.yaml file
func (sm *StateManager) LoadSchemaSnapshot(migrationsDir string) (*Schema, error) {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	snapshotPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml")

	// Check if snapshot file exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		if sm.verbose {
			fmt.Printf("No existing schema snapshot found at %s\n", snapshotPath)
		}
		return nil, nil // Not an error, just no existing snapshot
	}

	// Read the file
	yamlData, err := os.ReadFile(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema snapshot: %w", err)
	}

	// Parse YAML
	var schema Schema
	if err := yaml.Unmarshal(yamlData, &schema); err != nil {
		return nil, errors.NewSchemaParseError(snapshotPath, 0, fmt.Sprintf("invalid YAML in snapshot: %v", err))
	}

	if sm.verbose {
		fmt.Printf("Loaded schema snapshot from %s with %d tables\n", snapshotPath, len(schema.Tables))
	}

	return &schema, nil
}

// SchemaSnapshotExists checks if a schema snapshot file exists
func (sm *StateManager) SchemaSnapshotExists(migrationsDir string) bool {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	snapshotPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml")
	_, err := os.Stat(snapshotPath)
	return err == nil
}

// GetSnapshotPath returns the path to the schema snapshot file
func (sm *StateManager) GetSnapshotPath(migrationsDir string) string {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}
	return filepath.Join(migrationsDir, ".schema_snapshot.yaml")
}

// ValidateSnapshot validates that a snapshot file is valid YAML schema
func (sm *StateManager) ValidateSnapshot(migrationsDir string) error {
	schema, err := sm.LoadSchemaSnapshot(migrationsDir)
	if err != nil {
		return err
	}

	if schema == nil {
		return fmt.Errorf("no schema snapshot found")
	}

	// Validate the schema structure
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("invalid schema snapshot: %w", err)
	}

	return nil
}

// CreateInitialSnapshot creates an initial empty schema snapshot
func (sm *StateManager) CreateInitialSnapshot(databaseName string, migrationsDir string) error {
	initialSchema := &Schema{
		Database: Database{
			Name:             databaseName,
			Version:          "1.0.0",
			MigrationVersion: version.GetVersion(),
		},
		Defaults: Defaults{
			PostgreSQL: map[string]string{
				"blank":        "''",
				"array":        "'[]'::jsonb",
				"object":       "'{}'::jsonb",
				"zero":         "0",
				"current_time": "CURRENT_TIME",
				"new_uuid":     "gen_random_uuid()",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "CURRENT_DATE",
				"false":        "false",
				"null":         "null",
				"true":         "true",
			},
			MySQL: map[string]string{
				"blank":        "''",
				"array":        "('[]')",
				"object":       "('{}')",
				"zero":         "0",
				"current_time": "(CURTIME())",
				"new_uuid":     "(UUID())",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "(CURDATE())",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
			SQLServer: map[string]string{
				"blank":        "''",
				"array":        "'[]'",
				"object":       "'{}'",
				"zero":         "0",
				"current_time": "CAST(GETDATE() AS TIME)",
				"new_uuid":     "NEWID()",
				"now":          "GETDATE()",
				"today":        "CAST(GETDATE() AS DATE)",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
			SQLite: map[string]string{
				"blank":        "''",
				"array":        "'[]'",
				"object":       "'{}'",
				"zero":         "0",
				"current_time": "CURRENT_TIME",
				"new_uuid":     "",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "CURRENT_DATE",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
		},
		Tables: []Table{},
	}

	return sm.SaveSchemaSnapshot(initialSchema, migrationsDir)
}

// CompareSnapshots compares two schema snapshots and returns whether they are equivalent
func (sm *StateManager) CompareSnapshots(schema1, schema2 *Schema) bool {
	if schema1 == nil && schema2 == nil {
		return true
	}
	if schema1 == nil || schema2 == nil {
		return false
	}

	// Compare database info
	if schema1.Database.Name != schema2.Database.Name {
		return false
	}

	// Compare number of tables
	if len(schema1.Tables) != len(schema2.Tables) {
		return false
	}

	// Create maps for easier comparison
	tables1 := make(map[string]*Table)
	tables2 := make(map[string]*Table)

	for i := range schema1.Tables {
		tables1[schema1.Tables[i].Name] = &schema1.Tables[i]
	}
	for i := range schema2.Tables {
		tables2[schema2.Tables[i].Name] = &schema2.Tables[i]
	}

	// Compare each table
	for tableName, table1 := range tables1 {
		table2, exists := tables2[tableName]
		if !exists {
			return false
		}

		if !sm.compareTables(table1, table2) {
			return false
		}
	}

	return true
}

// compareTables compares two table definitions
func (sm *StateManager) compareTables(table1, table2 *Table) bool {
	if table1.Name != table2.Name {
		return false
	}

	if len(table1.Fields) != len(table2.Fields) {
		return false
	}

	// Create maps for easier comparison
	fields1 := make(map[string]*Field)
	fields2 := make(map[string]*Field)

	for i := range table1.Fields {
		fields1[table1.Fields[i].Name] = &table1.Fields[i]
	}
	for i := range table2.Fields {
		fields2[table2.Fields[i].Name] = &table2.Fields[i]
	}

	// Compare each field
	for fieldName, field1 := range fields1 {
		field2, exists := fields2[fieldName]
		if !exists {
			return false
		}

		if !sm.compareFields(field1, field2) {
			return false
		}
	}

	return true
}

// compareFields compares two field definitions
func (sm *StateManager) compareFields(field1, field2 *Field) bool {
	return field1.Name == field2.Name &&
		field1.Type == field2.Type &&
		field1.PrimaryKey == field2.PrimaryKey &&
		field1.IsNullable() == field2.IsNullable() &&
		field1.Default == field2.Default &&
		field1.Length == field2.Length &&
		field1.Precision == field2.Precision &&
		field1.Scale == field2.Scale &&
		field1.AutoCreate == field2.AutoCreate &&
		field1.AutoUpdate == field2.AutoUpdate &&
		sm.compareForeignKeys(field1.ForeignKey, field2.ForeignKey) &&
		sm.compareManyToMany(field1.ManyToMany, field2.ManyToMany)
}

// compareForeignKeys compares two foreign key definitions
func (sm *StateManager) compareForeignKeys(fk1, fk2 *ForeignKey) bool {
	if fk1 == nil && fk2 == nil {
		return true
	}
	if fk1 == nil || fk2 == nil {
		return false
	}
	return fk1.Table == fk2.Table &&
		fk1.OnDelete == fk2.OnDelete
}

// compareManyToMany compares two many-to-many definitions
func (sm *StateManager) compareManyToMany(m2m1, m2m2 *ManyToMany) bool {
	if m2m1 == nil && m2m2 == nil {
		return true
	}
	if m2m1 == nil || m2m2 == nil {
		return false
	}
	return m2m1.Table == m2m2.Table
}

// BackupSnapshot creates a backup of the current snapshot
func (sm *StateManager) BackupSnapshot(migrationsDir string) error {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	snapshotPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml")
	backupPath := filepath.Join(migrationsDir, ".schema_snapshot.yaml.backup")

	// Check if snapshot exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return nil // No snapshot to backup
	}

	// Copy the file
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fmt.Errorf("failed to read snapshot for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot backup: %w", err)
	}

	if sm.verbose {
		fmt.Printf("Created snapshot backup at %s\n", backupPath)
	}

	return nil
}
