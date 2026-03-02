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
	got := p.GenerateForeignKeyConstraint("users", "org_id", "organizations", "cascade")
	if !strings.Contains(got, "FOREIGN KEY") {
		t.Errorf("expected FOREIGN KEY in:\n%s", got)
	}
	if !strings.Contains(got, "ON DELETE CASCADE") {
		t.Errorf("expected ON DELETE CASCADE in:\n%s", got)
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
