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
package auroradsql

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
	}

	for _, test := range tests {
		result := provider.QuoteName(test.input)
		if result != test.expected {
			t.Errorf("QuoteName(%s) = %s; expected %s", test.input, result, test.expected)
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
		{types.Field{Type: "integer"}, "INTEGER"},
		{types.Field{Type: "serial"}, "SERIAL"},
		{types.Field{Type: "uuid"}, "UUID"},
		{types.Field{Type: "jsonb"}, "JSONB"},
	}

	for _, test := range tests {
		result := provider.ConvertFieldType(&test.field)
		if result != test.expected {
			t.Errorf("ConvertFieldType(%+v) = %s; expected %s", test.field, result, test.expected)
		}
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

	result, err := provider.GenerateCreateTable(schema, table)
	if err != nil {
		t.Errorf("GenerateCreateTable() returned error: %v", err)
		return
	}

	expected := `CREATE TABLE "users" (
    "id" SERIAL NOT NULL,
    "email" VARCHAR(255),
    PRIMARY KEY ("id")
);`

	if result != expected {
		t.Errorf("GenerateCreateTable() = %s; expected %s", result, expected)
	}
}
