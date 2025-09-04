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
	"strings"
	"testing"
)

func TestGenerateMigration(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	oldSchema := &Schema{
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

	diff := &SchemaDiff{
		Changes: []Change{
			{
				Type:      ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "email",
				NewValue:  Field{Name: "email", Type: "varchar", Length: 255},
			},
		},
		HasChanges: true,
	}

	migration, err := generator.GenerateMigration(diff, oldSchema, newSchema, "")
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	// Validate migration structure
	if migration.Filename == "" {
		t.Error("Expected non-empty filename")
	}

	if migration.Description == "" {
		t.Error("Expected non-empty description")
	}

	if migration.UpSQL == "" {
		t.Error("Expected UP SQL")
	}

	if migration.DownSQL == "" {
		t.Error("Expected DOWN SQL")
	}

	// Check for Goose annotations
	if !strings.Contains(migration.UpSQL, "-- +goose Up") {
		t.Error("UP migration missing Goose Up annotation")
	}

	if !strings.Contains(migration.DownSQL, "-- +goose Down") {
		t.Error("DOWN migration missing Goose Down annotation")
	}

	if !strings.Contains(migration.UpSQL, "ADD COLUMN") {
		t.Error("Expected ADD COLUMN in UP migration")
	}

	if !strings.Contains(migration.DownSQL, "DROP COLUMN") {
		t.Error("Expected DROP COLUMN in DOWN migration")
	}
}

func TestGenerateMigrationWithCustomName(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	diff := &SchemaDiff{
		Changes: []Change{
			{
				Type:      ChangeTypeTableAdded,
				TableName: "posts",
				NewValue:  Table{Name: "posts"},
			},
		},
		HasChanges: true,
	}

	customName := "add_posts_table"
	migration, err := generator.GenerateMigration(diff, nil, &Schema{}, customName)
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	if migration.Description != customName {
		t.Errorf("Expected description '%s', got '%s'", customName, migration.Description)
	}

	if !strings.Contains(migration.Filename, customName) {
		t.Errorf("Expected filename to contain '%s', got '%s'", customName, migration.Filename)
	}
}

func TestGenerateCompleteMigration(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	oldSchema := &Schema{
		Tables: []Table{
			{
				Name: "old_table",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
				},
			},
		},
	}

	diff := &SchemaDiff{
		Changes: []Change{
			{
				Type:        ChangeTypeTableRemoved,
				TableName:   "old_table",
				OldValue:    oldSchema.Tables[0],
				Destructive: true,
			},
		},
		HasChanges:    true,
		IsDestructive: true,
	}

	content, err := generator.GenerateCompleteMigration(diff, oldSchema, &Schema{}, "")
	if err != nil {
		t.Fatalf("Failed to generate complete migration: %v", err)
	}

	// Check for header comments
	if !strings.Contains(content, "-- Migration:") {
		t.Error("Expected migration header comment")
	}

	if !strings.Contains(content, "-- Generated:") {
		t.Error("Expected generated timestamp comment")
	}

	if !strings.Contains(content, "-- Database: postgresql") {
		t.Error("Expected database type comment")
	}

	if !strings.Contains(content, "WARNING: This migration contains destructive operations") {
		t.Error("Expected destructive warning for destructive migration")
	}

	// Check for Goose sections
	if !strings.Contains(content, "-- +goose Up") {
		t.Error("Expected Goose Up section")
	}

	if !strings.Contains(content, "-- +goose Down") {
		t.Error("Expected Goose Down section")
	}
}

func TestGenerateInitialMigration(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	schema := &Schema{
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
					{Name: "title", Type: "varchar", Length: 255},
				},
			},
		},
	}

	content, err := generator.GenerateInitialMigration(schema, "")
	if err != nil {
		t.Fatalf("Failed to generate initial migration: %v", err)
	}

	// Check for initial migration markers
	if !strings.Contains(content, "-- Initial schema migration") {
		t.Error("Expected initial schema migration comment")
	}

	// Check for CREATE TABLE statements
	if !strings.Contains(content, "CREATE TABLE") {
		t.Error("Expected CREATE TABLE statements")
	}

	// Check for both tables
	if !strings.Contains(content, "users") {
		t.Error("Expected users table")
	}

	if !strings.Contains(content, "posts") {
		t.Error("Expected posts table")
	}

	// Check for DROP TABLE statements in DOWN migration
	if !strings.Contains(content, "DROP TABLE") {
		t.Error("Expected DROP TABLE statements in DOWN migration")
	}

	// Check for REVIEW comments on destructive operations
	if !strings.Contains(content, "-- REVIEW") {
		t.Error("Expected REVIEW comments on DROP statements")
	}
}

func TestValidateMigration(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	// Valid migration
	validMigration := &Migration{
		Filename:    "20240101120000_test.sql",
		Description: "test migration",
		UpSQL: `-- +goose Up
-- +goose StatementBegin
CREATE TABLE test (id SERIAL);
-- +goose StatementEnd`,
		DownSQL: `-- +goose Down
-- +goose StatementBegin
DROP TABLE test;
-- +goose StatementEnd`,
	}

	err := generator.ValidateMigration(validMigration)
	if err != nil {
		t.Errorf("Valid migration failed validation: %v", err)
	}

	// Invalid migration - missing filename
	invalidMigration := &Migration{
		Description: "test",
		UpSQL:       "CREATE TABLE test (id SERIAL);",
	}

	err = generator.ValidateMigration(invalidMigration)
	if err == nil {
		t.Error("Expected validation error for missing filename")
	}

	// Invalid migration - missing Goose annotations
	invalidMigration2 := &Migration{
		Filename:    "test.sql",
		Description: "test",
		UpSQL:       "CREATE TABLE test (id SERIAL);",
	}

	err = generator.ValidateMigration(invalidMigration2)
	if err == nil {
		t.Error("Expected validation error for missing Goose annotations")
	}
}

func TestGetChangesSummary(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	// Test with multiple changes
	diff := &SchemaDiff{
		Changes: []Change{
			{Type: ChangeTypeTableAdded},
			{Type: ChangeTypeTableAdded},
			{Type: ChangeTypeFieldAdded},
			{Type: ChangeTypeFieldRemoved, Destructive: true},
		},
		HasChanges:    true,
		IsDestructive: true,
	}

	summary := generator.GetChangesSummary(diff)

	if !strings.Contains(summary, "2 table(s) added") {
		t.Error("Expected table addition count in summary")
	}

	if !strings.Contains(summary, "1 field(s) added") {
		t.Error("Expected field addition count in summary")
	}

	if !strings.Contains(summary, "1 field(s) removed") {
		t.Error("Expected field removal count in summary")
	}

	if !strings.Contains(summary, "destructive changes") {
		t.Error("Expected destructive changes warning in summary")
	}

	// Test with no changes
	emptyDiff := &SchemaDiff{HasChanges: false}
	summary = generator.GetChangesSummary(emptyDiff)

	if summary != "No changes" {
		t.Errorf("Expected 'No changes', got '%s'", summary)
	}
}

func TestGenerateFilename(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	tests := []struct {
		description string
		expected    string // just check the suffix
	}{
		{"add users table", "_add_users_table.sql"},
		{"modify-field-type", "_modify_field_type.sql"},
		{"Remove Old Data", "_remove_old_data.sql"},
	}

	for _, test := range tests {
		filename := generator.GenerateFilename(test.description)

		if !strings.HasSuffix(filename, test.expected) {
			t.Errorf("Expected filename to end with '%s', got '%s'", test.expected, filename)
		}

		// Check that it starts with a timestamp
		if len(filename) < 14 {
			t.Errorf("Expected timestamp prefix, got short filename: '%s'", filename)
		}
	}
}

func TestGetMigrationStats(t *testing.T) {
	generator := NewMigrationGenerator(DatabasePostgreSQL, false)

	migration := &Migration{
		Filename:    "test.sql",
		Description: "test migration",
		UpSQL:       "CREATE TABLE test (\n  id SERIAL\n);",
		DownSQL:     "DROP TABLE test;",
		Destructive: true,
	}

	stats := generator.GetMigrationStats(migration)

	if stats["filename"] != "test.sql" {
		t.Error("Expected filename in stats")
	}

	if stats["destructive"] != true {
		t.Error("Expected destructive flag in stats")
	}

	if stats["has_up_sql"] != true {
		t.Error("Expected has_up_sql flag")
	}

	if stats["has_down_sql"] != true {
		t.Error("Expected has_down_sql flag")
	}

	if stats["up_sql_lines"] != 3 {
		t.Errorf("Expected 3 lines in UP SQL, got %v", stats["up_sql_lines"])
	}

	if stats["down_sql_lines"] != 1 {
		t.Errorf("Expected 1 line in DOWN SQL, got %v", stats["down_sql_lines"])
	}
}
