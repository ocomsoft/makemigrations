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

func TestConvertSchema(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

	schema := &Schema{
		Database: Database{Name: "test", Version: "1.0"},
		Defaults: Defaults{
			PostgreSQL: map[string]string{
				"Now": "CURRENT_TIMESTAMP",
			},
		},
		Tables: []Table{
			{
				Name: "users",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
					{Name: "created_at", Type: "timestamp", Default: "Now"},
				},
			},
		},
	}

	sql, err := converter.ConvertSchema(schema)
	if err != nil {
		t.Fatalf("Failed to convert schema: %v", err)
	}

	// Check that SQL contains expected elements
	if !strings.Contains(sql, "CREATE TABLE") {
		t.Error("Expected CREATE TABLE statement")
	}

	if !strings.Contains(sql, "users") {
		t.Error("Expected users table")
	}

	if !strings.Contains(sql, "SERIAL") {
		t.Error("Expected SERIAL type for PostgreSQL")
	}

	if !strings.Contains(sql, "VARCHAR(255)") {
		t.Error("Expected VARCHAR(255)")
	}

	if !strings.Contains(sql, "PRIMARY KEY") {
		t.Error("Expected PRIMARY KEY constraint")
	}
}

func TestConvertFieldType(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

	tests := []struct {
		field    Field
		expected string
	}{
		{
			field:    Field{Type: "varchar", Length: 255},
			expected: "VARCHAR(255)",
		},
		{
			field:    Field{Type: "integer"},
			expected: "INTEGER",
		},
		{
			field:    Field{Type: "serial"},
			expected: "SERIAL",
		},
		{
			field:    Field{Type: "boolean"},
			expected: "BOOLEAN",
		},
		{
			field:    Field{Type: "timestamp"},
			expected: "TIMESTAMPTZ",
		},
		{
			field:    Field{Type: "decimal", Precision: 10, Scale: 2},
			expected: "DECIMAL(10,2)",
		},
	}

	for _, test := range tests {
		result, err := converter.convertFieldType(&test.field)
		if err != nil {
			t.Errorf("Failed to convert field type %s: %v", test.field.Type, err)
			continue
		}

		if result != test.expected {
			t.Errorf("Expected %s, got %s for field type %s", test.expected, result, test.field.Type)
		}
	}
}

func TestConvertFieldTypeMySQL(t *testing.T) {
	converter := NewSQLConverter(DatabaseMySQL, false)

	tests := []struct {
		field    Field
		expected string
	}{
		{
			field:    Field{Type: "serial"},
			expected: "INT AUTO_INCREMENT",
		},
		{
			field:    Field{Type: "boolean"},
			expected: "BOOLEAN",
		},
		{
			field:    Field{Type: "uuid"},
			expected: "CHAR(36)",
		},
	}

	for _, test := range tests {
		result, err := converter.convertFieldType(&test.field)
		if err != nil {
			t.Errorf("Failed to convert field type %s: %v", test.field.Type, err)
			continue
		}

		if result != test.expected {
			t.Errorf("Expected %s, got %s for field type %s", test.expected, result, test.field.Type)
		}
	}
}

func TestConvertDefaultValue(t *testing.T) {
	schema := &Schema{
		Defaults: Defaults{
			PostgreSQL: map[string]string{
				"Now":   "CURRENT_TIMESTAMP",
				"Today": "CURRENT_DATE",
				"null":  "null",
			},
		},
	}

	converter := NewSQLConverter(DatabasePostgreSQL, false)

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Now",
			expected: "CURRENT_TIMESTAMP",
		},
		{
			input:    "Today",
			expected: "CURRENT_DATE",
		},
		{
			input:    "null",
			expected: "null",
		},
		{
			input:    "custom_value",
			expected: "'custom_value'",
		},
	}

	for _, test := range tests {
		result, err := converter.convertDefaultValue(schema, test.input)
		if err != nil {
			t.Errorf("Failed to convert default value %s: %v", test.input, err)
			continue
		}

		if result != test.expected {
			t.Errorf("Expected %s, got %s for default value %s", test.expected, result, test.input)
		}
	}
}

func TestConvertDiffToSQL(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

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

	upSQL, downSQL, err := converter.ConvertDiffToSQL(diff, oldSchema, newSchema)
	if err != nil {
		t.Fatalf("Failed to convert diff to SQL: %v", err)
	}

	// Check UP migration
	if !strings.Contains(upSQL, "ALTER TABLE") {
		t.Error("Expected ALTER TABLE in UP migration")
	}

	if !strings.Contains(upSQL, "ADD COLUMN") {
		t.Error("Expected ADD COLUMN in UP migration")
	}

	if !strings.Contains(upSQL, "email") {
		t.Error("Expected email field in UP migration")
	}

	// Check DOWN migration
	if !strings.Contains(downSQL, "ALTER TABLE") {
		t.Error("Expected ALTER TABLE in DOWN migration")
	}

	if !strings.Contains(downSQL, "DROP COLUMN") {
		t.Error("Expected DROP COLUMN in DOWN migration")
	}
}

func TestQuoteName(t *testing.T) {
	tests := []struct {
		dbType   DatabaseType
		name     string
		expected string
	}{
		{DatabasePostgreSQL, "users", `"users"`},
		{DatabaseMySQL, "users", "`users`"},
		{DatabaseSQLServer, "users", "[users]"},
		{DatabaseSQLite, "users", `"users"`},
	}

	for _, test := range tests {
		converter := NewSQLConverter(test.dbType, false)
		result := converter.quoteName(test.name)
		if result != test.expected {
			t.Errorf("Expected %s, got %s for database %s", test.expected, result, test.dbType)
		}
	}
}

func TestGenerateForeignKeyConstraints(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

	schema := &Schema{
		Tables: []Table{
			{
				Name: "posts",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{
						Name: "user_id",
						Type: "foreign_key",
						ForeignKey: &ForeignKey{
							Table:    "users",
							OnDelete: "CASCADE",
						},
					},
				},
			},
		},
	}

	sql := converter.generateForeignKeyConstraints(schema, []Table{})

	if !strings.Contains(sql, "ALTER TABLE") {
		t.Error("Expected ALTER TABLE statement")
	}

	if !strings.Contains(sql, "ADD CONSTRAINT") {
		t.Error("Expected ADD CONSTRAINT")
	}

	if !strings.Contains(sql, "FOREIGN KEY") {
		t.Error("Expected FOREIGN KEY")
	}

	if !strings.Contains(sql, "REFERENCES") {
		t.Error("Expected REFERENCES")
	}

	if !strings.Contains(sql, "CASCADE") {
		t.Error("Expected CASCADE")
	}
}

func TestConvertTableWithForeignKey(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

	schema := &Schema{
		Defaults: Defaults{
			PostgreSQL: map[string]string{},
		},
		Tables: []Table{
			{
				Name: "posts",
				Fields: []Field{
					{Name: "id", Type: "serial", PrimaryKey: true},
					{Name: "title", Type: "varchar", Length: 255},
					{
						Name: "user_id",
						Type: "foreign_key",
						ForeignKey: &ForeignKey{
							Table: "users",
						},
					},
				},
			},
		},
	}

	table := &schema.Tables[0]
	sql, err := converter.ConvertTable(schema, table)
	if err != nil {
		t.Fatalf("Failed to convert table: %v", err)
	}

	// Foreign key fields should create columns in the table definition
	if !strings.Contains(sql, "user_id") {
		t.Error("Foreign key fields should appear in table definition")
	}

	if !strings.Contains(sql, "title") {
		t.Error("Expected title field in table definition")
	}
}
