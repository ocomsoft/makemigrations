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

func TestSafeTypeChangeSQL(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		tableName    string
		oldField     Field
		newField     Field
		expectedUp   []string // Array of expected statements
		expectedDown []string // Array of expected statements
		shouldError  bool
	}{
		{
			name:         "PostgreSQL varchar to text safe change",
			databaseType: DatabasePostgreSQL,
			tableName:    "users",
			oldField: Field{
				Name:   "description",
				Type:   "varchar",
				Length: 255,
			},
			newField: Field{
				Name: "description",
				Type: "text",
			},
			expectedUp: []string{
				`ALTER TABLE "users" ADD COLUMN "description_temp_migration" TEXT;`,
				`UPDATE "users" SET "description_temp_migration" = "description"::TEXT;`,
				`ALTER TABLE "users" DROP COLUMN "description";`,
				`ALTER TABLE "users" RENAME COLUMN "description_temp_migration" TO "description";`,
			},
			expectedDown: []string{
				`ALTER TABLE "users" ADD COLUMN "description_temp_migration" TEXT;`,
				`UPDATE "users" SET "description_temp_migration" = "description"::VARCHAR(255);`,
				`ALTER TABLE "users" DROP COLUMN "description";`,
				`ALTER TABLE "users" RENAME COLUMN "description_temp_migration" TO "description";`,
			},
		},
		{
			name:         "PostgreSQL integer to bigint safe change",
			databaseType: DatabasePostgreSQL,
			tableName:    "analytics",
			oldField: Field{
				Name: "count",
				Type: "integer",
			},
			newField: Field{
				Name: "count",
				Type: "bigint",
			},
			expectedUp: []string{
				`ALTER TABLE "analytics" ADD COLUMN "count_temp_migration" BIGINT;`,
				`UPDATE "analytics" SET "count_temp_migration" = "count"::BIGINT;`,
				`ALTER TABLE "analytics" DROP COLUMN "count";`,
				`ALTER TABLE "analytics" RENAME COLUMN "count_temp_migration" TO "count";`,
			},
			expectedDown: []string{
				`ALTER TABLE "analytics" ADD COLUMN "count_temp_migration" BIGINT;`,
				`UPDATE "analytics" SET "count_temp_migration" = "count"::INTEGER;`,
				`ALTER TABLE "analytics" DROP COLUMN "count";`,
				`ALTER TABLE "analytics" RENAME COLUMN "count_temp_migration" TO "count";`,
			},
		},
		{
			name:         "MySQL varchar length increase safe change",
			databaseType: DatabaseMySQL,
			tableName:    "products",
			oldField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 100,
			},
			newField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 500,
			},
			expectedUp: []string{
				"ALTER TABLE `products` ADD COLUMN `name_temp_migration` VARCHAR(500);",
				"UPDATE `products` SET `name_temp_migration` = CAST(`name` AS VARCHAR(500));",
				"ALTER TABLE `products` DROP COLUMN `name`;",
				"ALTER TABLE `products` CHANGE `name_temp_migration` `name` VARCHAR(500);",
			},
			expectedDown: []string{
				"ALTER TABLE `products` ADD COLUMN `name_temp_migration` VARCHAR(100);",
				"UPDATE `products` SET `name_temp_migration` = CAST(`name` AS VARCHAR(100));",
				"ALTER TABLE `products` DROP COLUMN `name`;",
				"ALTER TABLE `products` CHANGE `name_temp_migration` `name` VARCHAR(100);",
			},
		},
		{
			name:         "SQL Server decimal precision change",
			databaseType: DatabaseSQLServer,
			tableName:    "financial_data",
			oldField: Field{
				Name:      "amount",
				Type:      "decimal",
				Precision: 10,
				Scale:     2,
			},
			newField: Field{
				Name:      "amount",
				Type:      "decimal",
				Precision: 15,
				Scale:     4,
			},
			expectedUp: []string{
				"ALTER TABLE [financial_data] ADD [amount_temp_migration] DECIMAL(15,4);",
				"UPDATE [financial_data] SET [amount_temp_migration] = CAST([amount] AS DECIMAL(15,4));",
				"ALTER TABLE [financial_data] DROP COLUMN [amount];",
				"EXEC sp_rename 'financial_data.amount_temp_migration', 'amount', 'COLUMN';",
			},
			expectedDown: []string{
				"ALTER TABLE [financial_data] ADD [amount_temp_migration] DECIMAL(10,2);",
				"UPDATE [financial_data] SET [amount_temp_migration] = CAST([amount] AS DECIMAL(10,2));",
				"ALTER TABLE [financial_data] DROP COLUMN [amount];",
				"EXEC sp_rename 'financial_data.amount_temp_migration', 'amount', 'COLUMN';",
			},
		},
		{
			name:         "SQLite type change (unsupported)",
			databaseType: DatabaseSQLite,
			tableName:    "logs",
			oldField: Field{
				Name:   "message",
				Type:   "varchar",
				Length: 255,
			},
			newField: Field{
				Name: "message",
				Type: "text",
			},
			expectedUp: []string{
				"-- SQLite doesn't support safe column type changes. Manual table recreation required for logs.message",
			},
			expectedDown: []string{
				"-- SQLite doesn't support safe column type changes. Manual table recreation required for logs.message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal schemas for testing
			oldSchema := &Schema{
				Database: Database{Name: "test", Version: "1.0.0"},
				Tables: []Table{
					{
						Name:   tt.tableName,
						Fields: []Field{tt.oldField},
					},
				},
				Defaults: Defaults{
					PostgreSQL: map[string]string{},
					MySQL:      map[string]string{},
					SQLServer:  map[string]string{},
					SQLite:     map[string]string{},
				},
			}

			newSchema := &Schema{
				Database: Database{Name: "test", Version: "1.0.0"},
				Tables: []Table{
					{
						Name:   tt.tableName,
						Fields: []Field{tt.newField},
					},
				},
				Defaults: Defaults{
					PostgreSQL: map[string]string{},
					MySQL:      map[string]string{},
					SQLServer:  map[string]string{},
					SQLite:     map[string]string{},
				},
			}

			converter := NewSQLConverterWithOptions(tt.databaseType, false, true) // Enable safe type changes
			upSQL, downSQL, err := converter.generateSafeTypeChangeSQL(tt.tableName, &tt.oldField, &tt.newField, oldSchema, newSchema)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Split multi-line SQL into individual statements for comparison
			upStatements := strings.Split(strings.TrimSpace(upSQL), "\n")
			downStatements := strings.Split(strings.TrimSpace(downSQL), "\n")

			if len(upStatements) != len(tt.expectedUp) {
				t.Errorf("Up SQL statement count mismatch.\nExpected: %d statements\nGot: %d statements\nSQL: %q", len(tt.expectedUp), len(upStatements), upSQL)
				return
			}

			if len(downStatements) != len(tt.expectedDown) {
				t.Errorf("Down SQL statement count mismatch.\nExpected: %d statements\nGot: %d statements\nSQL: %q", len(tt.expectedDown), len(downStatements), downSQL)
				return
			}

			// Compare each statement
			for i, expected := range tt.expectedUp {
				if strings.TrimSpace(upStatements[i]) != expected {
					t.Errorf("Up SQL statement %d mismatch.\nExpected: %q\nGot:      %q", i+1, expected, strings.TrimSpace(upStatements[i]))
				}
			}

			for i, expected := range tt.expectedDown {
				if strings.TrimSpace(downStatements[i]) != expected {
					t.Errorf("Down SQL statement %d mismatch.\nExpected: %q\nGot:      %q", i+1, expected, strings.TrimSpace(downStatements[i]))
				}
			}
		})
	}
}

func TestFieldModificationWithSafeTypeChanges(t *testing.T) {
	tests := []struct {
		name           string
		databaseType   DatabaseType
		tableName      string
		oldField       Field
		newField       Field
		safeTypeChange bool
		expectSafeSQL  bool
	}{
		{
			name:         "Safe type change enabled for type change",
			databaseType: DatabasePostgreSQL,
			tableName:    "users",
			oldField: Field{
				Name:   "status",
				Type:   "varchar",
				Length: 50,
			},
			newField: Field{
				Name:   "status",
				Type:   "text",
				Length: 0,
			},
			safeTypeChange: true,
			expectSafeSQL:  true,
		},
		{
			name:         "Safe type change disabled for type change",
			databaseType: DatabasePostgreSQL,
			tableName:    "users",
			oldField: Field{
				Name:   "status",
				Type:   "varchar",
				Length: 50,
			},
			newField: Field{
				Name:   "status",
				Type:   "text",
				Length: 0,
			},
			safeTypeChange: false,
			expectSafeSQL:  false,
		},
		{
			name:         "Safe type change enabled but no type change (only length)",
			databaseType: DatabasePostgreSQL,
			tableName:    "users",
			oldField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 100,
			},
			newField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 200,
			},
			safeTypeChange: true,
			expectSafeSQL:  false, // Should use standard approach for non-type changes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal schemas for testing
			oldSchema := &Schema{
				Database: Database{Name: "test", Version: "1.0.0"},
				Tables: []Table{
					{
						Name:   tt.tableName,
						Fields: []Field{tt.oldField},
					},
				},
				Defaults: Defaults{
					PostgreSQL: map[string]string{},
				},
			}

			newSchema := &Schema{
				Database: Database{Name: "test", Version: "1.0.0"},
				Tables: []Table{
					{
						Name:   tt.tableName,
						Fields: []Field{tt.newField},
					},
				},
				Defaults: Defaults{
					PostgreSQL: map[string]string{},
				},
			}

			converter := NewSQLConverterWithOptions(tt.databaseType, false, tt.safeTypeChange)
			upSQL, downSQL, err := converter.generateFieldModificationSQL(tt.tableName, &tt.oldField, &tt.newField, oldSchema, newSchema)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check if we got the expected type of SQL
			containsTempColumn := strings.Contains(upSQL, "_temp_migration")

			if tt.expectSafeSQL && !containsTempColumn {
				t.Errorf("Expected safe type change SQL with temporary column, but got standard SQL: %q", upSQL)
			}

			if !tt.expectSafeSQL && containsTempColumn {
				t.Errorf("Expected standard SQL, but got safe type change SQL with temporary column: %q", upSQL)
			}

			// Ensure we got some SQL (not empty)
			if strings.TrimSpace(upSQL) == "" {
				t.Errorf("Expected some SQL but got empty string")
			}

			if strings.TrimSpace(downSQL) == "" {
				t.Errorf("Expected some down SQL but got empty string")
			}
		})
	}
}

func TestTypeNameHelpers(t *testing.T) {
	converter := NewSQLConverter(DatabasePostgreSQL, false)

	postgresTests := []struct {
		field    Field
		expected string
	}{
		{Field{Type: "varchar", Length: 255}, "VARCHAR(255)"},
		{Field{Type: "varchar"}, "VARCHAR"},
		{Field{Type: "text"}, "TEXT"},
		{Field{Type: "integer"}, "INTEGER"},
		{Field{Type: "bigint"}, "BIGINT"},
		{Field{Type: "decimal", Precision: 10, Scale: 2}, "DECIMAL(10,2)"},
		{Field{Type: "decimal"}, "DECIMAL"},
		{Field{Type: "float"}, "REAL"},
		{Field{Type: "boolean"}, "BOOLEAN"},
		{Field{Type: "timestamp"}, "TIMESTAMPTZ"},
		{Field{Type: "uuid"}, "UUID"},
		{Field{Type: "jsonb"}, "JSONB"},
	}

	for _, tt := range postgresTests {
		result := converter.getPostgreSQLTypeName(&tt.field)
		if result != tt.expected {
			t.Errorf("PostgreSQL type name for %+v: expected %q, got %q", tt.field, tt.expected, result)
		}
	}

	mysqlConverter := NewSQLConverter(DatabaseMySQL, false)
	mysqlTests := []struct {
		field    Field
		expected string
	}{
		{Field{Type: "varchar", Length: 255}, "VARCHAR(255)"},
		{Field{Type: "varchar"}, "VARCHAR(255)"},
		{Field{Type: "integer"}, "INT"},
		{Field{Type: "boolean"}, "TINYINT(1)"},
		{Field{Type: "uuid"}, "CHAR(36)"},
		{Field{Type: "jsonb"}, "JSON"},
	}

	for _, tt := range mysqlTests {
		result := mysqlConverter.getMySQLTypeName(&tt.field)
		if result != tt.expected {
			t.Errorf("MySQL type name for %+v: expected %q, got %q", tt.field, tt.expected, result)
		}
	}
}
