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
package vertica

import (
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func TestProvider_QuoteName(t *testing.T) {
	provider := New()

	tests := []struct {
		input    string
		expected string
	}{
		{"users", `"users"`},
		{"user_id", `"user_id"`},
		{"UsErS", `"UsErS"`},
	}

	for _, test := range tests {
		result := provider.QuoteName(test.input)
		if result != test.expected {
			t.Errorf("QuoteName(%s) = %s; expected %s", test.input, result, test.expected)
		}
	}
}

func TestProvider_SupportsOperation(t *testing.T) {
	provider := New()

	tests := []struct {
		operation string
		expected  bool
	}{
		{"DROP_COLUMN", true},
		{"ALTER_COLUMN", true},
		{"RENAME_TABLE", true},
		{"RENAME_COLUMN", false}, // Vertica doesn't support direct column rename
		{"UNKNOWN_OPERATION", false},
	}

	for _, test := range tests {
		result := provider.SupportsOperation(test.operation)
		if result != test.expected {
			t.Errorf("SupportsOperation(%s) = %v; expected %v", test.operation, result, test.expected)
		}
	}
}

func TestProvider_ConvertFieldType(t *testing.T) {
	provider := New()

	tests := []struct {
		field    types.Field
		expected string
	}{
		{types.Field{Type: "varchar", Length: 255}, "VARCHAR(255)"},
		{types.Field{Type: "varchar"}, "VARCHAR(65000)"},
		{types.Field{Type: "text"}, "LONG VARCHAR"},
		{types.Field{Type: "integer"}, "INTEGER"},
		{types.Field{Type: "bigint"}, "BIGINT"},
		{types.Field{Type: "serial"}, "INTEGER IDENTITY"},
		{types.Field{Type: "float"}, "FLOAT"},
		{types.Field{Type: "decimal", Precision: 10, Scale: 2}, "DECIMAL(10,2)"},
		{types.Field{Type: "decimal"}, "DECIMAL(37,15)"},
		{types.Field{Type: "boolean"}, "BOOLEAN"},
		{types.Field{Type: "date"}, "DATE"},
		{types.Field{Type: "time"}, "TIME"},
		{types.Field{Type: "timestamp"}, "TIMESTAMP"},
		{types.Field{Type: "uuid"}, "VARCHAR(36)"},
		{types.Field{Type: "jsonb"}, "LONG VARCHAR"},
		{types.Field{Type: "unknown"}, "VARCHAR(65000)"},
	}

	for _, test := range tests {
		result := provider.ConvertFieldType(&test.field)
		if result != test.expected {
			t.Errorf("ConvertFieldType(%+v) = %s; expected %s", test.field, result, test.expected)
		}
	}
}

func TestProvider_GenerateCreateIndex(t *testing.T) {
	provider := New()

	tests := []struct {
		index     types.Index
		tableName string
		expected  string
	}{
		{
			index:     types.Index{Name: "idx_user_email", Fields: []string{"email"}, Unique: false},
			tableName: "users",
			expected:  `CREATE INDEX "idx_user_email" ON "users" ("email");`,
		},
		{
			index:     types.Index{Name: "idx_unique_email", Fields: []string{"email"}, Unique: true},
			tableName: "users",
			expected:  `CREATE UNIQUE INDEX "idx_unique_email" ON "users" ("email");`,
		},
		{
			index:     types.Index{Name: "idx_name_age", Fields: []string{"name", "age"}, Unique: false},
			tableName: "users",
			expected:  `CREATE INDEX "idx_name_age" ON "users" ("name", "age");`,
		},
	}

	for _, test := range tests {
		result := provider.GenerateCreateIndex(&test.index, test.tableName)
		if result != test.expected {
			t.Errorf("GenerateCreateIndex(%+v, %s) = %s; expected %s", test.index, test.tableName, result, test.expected)
		}
	}
}

func TestProvider_GenerateDropIndex(t *testing.T) {
	provider := New()

	result := provider.GenerateDropIndex("idx_user_email", "users")
	expected := `DROP INDEX "idx_user_email";`

	if result != expected {
		t.Errorf("GenerateDropIndex() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateDropTable(t *testing.T) {
	provider := New()

	result := provider.GenerateDropTable("users")
	expected := `DROP TABLE "users" CASCADE;`

	if result != expected {
		t.Errorf("GenerateDropTable() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateAddColumn(t *testing.T) {
	provider := New()

	field := types.Field{
		Name:   "email",
		Type:   "varchar",
		Length: 255,
	}
	field.SetNullable(false)

	result := provider.GenerateAddColumn("users", &field)
	expected := `ALTER TABLE "users" ADD COLUMN "email" VARCHAR(255) NOT NULL;`

	if result != expected {
		t.Errorf("GenerateAddColumn() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateDropColumn(t *testing.T) {
	provider := New()

	result := provider.GenerateDropColumn("users", "email")
	expected := `ALTER TABLE "users" DROP COLUMN "email" CASCADE;`

	if result != expected {
		t.Errorf("GenerateDropColumn() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateRenameTable(t *testing.T) {
	provider := New()

	result := provider.GenerateRenameTable("old_users", "new_users")
	expected := `ALTER TABLE "old_users" RENAME TO "new_users";`

	if result != expected {
		t.Errorf("GenerateRenameTable() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateRenameColumn(t *testing.T) {
	provider := New()

	result := provider.GenerateRenameColumn("users", "old_name", "new_name")
	// Vertica doesn't support direct column rename
	expected := "-- Vertica doesn't support RENAME COLUMN. Use ADD COLUMN + UPDATE + DROP COLUMN pattern for users.old_name -> new_name;"

	if result != expected {
		t.Errorf("GenerateRenameColumn() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateCreateTable(t *testing.T) {
	provider := New()

	schema := &types.Schema{}
	table := &types.Table{
		Name: "users",
		Fields: []types.Field{
			{
				Name:       "id",
				Type:       "serial",
				PrimaryKey: true,
			},
			{
				Name:   "email",
				Type:   "varchar",
				Length: 255,
			},
		},
	}

	// Set nullable to false for email
	table.Fields[1].SetNullable(false)

	result, err := provider.GenerateCreateTable(schema, table)
	if err != nil {
		t.Errorf("GenerateCreateTable() returned error: %v", err)
		return
	}

	expected := `CREATE TABLE "users" (
    "id" INTEGER IDENTITY NOT NULL,
    "email" VARCHAR(255) NOT NULL,
    PRIMARY KEY ("id")
);`

	if result != expected {
		t.Errorf("GenerateCreateTable() = %s; expected %s", result, expected)
	}
}

func TestProvider_GenerateForeignKeyConstraint(t *testing.T) {
	provider := New()

	result := provider.GenerateForeignKeyConstraint("posts", "user_id", "users", "CASCADE")
	expected := `ALTER TABLE "posts" ADD CONSTRAINT "fk_posts_user_id" FOREIGN KEY ("user_id") REFERENCES "users" ON DELETE CASCADE;`

	if result != expected {
		t.Errorf("GenerateForeignKeyConstraint() = %s; expected %s", result, expected)
	}
}

func TestProvider_InferForeignKeyType(t *testing.T) {
	provider := New()

	result := provider.InferForeignKeyType("users", &types.Schema{})
	expected := "INTEGER"

	if result != expected {
		t.Errorf("InferForeignKeyType() = %s; expected %s", result, expected)
	}
}
