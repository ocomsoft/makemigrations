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

func TestConvertChangeToSQLTableRenamed(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		change       Change
		expectedUp   string
		expectedDown string
	}{
		{
			name:         "PostgreSQL table rename",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:      ChangeTypeTableRenamed,
				TableName: "old_table",
				OldValue:  "old_table",
				NewValue:  "new_table",
			},
			expectedUp:   `ALTER TABLE "old_table" RENAME TO "new_table";`,
			expectedDown: `ALTER TABLE "new_table" RENAME TO "old_table";`,
		},
		{
			name:         "MySQL table rename",
			databaseType: DatabaseMySQL,
			change: Change{
				Type:      ChangeTypeTableRenamed,
				TableName: "users",
				OldValue:  "users",
				NewValue:  "customers",
			},
			expectedUp:   "ALTER TABLE `users` RENAME TO `customers`;",
			expectedDown: "ALTER TABLE `customers` RENAME TO `users`;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.convertChangeToSQL(tt.change, nil, nil)

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

func TestConvertChangeToSQLFieldRenamed(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		change       Change
		newSchema    *Schema
		expectedUp   string
		expectedDown string
		shouldError  bool
	}{
		{
			name:         "PostgreSQL field rename",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:      ChangeTypeFieldRenamed,
				TableName: "users",
				FieldName: "new_name",
				OldValue:  "old_name",
				NewValue:  "new_name",
			},
			expectedUp:   `ALTER TABLE "users" RENAME COLUMN "old_name" TO "new_name";`,
			expectedDown: `ALTER TABLE "users" RENAME COLUMN "new_name" TO "old_name";`,
		},
		{
			name:         "MySQL field rename",
			databaseType: DatabaseMySQL,
			change: Change{
				Type:      ChangeTypeFieldRenamed,
				TableName: "products",
				FieldName: "new_title",
				OldValue:  "old_title",
				NewValue:  "new_title",
			},
			newSchema: &Schema{
				Tables: []Table{
					{
						Name: "products",
						Fields: []Field{
							{Name: "new_title", Type: "varchar", Length: 255},
						},
					},
				},
			},
			expectedUp:   "ALTER TABLE `products` CHANGE `old_title` `new_title` VARCHAR(255);",
			expectedDown: "ALTER TABLE `products` CHANGE `new_title` `old_title` VARCHAR(255);",
		},
		{
			name:         "SQL Server field rename",
			databaseType: DatabaseSQLServer,
			change: Change{
				Type:      ChangeTypeFieldRenamed,
				TableName: "orders",
				FieldName: "new_status",
				OldValue:  "old_status",
				NewValue:  "new_status",
			},
			expectedUp:   "EXEC sp_rename 'orders.old_status', 'new_status', 'COLUMN';",
			expectedDown: "EXEC sp_rename 'orders.new_status', 'old_status', 'COLUMN';",
		},
		{
			name:         "SQLite field rename (unsupported)",
			databaseType: DatabaseSQLite,
			change: Change{
				Type:      ChangeTypeFieldRenamed,
				TableName: "logs",
				FieldName: "new_message",
				OldValue:  "old_message",
				NewValue:  "new_message",
			},
			expectedUp:   "-- SQLite doesn't support ALTER TABLE RENAME COLUMN. Manual table recreation required for logs.old_message -> new_message",
			expectedDown: "-- SQLite doesn't support ALTER TABLE RENAME COLUMN. Manual table recreation required for logs.new_message -> old_message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.convertChangeToSQL(tt.change, nil, tt.newSchema)

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

func TestConvertChangeToSQLIndexChanges(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		change       Change
		expectedUp   string
		expectedDown string
	}{
		{
			name:         "PostgreSQL index added",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:      ChangeTypeIndexAdded,
				TableName: "users",
				FieldName: "email",
				NewValue:  "idx_users_email",
			},
			expectedUp:   `CREATE INDEX "idx_users_email" ON "users" ("email");`,
			expectedDown: `DROP INDEX "idx_users_email";`,
		},
		{
			name:         "MySQL index added",
			databaseType: DatabaseMySQL,
			change: Change{
				Type:      ChangeTypeIndexAdded,
				TableName: "products",
				FieldName: "sku",
				NewValue:  "idx_products_sku",
			},
			expectedUp:   "CREATE INDEX `idx_products_sku` ON `products` (`sku`);",
			expectedDown: "DROP INDEX `idx_products_sku` ON `products`;",
		},
		{
			name:         "SQL Server index added",
			databaseType: DatabaseSQLServer,
			change: Change{
				Type:      ChangeTypeIndexAdded,
				TableName: "orders",
				FieldName: "status",
				NewValue:  "idx_orders_status",
			},
			expectedUp:   "CREATE INDEX [idx_orders_status] ON [orders] ([status]);",
			expectedDown: "DROP INDEX [idx_orders_status] ON [orders];",
		},
		{
			name:         "PostgreSQL index removed",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:      ChangeTypeIndexRemoved,
				TableName: "logs",
				FieldName: "timestamp",
				OldValue:  "idx_logs_timestamp",
			},
			expectedUp:   `DROP INDEX "idx_logs_timestamp";`,
			expectedDown: `CREATE INDEX "idx_logs_timestamp" ON "logs" ("timestamp");`,
		},
		{
			name:         "MySQL index removed",
			databaseType: DatabaseMySQL,
			change: Change{
				Type:      ChangeTypeIndexRemoved,
				TableName: "sessions",
				FieldName: "user_id",
				OldValue:  "idx_sessions_user_id",
			},
			expectedUp:   "DROP INDEX `idx_sessions_user_id` ON `sessions`;",
			expectedDown: "CREATE INDEX `idx_sessions_user_id` ON `sessions` (`user_id`);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.convertChangeToSQL(tt.change, nil, nil)

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

func TestConvertChangeToSQLFieldRemoved(t *testing.T) {
	tests := []struct {
		name         string
		databaseType DatabaseType
		change       Change
		oldSchema    *Schema
		expectedUp   string
		expectedDown string
	}{
		{
			name:         "PostgreSQL field removed",
			databaseType: DatabasePostgreSQL,
			change: Change{
				Type:      ChangeTypeFieldRemoved,
				TableName: "users",
				FieldName: "old_field",
				OldValue: Field{
					Name:   "old_field",
					Type:   "varchar",
					Length: 255,
				},
			},
			oldSchema: &Schema{
				Tables: []Table{
					{
						Name: "users",
						Fields: []Field{
							{Name: "old_field", Type: "varchar", Length: 255},
						},
					},
				},
			},
			expectedUp:   `ALTER TABLE "users" DROP COLUMN "old_field";`,
			expectedDown: `ALTER TABLE "users" ADD COLUMN "old_field" VARCHAR(255);`,
		},
		{
			name:         "MySQL field removed with nullable",
			databaseType: DatabaseMySQL,
			change: Change{
				Type:      ChangeTypeFieldRemoved,
				TableName: "products",
				FieldName: "description",
				OldValue: Field{
					Name:     "description",
					Type:     "text",
					Nullable: boolPtr(true),
				},
			},
			oldSchema: &Schema{
				Tables: []Table{
					{
						Name: "products",
						Fields: []Field{
							{Name: "description", Type: "text", Nullable: boolPtr(true)},
						},
					},
				},
			},
			expectedUp:   "ALTER TABLE `products` DROP COLUMN `description`;",
			expectedDown: "ALTER TABLE `products` ADD COLUMN `description` TEXT;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewSQLConverter(tt.databaseType, false)
			upSQL, downSQL, err := converter.convertChangeToSQL(tt.change, tt.oldSchema, nil)

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
