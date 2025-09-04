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

func TestParseSchema(t *testing.T) {
	parser := NewParser(false)

	yamlContent := `
database:
  name: test_app
  version: 1.0.0

defaults:
  postgresql:
    Now: "CURRENT_TIMESTAMP"
    "": "''"
    "null": "null"

tables:
  - name: users
    fields:
      - name: id
        type: serial
        primary_key: true
        nullable: false
      - name: email
        type: varchar
        length: 255
        nullable: false
      - name: created_at
        type: timestamp
        default: Now
`

	schema, err := parser.ParseSchema(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	// Validate database
	if schema.Database.Name != "test_app" {
		t.Errorf("Expected database name 'test_app', got '%s'", schema.Database.Name)
	}

	// Validate tables
	if len(schema.Tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(schema.Tables))
	}

	table := schema.Tables[0]
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}

	// Validate fields
	if len(table.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(table.Fields))
	}

	idField := table.Fields[0]
	if idField.Name != "id" || idField.Type != "serial" || !idField.PrimaryKey {
		t.Errorf("Invalid id field: %+v", idField)
	}

	emailField := table.Fields[1]
	if emailField.Name != "email" || emailField.Type != "varchar" || emailField.Length != 255 {
		t.Errorf("Invalid email field: %+v", emailField)
	}
}

func TestValidateForeignKeyReferences(t *testing.T) {
	parser := NewParser(false)

	yamlContent := `
database:
  name: test_app
  version: 1.0.0

tables:
  - name: users
    fields:
      - name: id
        type: serial
        primary_key: true
  - name: posts
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: user_id
        type: foreign_key
        foreign_key:
          table: users
          on_delete: CASCADE
`

	schema, err := parser.ParseSchema(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	err = parser.ValidateForeignKeyReferences(schema)
	if err != nil {
		t.Fatalf("Failed to validate foreign key references: %v", err)
	}
}

func TestInvalidForeignKeyReference(t *testing.T) {
	parser := NewParser(false)

	yamlContent := `
database:
  name: test_app
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
          on_delete: CASCADE
`

	schema, err := parser.ParseSchema(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	err = parser.ValidateForeignKeyReferences(schema)
	if err == nil {
		t.Fatal("Expected validation error for invalid foreign key reference")
	}
}

func TestValidateDatabaseSpecificRules(t *testing.T) {
	parser := NewParser(false)

	tests := []struct {
		name          string
		schema        string
		databaseType  DatabaseType
		expectError   bool
		errorContains string
	}{
		{
			name: "valid varchar length for MySQL",
			schema: `
database:
  name: test
  version: 1.0.0
tables:
  - name: users
    fields:
      - name: name
        type: varchar
        length: 255
`,
			databaseType: DatabaseMySQL,
			expectError:  false,
		},
		{
			name: "invalid varchar length for MySQL",
			schema: `
database:
  name: test
  version: 1.0.0
tables:
  - name: users
    fields:
      - name: name
        type: varchar
        length: 70000
`,
			databaseType:  DatabaseMySQL,
			expectError:   true,
			errorContains: "exceeds MySQL limit",
		},
		{
			name: "valid decimal precision for PostgreSQL",
			schema: `
database:
  name: test
  version: 1.0.0
tables:
  - name: products
    fields:
      - name: price
        type: decimal
        precision: 10
        scale: 2
`,
			databaseType: DatabasePostgreSQL,
			expectError:  false,
		},
		{
			name: "invalid decimal precision for SQL Server",
			schema: `
database:
  name: test
  version: 1.0.0
tables:
  - name: products
    fields:
      - name: price
        type: decimal
        precision: 50
        scale: 2
`,
			databaseType:  DatabaseSQLServer,
			expectError:   true,
			errorContains: "exceeds SQL Server limit",
		},
		{
			name: "invalid decimal precision for PostgreSQL",
			schema: `
database:
  name: test
  version: 1.0.0
tables:
  - name: products
    fields:
      - name: price
        type: decimal
        precision: 1500
        scale: 2
`,
			databaseType:  DatabasePostgreSQL,
			expectError:   true,
			errorContains: "exceeds PostgreSQL limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := parser.ParseSchema(tt.schema)
			if err != nil {
				t.Fatalf("Failed to parse schema: %v", err)
			}

			err = parser.ValidateDatabaseSpecificRules(schema, tt.databaseType)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected validation error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateComprehensive(t *testing.T) {
	parser := NewParser(false)

	// Test schema with multiple validation issues
	yamlContent := `
database:
  name: test
  version: 1.0.0
tables:
  - name: users
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: name
        type: varchar
        length: 100000  # Too long for SQL Server
      - name: invalid_fk
        type: foreign_key
        foreign_key:
          table: nonexistent_table
  - name: users  # Duplicate table name
    fields:
      - name: id
        type: serial
`

	schema, err := parser.ParseSchema(yamlContent)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	validationErrors := parser.ValidateComprehensive(schema, DatabaseSQLServer)

	if len(validationErrors) == 0 {
		t.Fatal("Expected validation errors but got none")
	}

	// Check that we have different types of errors
	errorTypes := make(map[string]bool)
	for _, validationErr := range validationErrors {
		errorTypes[validationErr.Type] = true
	}

	expectedTypes := []string{"structure", "foreign_key", "database_specific"}
	for _, expectedType := range expectedTypes {
		if !errorTypes[expectedType] {
			t.Errorf("Expected validation error of type '%s' but didn't find one", expectedType)
		}
	}
}

func TestFormatValidationErrors(t *testing.T) {
	parser := NewParser(false)

	errors := []ValidationError{
		{
			Type:    "structure",
			Table:   "users",
			Message: "Duplicate table name",
		},
		{
			Type:     "database_specific",
			Table:    "products",
			Field:    "price",
			Message:  "Decimal precision too high",
			Severity: "error",
		},
	}

	formatted := parser.FormatValidationErrors(errors)

	if !strings.Contains(formatted, "2 error(s)") {
		t.Error("Expected error count in formatted output")
	}

	if !strings.Contains(formatted, "STRUCTURE") {
		t.Error("Expected STRUCTURE error type in formatted output")
	}

	if !strings.Contains(formatted, "DATABASE_SPECIFIC") {
		t.Error("Expected DATABASE_SPECIFIC error type in formatted output")
	}

	if !strings.Contains(formatted, "Table: users") {
		t.Error("Expected table name in formatted output")
	}

	if !strings.Contains(formatted, "Field: price") {
		t.Error("Expected field name in formatted output")
	}
}

func TestParseAndValidate(t *testing.T) {
	parser := NewParser(false)

	// Test valid schema
	validSchema := `
database:
  name: test
  version: 1.0.0
tables:
  - name: users
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: email
        type: varchar
        length: 255
`

	schema, err := parser.ParseAndValidate(validSchema)
	if err != nil {
		t.Fatalf("Expected valid schema to pass validation: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected valid schema object")
	}

	// Test invalid schema
	invalidSchema := `
database:
  name: test
  version: 1.0.0
tables:
  - name: users
    fields:
      - name: name
        type: varchar
        # Missing required length
`

	_, err = parser.ParseAndValidate(invalidSchema)
	if err == nil {
		t.Fatal("Expected invalid schema to fail validation")
	}
}
