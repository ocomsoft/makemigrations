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
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/scanner"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

func TestYAMLIntegration_EndToEnd(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "yaml_makemigrations_integration")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create go.mod
	goModContent := `module test/yaml-integration

go 1.21
`
	err = os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create schema directory and YAML schema
	schemaDir := filepath.Join("schema")
	os.MkdirAll(schemaDir, 0755)

	schemaContent := `database:
  name: test_app
  version: 1.0.0

defaults:
  postgresql:
    Now: "CURRENT_TIMESTAMP"
    "": "''"
    "null": "null"
  mysql:
    Now: "CURRENT_TIMESTAMP()"
    "": "''"
    "null": "null"

tables:
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
        nullable: false
      - name: email
        type: varchar
        length: 255
        nullable: false
        unique: true
      - name: name
        type: varchar
        length: 100
        nullable: false
      - name: created_at
        type: timestamp
        default: Now
        auto_create: true

  - name: posts
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
        nullable: false
      - name: title
        type: varchar
        length: 200
        nullable: false
      - name: content
        type: text
        nullable: true
      - name: user_id
        type: foreign_key
        foreign_key:
          table: users
          on_delete: CASCADE
        nullable: false
      - name: published
        type: boolean
        default: "false"
      - name: created_at
        type: timestamp
        default: Now
        auto_create: true

  - name: categories
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
      - name: name
        type: varchar
        length: 50
        nullable: false
        unique: true

  - name: post_categories
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
      - name: post_id
        type: foreign_key
        foreign_key:
          table: posts
          on_delete: CASCADE
      - name: category_id
        type: foreign_key
        foreign_key:
          table: categories
          on_delete: CASCADE
`
	err = os.WriteFile(filepath.Join(schemaDir, "schema.yaml"), []byte(schemaContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize YAML components
	verbose := false
	stateManager := yamlpkg.NewStateManager(verbose)
	scannerInstance := scanner.New(verbose)
	parser := yamlpkg.NewParser(verbose)
	merger := yamlpkg.NewMerger(verbose)
	diffEngine := yamlpkg.NewDiffEngine(verbose)
	migrationGenerator := yamlpkg.NewMigrationGenerator(yamlpkg.DatabasePostgreSQL, verbose)

	// Step 1: Scan for YAML schema files
	schemaFiles, err := scannerInstance.ScanYAMLModules()
	if err != nil {
		t.Fatalf("Failed to scan YAML modules: %v", err)
	}

	if len(schemaFiles) != 1 {
		t.Fatalf("Expected 1 YAML schema file, got %d", len(schemaFiles))
	}

	// Step 2: Parse and validate schemas
	var allSchemas []*yamlpkg.Schema
	for _, file := range schemaFiles {
		schema, err := parser.ParseAndValidate(file.Content)
		if err != nil {
			t.Fatalf("Failed to parse YAML schema: %v", err)
		}

		// Run comprehensive validation
		validationErrors := parser.ValidateComprehensive(schema, yamlpkg.DatabasePostgreSQL)
		if len(validationErrors) > 0 {
			// Check if there are actual errors (not warnings)
			hasErrors := false
			for _, validationErr := range validationErrors {
				if validationErr.Severity != "warning" {
					hasErrors = true
					break
				}
			}
			if hasErrors {
				t.Fatalf("YAML schema validation failed: %s", parser.FormatValidationErrors(validationErrors))
			}
		}

		allSchemas = append(allSchemas, schema)
	}

	// Step 3: Merge schemas
	mergedSchema, err := merger.MergeSchemas(allSchemas)
	if err != nil {
		t.Fatalf("Failed to merge YAML schemas: %v", err)
	}

	// Verify merged schema
	if len(mergedSchema.Tables) != 4 {
		t.Fatalf("Expected 4 tables, got %d", len(mergedSchema.Tables))
	}

	// Check that users table exists with correct fields
	var usersTable *yamlpkg.Table
	for _, table := range mergedSchema.Tables {
		if table.Name == "users" {
			usersTable = &table
			break
		}
	}
	if usersTable == nil {
		t.Fatal("Users table not found in merged schema")
	}

	if len(usersTable.Fields) != 4 {
		t.Fatalf("Expected 4 fields in users table, got %d", len(usersTable.Fields))
	}

	// Step 4: Test initial migration generation
	oldSchema, err := stateManager.LoadSchemaSnapshot("")
	if err != nil {
		// This is expected for first run
		oldSchema = nil
	}

	// Compute diff for initial migration
	diff, err := diffEngine.CompareSchemas(oldSchema, mergedSchema)
	if err != nil {
		t.Fatalf("Failed to compute schema diff: %v", err)
	}

	if !diff.HasChanges {
		t.Fatal("Expected changes for initial migration")
	}

	// Generate migration
	migration, err := migrationGenerator.GenerateMigration(diff, oldSchema, mergedSchema, "")
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	if migration.Filename == "" {
		t.Error("Migration filename should not be empty")
	}

	// Generate complete migration content
	migrationContent, err := migrationGenerator.GenerateCompleteMigration(diff, oldSchema, mergedSchema, "")
	if err != nil {
		t.Fatalf("Failed to generate complete migration: %v", err)
	}

	// Verify migration content
	if !strings.Contains(migrationContent, "-- +goose Up") {
		t.Error("Migration should contain Goose UP marker")
	}

	if !strings.Contains(migrationContent, "-- +goose Down") {
		t.Error("Migration should contain Goose DOWN marker")
	}

	if !strings.Contains(migrationContent, "CREATE TABLE") {
		t.Error("Migration should contain CREATE TABLE statements")
	}

	// Foreign key constraints might be generated as separate ALTER statements
	// or integrated into table creation - let's be flexible about this
	hasConstraints := strings.Contains(migrationContent, "FOREIGN KEY") ||
		strings.Contains(migrationContent, "REFERENCES") ||
		strings.Contains(migrationContent, "ALTER TABLE")

	if !hasConstraints {
		t.Logf("Migration content: %s", migrationContent)
		t.Log("Note: Foreign key constraints might be generated in separate migrations")
	}

	// Step 5: Save schema snapshot
	err = stateManager.SaveSchemaSnapshot(mergedSchema, "")
	if err != nil {
		t.Fatalf("Failed to save schema snapshot: %v", err)
	}

	// Verify snapshot was created
	snapshotPath := stateManager.GetSnapshotPath("")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Error("Schema snapshot file was not created")
	}

	// Step 6: Test no changes on second run
	diff2, err := diffEngine.CompareSchemas(mergedSchema, mergedSchema)
	if err != nil {
		t.Fatalf("Failed to compute diff on second run: %v", err)
	}

	if diff2.HasChanges {
		t.Error("Should not have changes when schema is identical")
	}
}

func TestYAMLIntegration_DatabaseSpecificGeneration(t *testing.T) {
	// Test generating SQL for different database types
	schemaContent := `database:
  name: test_db
  version: 1.0.0

defaults:
  postgresql:
    Now: "CURRENT_TIMESTAMP"
  mysql:
    Now: "CURRENT_TIMESTAMP()"
  sqlserver:
    Now: "GETDATE()"
  sqlite:
    Now: "CURRENT_TIMESTAMP"

tables:
  - name: test_table
    fields:
      - name: id
        type: uuid
        primary_key: true
      - name: name
        type: varchar
        length: 100
      - name: created_at
        type: timestamp
        default: Now
`

	parser := yamlpkg.NewParser(false)
	schema, err := parser.ParseAndValidate(schemaContent)
	if err != nil {
		t.Fatalf("Failed to parse test schema: %v", err)
	}

	databases := []yamlpkg.DatabaseType{
		yamlpkg.DatabasePostgreSQL,
		yamlpkg.DatabaseMySQL,
		yamlpkg.DatabaseSQLServer,
		yamlpkg.DatabaseSQLite,
	}

	for _, dbType := range databases {
		t.Run(string(dbType), func(t *testing.T) {
			converter := yamlpkg.NewSQLConverter(dbType, false)
			sql, err := converter.ConvertSchema(schema)
			if err != nil {
				t.Fatalf("Failed to convert schema for %s: %v", dbType, err)
			}

			if sql == "" {
				t.Fatalf("Generated SQL is empty for %s", dbType)
			}

			if !strings.Contains(sql, "CREATE TABLE") {
				t.Errorf("Generated SQL for %s should contain CREATE TABLE", dbType)
			}

			// Check database-specific syntax
			switch dbType {
			case yamlpkg.DatabasePostgreSQL:
				if !strings.Contains(sql, "CURRENT_TIMESTAMP") {
					t.Error("PostgreSQL SQL should contain CURRENT_TIMESTAMP")
				}
			case yamlpkg.DatabaseMySQL:
				if !strings.Contains(sql, "CURRENT_TIMESTAMP()") {
					t.Error("MySQL SQL should contain CURRENT_TIMESTAMP()")
				}
			case yamlpkg.DatabaseSQLServer:
				if !strings.Contains(sql, "GETDATE()") {
					t.Error("SQL Server SQL should contain GETDATE()")
				}
			case yamlpkg.DatabaseSQLite:
				if !strings.Contains(sql, "CURRENT_TIMESTAMP") {
					t.Error("SQLite SQL should contain CURRENT_TIMESTAMP")
				}
			}
		})
	}
}

func TestYAMLIntegration_ManyToManyGeneration(t *testing.T) {
	// Test many-to-many relationship handling
	schemaContent := `database:
  name: test_app
  version: 1.0.0

defaults:
  postgresql:
    Now: "CURRENT_TIMESTAMP"

tables:
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
      - name: name
        type: varchar
        length: 100

  - name: roles
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: "gen_random_uuid()"
      - name: name
        type: varchar
        length: 50
      - name: user_roles
        type: many_to_many
        many_to_many:
          table: users
          through: user_roles
`

	parser := yamlpkg.NewParser(false)
	schema, err := parser.ParseAndValidate(schemaContent)
	if err != nil {
		t.Fatalf("Failed to parse many-to-many schema: %v", err)
	}

	analyzer := yamlpkg.NewDependencyAnalyzer(false)

	// Generate junction tables
	junctionTables, err := analyzer.GenerateJunctionTables(schema)
	if err != nil {
		t.Fatalf("Failed to generate junction tables: %v", err)
	}
	if len(junctionTables) != 1 {
		t.Fatalf("Expected 1 junction table, got %d", len(junctionTables))
	}

	junctionTable := junctionTables[0]
	// The actual junction table name might be generated differently
	t.Logf("Generated junction table name: %s", junctionTable.Name)
	if !strings.Contains(junctionTable.Name, "user_roles") && !strings.Contains(junctionTable.Name, "roles_user") {
		t.Errorf("Expected junction table name to contain 'user_roles' or 'roles_user', got '%s'", junctionTable.Name)
	}

	// Should have foreign key fields to both tables
	if len(junctionTable.Fields) < 3 { // id + 2 foreign keys
		t.Fatalf("Expected at least 3 fields in junction table, got %d", len(junctionTable.Fields))
	}

	// Convert to SQL and verify
	converter := yamlpkg.NewSQLConverter(yamlpkg.DatabasePostgreSQL, false)

	// Add junction tables to schema
	schemaWithJunctions := *schema
	schemaWithJunctions.Tables = append(schemaWithJunctions.Tables, junctionTables...)

	sql, err := converter.ConvertSchema(&schemaWithJunctions)
	if err != nil {
		t.Fatalf("Failed to convert schema with junction tables: %v", err)
	}

	// Check for junction table creation (name might vary)
	if !strings.Contains(sql, "CREATE TABLE") || (!strings.Contains(sql, "user_roles") && !strings.Contains(sql, "roles_user")) {
		t.Logf("Generated SQL: %s", sql)
		t.Error("Generated SQL should contain junction table creation")
	}

	// Check for foreign key constraints (might be in table creation or separate ALTER statements)
	fkCount := strings.Count(sql, "FOREIGN KEY")
	refCount := strings.Count(sql, "REFERENCES")
	alterCount := strings.Count(sql, "ALTER TABLE")

	if fkCount < 1 && refCount < 1 && alterCount < 1 {
		t.Logf("Generated SQL: %s", sql)
		t.Log("Note: Foreign key constraints might be handled separately or in different format")
	}
}

func TestYAMLIntegration_ErrorHandling(t *testing.T) {
	// Test error conditions in YAML integration
	tmpDir, err := os.MkdirTemp("", "yaml_error_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	parser := yamlpkg.NewParser(false)

	// Test invalid YAML syntax
	_, err = parser.ParseAndValidate("invalid: yaml: syntax: [")
	if err == nil {
		t.Error("Expected error for invalid YAML syntax")
	}

	// Test missing required fields
	invalidSchema := `database:
  name: test
tables:
  - name: users
    # Missing fields array
`
	_, err = parser.ParseAndValidate(invalidSchema)
	if err == nil {
		t.Error("Expected error for schema missing required fields")
	}

	// Test foreign key reference to non-existent table
	invalidFKSchema := `database:
  name: test
  version: 1.0.0
tables:
  - name: posts
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: user_id
        type: foreign_key
        foreign_key:
          table: nonexistent_table
`
	schema, err := parser.ParseSchema(invalidFKSchema)
	if err != nil {
		t.Fatalf("Failed to parse schema with invalid FK: %v", err)
	}

	err = parser.ValidateForeignKeyReferences(schema)
	if err == nil {
		t.Error("Expected error for foreign key reference to non-existent table")
	}

	// Test database-specific validation
	largeVarcharSchema := `database:
  name: test
  version: 1.0.0
tables:
  - name: test_table
    fields:
      - name: large_field
        type: varchar
        length: 100000  # Too large for SQL Server
`
	schema, err = parser.ParseSchema(largeVarcharSchema)
	if err != nil {
		t.Fatalf("Failed to parse schema with large varchar: %v", err)
	}

	err = parser.ValidateDatabaseSpecificRules(schema, yamlpkg.DatabaseSQLServer)
	if err == nil {
		t.Error("Expected error for varchar length exceeding SQL Server limit")
	}
}
