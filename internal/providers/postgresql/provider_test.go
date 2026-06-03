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
package postgresql

import (
	"errors"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func boolPtr(b bool) *bool { return &b }

func TestProvider_GenerateAlterColumn_TypeChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "score", Type: "integer"}
	nw := &types.Field{Name: "score", Type: "bigint"}
	got, err := p.GenerateAlterColumn("results", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER TABLE "results" ALTER COLUMN "score" TYPE BIGINT;`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProvider_GenerateAlterColumn_NullableToNotNull(t *testing.T) {
	p := New()
	old := &types.Field{Name: "email", Type: "varchar", Nullable: boolPtr(true)}
	nw := &types.Field{Name: "email", Type: "varchar", Nullable: boolPtr(false)}
	got, err := p.GenerateAlterColumn("users", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER TABLE "users" ALTER COLUMN "email" SET NOT NULL;`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProvider_GenerateAlterColumn_NotNullToNullable(t *testing.T) {
	p := New()
	old := &types.Field{Name: "email", Type: "varchar", Nullable: boolPtr(false)}
	nw := &types.Field{Name: "email", Type: "varchar", Nullable: boolPtr(true)}
	got, err := p.GenerateAlterColumn("users", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER TABLE "users" ALTER COLUMN "email" DROP NOT NULL;`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProvider_GenerateAlterColumn_AddDefault(t *testing.T) {
	p := New()
	old := &types.Field{Name: "status", Type: "varchar"}
	nw := &types.Field{Name: "status", Type: "varchar", Default: "active"}
	got, err := p.GenerateAlterColumn("orders", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER TABLE "orders" ALTER COLUMN "status" SET DEFAULT 'active';`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProvider_GenerateAlterColumn_DropDefault(t *testing.T) {
	p := New()
	old := &types.Field{Name: "status", Type: "varchar", Default: "active"}
	nw := &types.Field{Name: "status", Type: "varchar"}
	got, err := p.GenerateAlterColumn("orders", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `ALTER TABLE "orders" ALTER COLUMN "status" DROP DEFAULT;`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestProvider_GenerateAlterColumn_MultipleChanges(t *testing.T) {
	p := New()
	old := &types.Field{Name: "ref_id", Type: "integer", Nullable: boolPtr(true)}
	nw := &types.Field{Name: "ref_id", Type: "bigint", Nullable: boolPtr(false)}
	got, err := p.GenerateAlterColumn("items", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, `ALTER COLUMN "ref_id" TYPE BIGINT`) {
		t.Errorf("expected TYPE clause in:\n%s", got)
	}
	if !strings.Contains(got, `ALTER COLUMN "ref_id" SET NOT NULL`) {
		t.Errorf("expected SET NOT NULL clause in:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_NoChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "name", Type: "varchar"}
	nw := &types.Field{Name: "name", Type: "varchar"}
	got, err := p.GenerateAlterColumn("things", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for no-change alter, got: %q", got)
	}
}

func TestProvider_GenerateForeignKeyConstraint(t *testing.T) {
	p := New()
	got := p.GenerateForeignKeyConstraint("users", "org_id", "organizations", "", "cascade", "")
	if !strings.Contains(got, "FOREIGN KEY") {
		t.Errorf("expected FOREIGN KEY in:\n%s", got)
	}
	if !strings.Contains(got, "ON DELETE CASCADE") {
		t.Errorf("expected ON DELETE CASCADE in:\n%s", got)
	}
}

func TestProvider_GenerateForeignKeyConstraint_OnUpdate(t *testing.T) {
	p := New()
	got := p.GenerateForeignKeyConstraint("orders", "user_id", "users", "", "CASCADE", "CASCADE")
	if !strings.Contains(got, "ON DELETE CASCADE") {
		t.Errorf("expected ON DELETE CASCADE in:\n%s", got)
	}
	if !strings.Contains(got, "ON UPDATE CASCADE") {
		t.Errorf("expected ON UPDATE CASCADE in:\n%s", got)
	}
}

func TestProvider_GenerateForeignKeyConstraint_CustomConstraintName(t *testing.T) {
	p := New()
	got := p.GenerateForeignKeyConstraint("orders", "user_id", "users", "my_custom_fk", "CASCADE", "")
	if !strings.Contains(got, `"my_custom_fk"`) {
		t.Errorf("expected custom constraint name my_custom_fk in:\n%s", got)
	}
	// Should NOT contain the auto-generated name
	if strings.Contains(got, "fk_orders_user_id") {
		t.Errorf("should use custom constraint name, not auto-generated, in:\n%s", got)
	}
}

func TestProvider_GenerateForeignKeyConstraint_FallbackConstraintName(t *testing.T) {
	p := New()
	got := p.GenerateForeignKeyConstraint("orders", "user_id", "users", "", "CASCADE", "")
	// Should fall back to auto-generated name when constraintName is empty
	if !strings.Contains(got, "fk_orders_user_id") {
		t.Errorf("expected auto-generated constraint name fk_orders_user_id in:\n%s", got)
	}
}

func TestProvider_GenerateForeignKeyConstraint_OnUpdateOnly(t *testing.T) {
	p := New()
	got := p.GenerateForeignKeyConstraint("orders", "user_id", "users", "", "", "SET NULL")
	if strings.Contains(got, "ON DELETE") {
		t.Errorf("should not contain ON DELETE when onDelete is empty, got:\n%s", got)
	}
	if !strings.Contains(got, "ON UPDATE SET NULL") {
		t.Errorf("expected ON UPDATE SET NULL in:\n%s", got)
	}
}

func TestProvider_GenerateDropForeignKeyConstraint(t *testing.T) {
	p := New()
	got := p.GenerateDropForeignKeyConstraint("users", "fk_users_org_id")
	if got == "" {
		t.Error("expected non-empty drop constraint SQL")
	}
}

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New(`pq: table "users" does not exist`), true},
		{errors.New(`pq: column "email" of relation "users" does not exist`), true},
		{errors.New(`pq: index "idx_users_email" does not exist`), true},
		{errors.New(`pq: syntax error at or near "DROP"`), false},
		{errors.New(`connection refused`), false},
		{nil, false},
	}
	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
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
	if got == "" {
		t.Error("expected non-empty FK constraints")
	}
	if !strings.Contains(got, "FOREIGN KEY") {
		t.Errorf("expected FOREIGN KEY in:\n%s", got)
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

func TestProvider_InferForeignKeyType(t *testing.T) {
	provider := New()

	result := provider.InferForeignKeyType("users", &types.Schema{})
	expected := "BIGINT"

	if result != expected {
		t.Errorf("InferForeignKeyType() = %s; expected %s", result, expected)
	}
}

// TestConvertSQLTypeToYAML_UnknownPassthrough verifies that unrecognized
// PostgreSQL types are passed through as lowercase rather than defaulting
// to "text".
func TestConvertSQLTypeToYAML_UnknownPassthrough(t *testing.T) {
	p := &Provider{}

	cases := []struct {
		sqlType  string
		expected string
	}{
		{"citext", "citext"},
		{"inet", "inet"},
		{"bytea", "bytea"},
		{"money", "money"},
		{"interval", "interval"},
		{"hstore", "hstore"},
		{"point", "point"},
		{"tsvector", "tsvector"},
		{"USER-DEFINED", "user-defined"},
	}

	for _, tc := range cases {
		t.Run(tc.sqlType, func(t *testing.T) {
			result := p.convertSQLTypeToYAML(tc.sqlType)
			if result != tc.expected {
				t.Errorf("convertSQLTypeToYAML(%q) = %q, want %q", tc.sqlType, result, tc.expected)
			}
		})
	}
}

// TestConvertFieldType_UnknownPassthrough verifies that unrecognized YAML
// field types are passed through as uppercase SQL types rather than
// defaulting to "TEXT".
func TestConvertFieldType_UnknownPassthrough(t *testing.T) {
	p := &Provider{}

	cases := []struct {
		yamlType string
		expected string
	}{
		{"citext", "CITEXT"},
		{"inet", "INET"},
		{"bytea", "BYTEA"},
		{"money", "MONEY"},
		{"hstore", "HSTORE"},
		{"tsvector", "TSVECTOR"},
	}

	for _, tc := range cases {
		t.Run(tc.yamlType, func(t *testing.T) {
			field := &types.Field{Name: "test", Type: tc.yamlType}
			result := p.ConvertFieldType(field)
			if result != tc.expected {
				t.Errorf("ConvertFieldType(%q) = %q, want %q", tc.yamlType, result, tc.expected)
			}
		})
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
	if !strings.Contains(got, "FOREIGN KEY") {
		t.Errorf("expected FOREIGN KEY, got:\n%s", got)
	}
	if !strings.Contains(got, "REFERENCES") {
		t.Errorf("expected REFERENCES, got:\n%s", got)
	}
}

func TestProvider_GenerateAddColumn_NoDefault(t *testing.T) {
	p := New()
	field := types.Field{Name: "score", Type: "integer"}
	field.SetNullable(false)
	got := p.GenerateAddColumn("items", &field)
	expected := `ALTER TABLE "items" ADD COLUMN "score" INTEGER NOT NULL;`
	if got != expected {
		t.Errorf("GenerateAddColumn() = %q; want %q", got, expected)
	}
}

func TestProvider_GenerateAddColumn_WithDefault(t *testing.T) {
	p := New()
	field := types.Field{Name: "display_order", Type: "integer", Default: "0"}
	field.SetNullable(false)
	got := p.GenerateAddColumn("items", &field)
	expected := `ALTER TABLE "items" ADD COLUMN "display_order" INTEGER NOT NULL DEFAULT 0;`
	if got != expected {
		t.Errorf("GenerateAddColumn() = %q; want %q", got, expected)
	}
}

func TestProvider_GenerateAddColumn_WithFunctionDefault(t *testing.T) {
	p := New()
	field := types.Field{Name: "id", Type: "uuid", Default: "gen_random_uuid()"}
	field.SetNullable(false)
	got := p.GenerateAddColumn("items", &field)
	expected := `ALTER TABLE "items" ADD COLUMN "id" UUID NOT NULL DEFAULT gen_random_uuid();`
	if got != expected {
		t.Errorf("GenerateAddColumn() = %q; want %q", got, expected)
	}
}

func TestProvider_GenerateAddColumn_NullableWithDefault(t *testing.T) {
	p := New()
	field := types.Field{Name: "notes", Type: "text", Default: "''"}
	field.SetNullable(true)
	got := p.GenerateAddColumn("items", &field)
	expected := `ALTER TABLE "items" ADD COLUMN "notes" TEXT DEFAULT '';`
	if got != expected {
		t.Errorf("GenerateAddColumn() = %q; want %q", got, expected)
	}
}

func TestGenerateCreateIndex_WithMethod(t *testing.T) {
	p := New()
	idx := &types.Index{Name: "users_email_gin_idx", Fields: []string{"email"}, Method: "GIN"}
	sql := p.GenerateCreateIndex(idx, "users")
	if !strings.Contains(sql, "USING GIN") {
		t.Errorf("expected USING GIN in SQL, got: %s", sql)
	}
}

func TestGenerateCreateIndex_WithWhere(t *testing.T) {
	p := New()
	idx := &types.Index{Name: "users_active_idx", Fields: []string{"email"}, Where: "active = true"}
	sql := p.GenerateCreateIndex(idx, "users")
	if !strings.Contains(sql, "WHERE active = true") {
		t.Errorf("expected WHERE clause in SQL, got: %s", sql)
	}
}

// TestGenerateCreateTable_WithIndexes verifies that GenerateCreateTable emits
// CREATE INDEX statements for indexes defined on the table.
func TestGenerateCreateTable_WithIndexes(t *testing.T) {
	p := New()
	schema := &types.Schema{}
	table := &types.Table{
		Name: "orders",
		Fields: []types.Field{
			{Name: "id", Type: "integer", PrimaryKey: true},
			{Name: "customer_id", Type: "integer"},
			{Name: "email", Type: "varchar"},
			{Name: "status", Type: "varchar"},
		},
		Indexes: []types.Index{
			{
				Name:   "idx_orders_customer",
				Fields: []string{"customer_id"},
			},
			{
				Name:   "idx_orders_email_status",
				Fields: []string{"email", "status"},
				Unique: true,
			},
		},
	}

	result, err := p.GenerateCreateTable(schema, table)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE in output, got: %s", result)
	}

	if !strings.Contains(result, "CREATE INDEX") {
		t.Errorf("expected CREATE INDEX in output, got: %s", result)
	}
	if !strings.Contains(result, "idx_orders_customer") {
		t.Errorf("expected idx_orders_customer index name in output, got: %s", result)
	}

	if !strings.Contains(result, "CREATE UNIQUE INDEX") {
		t.Errorf("expected CREATE UNIQUE INDEX in output, got: %s", result)
	}
	if !strings.Contains(result, "idx_orders_email_status") {
		t.Errorf("expected idx_orders_email_status index name in output, got: %s", result)
	}
}

func TestProvider_GenerateAddColumn_PrimaryKey(t *testing.T) {
	p := New()
	field := types.Field{Name: "id", Type: "uuid", PrimaryKey: true}
	field.SetNullable(false)
	got := p.GenerateAddColumn("users", &field)
	if !strings.Contains(got, "PRIMARY KEY") {
		t.Errorf("GenerateAddColumn() with PrimaryKey=true should contain PRIMARY KEY, got: %s", got)
	}
	if !strings.Contains(got, `"id"`) {
		t.Errorf("GenerateAddColumn() should contain quoted field name, got: %s", got)
	}
}

func TestProvider_GenerateAlterColumn_SQLExpressionDefaultNotQuoted(t *testing.T) {
	p := New()

	tests := []struct {
		name       string
		newDefault string
		wantSQL    string
	}{
		{
			name:       "function call gen_random_uuid()",
			newDefault: "gen_random_uuid()",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT gen_random_uuid();`,
		},
		{
			name:       "function call uuid_generate_v4()",
			newDefault: "uuid_generate_v4()",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT uuid_generate_v4();`,
		},
		{
			name:       "CURRENT_TIMESTAMP keyword",
			newDefault: "CURRENT_TIMESTAMP",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT CURRENT_TIMESTAMP;`,
		},
		{
			name:       "now() function",
			newDefault: "now()",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT now();`,
		},
		{
			name:       "type cast expression",
			newDefault: "'{}'::jsonb",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT '{}'::jsonb;`,
		},
		{
			name:       "NULL literal",
			newDefault: "NULL",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT NULL;`,
		},
		{
			name:       "boolean true",
			newDefault: "true",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT true;`,
		},
		{
			name:       "numeric value",
			newDefault: "42",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT 42;`,
		},
		{
			name:       "plain string is quoted",
			newDefault: "hello",
			wantSQL:    `ALTER TABLE "users" ALTER COLUMN "id" SET DEFAULT 'hello';`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &types.Field{Name: "id", Type: "uuid", Default: "old_value"}
			nw := &types.Field{Name: "id", Type: "uuid", Default: tt.newDefault}
			got, err := p.GenerateAlterColumn("users", old, nw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSQL {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.wantSQL)
			}
		})
	}
}
