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
package turso

import (
	"errors"
	"strings"
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
		{types.Field{Type: "varchar"}, "TEXT"},
		{types.Field{Type: "integer"}, "INTEGER"},
		{types.Field{Type: "serial"}, "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{types.Field{Type: "boolean"}, "INTEGER"},
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
				Name: "email",
				Type: "varchar",
			},
		},
	}

	result, err := provider.GenerateCreateTable(schema, table)
	if err != nil {
		t.Errorf("GenerateCreateTable() returned error: %v", err)
		return
	}

	expected := `CREATE TABLE "users" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "email" TEXT
);`

	if result != expected {
		t.Errorf("GenerateCreateTable() = %s; expected %s", result, expected)
	}
}

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("no such table: users"), true},
		{errors.New("no such column: email"), true},
		{errors.New("no such index: idx_email"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestProvider_GenerateForeignKeyConstraints(t *testing.T) {
	p := New()
	schema := &types.Schema{
		Tables: []types.Table{
			{
				Name: "orders",
				Fields: []types.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key", ForeignKey: &types.ForeignKey{Table: "users", OnDelete: "cascade"}},
				},
			},
		},
	}
	got := p.GenerateForeignKeyConstraints(schema, nil)
	// Turso's GenerateForeignKeyConstraint returns "", so this should also be empty
	if got != "" {
		t.Errorf("expected empty for Turso FK constraints, got:\n%s", got)
	}
}

func TestProvider_GenerateForeignKeyConstraints_Empty(t *testing.T) {
	p := New()
	schema := &types.Schema{
		Tables: []types.Table{
			{
				Name: "simple",
				Fields: []types.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
				},
			},
		},
	}
	got := p.GenerateForeignKeyConstraints(schema, nil)
	if got != "" {
		t.Errorf("expected empty for no FKs, got:\n%s", got)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestProvider_GenerateAlterColumn_ReturnsError(t *testing.T) {
	p := New()
	old := &types.Field{Name: "score", Type: "integer"}
	nw := &types.Field{Name: "score", Type: "text"}
	_, err := p.GenerateAlterColumn("results", old, nw)
	if err == nil {
		t.Fatal("expected error for ALTER COLUMN")
	}
	if !strings.Contains(err.Error(), "does not support ALTER COLUMN") {
		t.Errorf("expected unsupported error, got: %v", err)
	}
}

func TestProvider_GenerateAlterColumn_NoChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "name", Type: "varchar", Length: 100}
	nw := &types.Field{Name: "name", Type: "varchar", Length: 100}
	got, err := p.GenerateAlterColumn("things", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for no-change alter, got: %q", got)
	}
}
