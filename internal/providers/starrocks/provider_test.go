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
package starrocks

import (
	"errors"
	"strings"
	"testing"

	"github.com/ocomsoft/morphic/internal/types"
)

func TestProvider_QuoteName(t *testing.T) {
	provider := New()

	tests := []struct {
		input    string
		expected string
	}{
		{"users", "`users`"},
		{"user_id", "`user_id`"},
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
		{types.Field{Type: "text"}, "STRING"},
		{types.Field{Type: "integer"}, "INT"},
		{types.Field{Type: "jsonb"}, "JSON"},
	}

	for _, test := range tests {
		result := provider.ConvertFieldType(&test.field)
		if result != test.expected {
			t.Errorf("ConvertFieldType(%+v) = %s; expected %s", test.field, result, test.expected)
		}
	}
}

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("Error 1051: Unknown table 'users'"), true},
		{errors.New("Error 1091: Can't DROP 'idx_email'; check that column/key exists"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

func TestProvider_GenerateAlterColumn_TypeChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "score", Type: "integer"}
	nw := &types.Field{Name: "score", Type: "bigint"}
	got, err := p.GenerateAlterColumn("results", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "MODIFY COLUMN") {
		t.Errorf("expected MODIFY COLUMN in:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_NullableToNotNull(t *testing.T) {
	p := New()
	old := &types.Field{Name: "email", Type: "varchar", Length: 255, Nullable: boolPtr(true)}
	nw := &types.Field{Name: "email", Type: "varchar", Length: 255, Nullable: boolPtr(false)}
	got, err := p.GenerateAlterColumn("users", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "NOT NULL") {
		t.Errorf("expected NOT NULL in:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_AddDefault(t *testing.T) {
	p := New()
	old := &types.Field{Name: "status", Type: "varchar", Length: 50}
	nw := &types.Field{Name: "status", Type: "varchar", Length: 50, Default: "active"}
	got, err := p.GenerateAlterColumn("orders", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "DEFAULT 'active'") {
		t.Errorf("expected DEFAULT clause in:\n%s", got)
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

func TestProvider_GenerateJunctionTable(t *testing.T) {
	p := New()
	got, err := p.GenerateJunctionTable("users", "roles", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "roles_users") {
		t.Errorf("expected alphabetically sorted junction table name, got:\n%s", got)
	}
	if !strings.Contains(got, "roles_id") {
		t.Errorf("expected roles_id column, got:\n%s", got)
	}
	if !strings.Contains(got, "users_id") {
		t.Errorf("expected users_id column, got:\n%s", got)
	}
	if !strings.Contains(got, "PRIMARY KEY") {
		t.Errorf("expected PRIMARY KEY, got:\n%s", got)
	}
	if strings.Contains(got, "FOREIGN KEY") {
		t.Errorf("expected no FOREIGN KEY for StarRocks, got:\n%s", got)
	}
}

func TestProvider_GenerateAddColumn_PrimaryKey(t *testing.T) {
	p := New()
	field := types.Field{Name: "id", Type: "uuid", PrimaryKey: true}
	field.SetNullable(false)
	got := p.GenerateAddColumn("users", &field)
	// StarRocks does not support inline PRIMARY KEY on ADD COLUMN; should emit a warning comment
	if !strings.Contains(got, "does not support adding PRIMARY KEY") {
		t.Errorf("GenerateAddColumn() with PrimaryKey=true should contain warning comment, got: %s", got)
	}
	if !strings.Contains(got, "ALTER TABLE") {
		t.Errorf("GenerateAddColumn() should still contain ALTER TABLE statement, got: %s", got)
	}
}

// --- AutoUpdate and AutoCreate tests ---

func TestProvider_ConvertField_AutoCreate(t *testing.T) {
	p := New()
	schema := &types.Schema{}
	field := types.Field{Name: "created_at", Type: "timestamp", AutoCreate: true}
	field.SetNullable(false)

	def, err := p.convertField(schema, &field)
	if err != nil {
		t.Fatalf("convertField() error: %v", err)
	}
	if !strings.Contains(def, "DEFAULT CURRENT_TIMESTAMP") {
		t.Errorf("expected DEFAULT CURRENT_TIMESTAMP in: %s", def)
	}
}

func TestProvider_ConvertField_AutoUpdate(t *testing.T) {
	p := New()
	schema := &types.Schema{}
	field := types.Field{Name: "updated_at", Type: "timestamp", AutoCreate: true, AutoUpdate: true}
	field.SetNullable(false)

	def, err := p.convertField(schema, &field)
	if err != nil {
		t.Fatalf("convertField() error: %v", err)
	}
	if !strings.Contains(def, "ON UPDATE CURRENT_TIMESTAMP") {
		t.Errorf("expected ON UPDATE CURRENT_TIMESTAMP in: %s", def)
	}
}

func TestProvider_GenerateAddColumn_AutoUpdate(t *testing.T) {
	p := New()
	field := types.Field{Name: "updated_at", Type: "timestamp", AutoCreate: true, AutoUpdate: true}
	field.SetNullable(false)

	got := p.GenerateAddColumn("users", &field)
	if !strings.Contains(got, "ON UPDATE CURRENT_TIMESTAMP") {
		t.Errorf("expected ON UPDATE CURRENT_TIMESTAMP in: %s", got)
	}
}

func TestProvider_GenerateAlterColumn_AutoCreate_Change(t *testing.T) {
	p := New()
	oldField := &types.Field{Name: "created_at", Type: "timestamp"}
	newField := &types.Field{Name: "created_at", Type: "timestamp", AutoCreate: true}

	got, err := p.GenerateAlterColumn("users", oldField, newField)
	if err != nil {
		t.Fatalf("GenerateAlterColumn() error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty SQL for AutoCreate change")
	}
	if !strings.Contains(got, "DEFAULT CURRENT_TIMESTAMP") {
		t.Errorf("expected DEFAULT CURRENT_TIMESTAMP in: %s", got)
	}
}

func TestProvider_GenerateAlterColumn_AutoUpdate_Change(t *testing.T) {
	p := New()
	oldField := &types.Field{Name: "updated_at", Type: "timestamp"}
	newField := &types.Field{Name: "updated_at", Type: "timestamp", AutoUpdate: true}

	got, err := p.GenerateAlterColumn("users", oldField, newField)
	if err != nil {
		t.Fatalf("GenerateAlterColumn() error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty SQL for AutoUpdate change")
	}
	if !strings.Contains(got, "ON UPDATE CURRENT_TIMESTAMP") {
		t.Errorf("expected ON UPDATE CURRENT_TIMESTAMP in: %s", got)
	}
}

func TestProvider_GenerateAlterColumn_NoChange_WithAutoFlags(t *testing.T) {
	p := New()
	oldField := &types.Field{Name: "updated_at", Type: "timestamp", AutoCreate: true, AutoUpdate: true}
	newField := &types.Field{Name: "updated_at", Type: "timestamp", AutoCreate: true, AutoUpdate: true}

	got, err := p.GenerateAlterColumn("users", oldField, newField)
	if err != nil {
		t.Fatalf("GenerateAlterColumn() error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty SQL when nothing changed, got: %s", got)
	}
}
