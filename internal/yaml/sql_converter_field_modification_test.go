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

func TestGenerateFieldModificationSQL(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		tableName    string
		oldField     Field
		newField     Field
		expectedUp   string
		expectedDown string
		shouldError  bool
	}{
		{
			name:         "PostgreSQL VARCHAR length change",
			databaseType: DatabasePostgreSQL,
			tableName:    "users",
			oldField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 255,
			},
			newField: Field{
				Name:   "name",
				Type:   "varchar",
				Length: 500,
			},
			expectedUp:   `ALTER TABLE "users" ALTER COLUMN "name" TYPE VARCHAR(500);`,
			expectedDown: `ALTER TABLE "users" ALTER COLUMN "name" TYPE VARCHAR(255);`,
		},
		{
			name:         "PostgreSQL type change varchar to text",
			databaseType: DatabasePostgreSQL,
			tableName:    "posts",
			oldField: Field{
				Name:   "content",
				Type:   "varchar",
				Length: 1000,
			},
			newField: Field{
				Name: "content",
				Type: "text",
			},
			expectedUp:   `ALTER TABLE "posts" ALTER COLUMN "content" TYPE TEXT;`,
			expectedDown: `ALTER TABLE "posts" ALTER COLUMN "content" TYPE VARCHAR(1000);`,
		},
		{
			name:         "PostgreSQL nullable change true to false",
			databaseType: DatabasePostgreSQL,
			tableName:    "products",
			oldField: Field{
				Name:     "description",
				Type:     "varchar",
				Length:   255,
				Nullable: boolPtr(true),
			},
			newField: Field{
				Name:     "description",
				Type:     "varchar",
				Length:   255,
				Nullable: boolPtr(false),
			},
			expectedUp:   `-- WARNING: Setting "description" to NOT NULL without a default. Ensure no NULL values exist.` + "\n" + `-- UPDATE "products" SET "description" = <your_default_value> WHERE "description" IS NULL;` + "\n" + `ALTER TABLE "products" ALTER COLUMN "description" SET NOT NULL;`,
			expectedDown: `ALTER TABLE "products" ALTER COLUMN "description" DROP NOT NULL;`,
		},
		{
			name:         "PostgreSQL nullable change false to true",
			databaseType: DatabasePostgreSQL,
			tableName:    "orders",
			oldField: Field{
				Name:     "notes",
				Type:     "text",
				Nullable: boolPtr(false),
			},
			newField: Field{
				Name:     "notes",
				Type:     "text",
				Nullable: boolPtr(true),
			},
			expectedUp:   `ALTER TABLE "orders" ALTER COLUMN "notes" DROP NOT NULL;`,
			expectedDown: `ALTER TABLE "orders" ALTER COLUMN "notes" SET NOT NULL;`,
		},
		{
			name:         "PostgreSQL default value change",
			databaseType: DatabasePostgreSQL,
			tableName:    "settings",
			oldField: Field{
				Name:    "status",
				Type:    "varchar",
				Length:  50,
				Default: "active",
			},
			newField: Field{
				Name:    "status",
				Type:    "varchar",
				Length:  50,
				Default: "pending",
			},
			expectedUp:   `ALTER TABLE "settings" ALTER COLUMN "status" SET DEFAULT 'pending';`,
			expectedDown: `ALTER TABLE "settings" ALTER COLUMN "status" SET DEFAULT 'active';`,
		},
		{
			name:         "PostgreSQL precision and scale change",
			databaseType: DatabasePostgreSQL,
			tableName:    "finances",
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
			expectedUp:   `ALTER TABLE "finances" ALTER COLUMN "amount" TYPE DECIMAL(15,4);`,
			expectedDown: `ALTER TABLE "finances" ALTER COLUMN "amount" TYPE DECIMAL(10,2);`,
		},
		{
			name:         "MySQL type and length change",
			databaseType: DatabaseMySQL,
			tableName:    "articles",
			oldField: Field{
				Name:   "title",
				Type:   "varchar",
				Length: 100,
			},
			newField: Field{
				Name:   "title",
				Type:   "varchar",
				Length: 200,
			},
			expectedUp:   "ALTER TABLE `articles` MODIFY COLUMN `title` VARCHAR(200);",
			expectedDown: "ALTER TABLE `articles` MODIFY COLUMN `title` VARCHAR(100);",
		},
		{
			name:         "SQLite unsupported type change",
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
			expectedUp:   "-- SQLite doesn't support ALTER COLUMN TYPE. Manual table recreation required for logs.message",
			expectedDown: "-- SQLite doesn't support ALTER COLUMN TYPE. Manual table recreation required for logs.message",
		},
		{
			name:         "No changes - should return empty",
			databaseType: DatabasePostgreSQL,
			tableName:    "unchanged",
			oldField: Field{
				Name: "id",
				Type: "serial",
			},
			newField: Field{
				Name: "id",
				Type: "serial",
			},
			expectedUp:   "",
			expectedDown: "",
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
					PostgreSQL: map[string]string{
						"active":  "'active'",
						"pending": "'pending'",
					},
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
					PostgreSQL: map[string]string{
						"active":  "'active'",
						"pending": "'pending'",
					},
				},
			}

			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.generateFieldModificationSQL(tt.tableName, &tt.oldField, &tt.newField, oldSchema, newSchema)

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

			if upSQL != tt.expectedUp {
				t.Errorf("Up SQL mismatch.\nExpected: %q\nGot:      %q", tt.expectedUp, upSQL)
			}

			if downSQL != tt.expectedDown {
				t.Errorf("Down SQL mismatch.\nExpected: %q\nGot:      %q", tt.expectedDown, downSQL)
			}
		})
	}
}

func TestConvertChangeToSQLFieldModified(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		change       Change
		oldSchema    *Schema
		newSchema    *Schema
		expectedUp   string
		expectedDown string
		shouldError  bool
	}{
		{
			name:         "Field modification through Change object",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:        ChangeTypeFieldModified,
				TableName:   "users",
				FieldName:   "email",
				Description: "Change field 'users.email' length from 255 to 320",
				OldValue:    255,
				NewValue:    320,
			},
			oldSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 255},
						},
					},
				},
			},
			newSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 320},
						},
					},
				},
			},
			expectedUp:   `ALTER TABLE "users" ALTER COLUMN "email" TYPE VARCHAR(320);`,
			expectedDown: `ALTER TABLE "users" ALTER COLUMN "email" TYPE VARCHAR(255);`,
		},
		{
			name:         "Field not found in old schema",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:        ChangeTypeFieldModified,
				TableName:   "users",
				FieldName:   "nonexistent",
				Description: "Change field that doesn't exist",
				OldValue:    255,
				NewValue:    320,
			},
			oldSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 255},
						},
					},
				},
			},
			newSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 255},
						},
					},
				},
			},
			shouldError: true,
		},
		{
			name:         "Table not found",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:        ChangeTypeFieldModified,
				TableName:   "nonexistent_table",
				FieldName:   "field",
				Description: "Change field in table that doesn't exist",
				OldValue:    255,
				NewValue:    320,
			},
			oldSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 255},
						},
					},
				},
			},
			newSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "email", Type: "varchar", Length: 255},
						},
					},
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.convertChangeToSQL(tt.change, tt.oldSchema, tt.newSchema)

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

			if upSQL != tt.expectedUp {
				t.Errorf("Up SQL mismatch.\nExpected: %q\nGot:      %q", tt.expectedUp, upSQL)
			}

			if downSQL != tt.expectedDown {
				t.Errorf("Down SQL mismatch.\nExpected: %q\nGot:      %q", tt.expectedDown, downSQL)
			}
		})
	}
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}
