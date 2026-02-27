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

package migrate_test

import (
	"testing"

	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/migrate"
)

func TestCreateTable_Up(t *testing.T) {
	p := sqlite.New()
	state := migrate.NewSchemaState()
	op := &migrate.CreateTable{
		Name: "users",
		Fields: []migrate.Field{
			{Name: "id", Type: "integer", PrimaryKey: true},
			{Name: "email", Type: "varchar", Length: 255},
		},
	}
	sql, err := op.Up(p, state, nil)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if err := op.Mutate(state); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if _, ok := state.Tables["users"]; !ok {
		t.Fatal("expected 'users' in state after Mutate")
	}
}

func TestCreateTable_Down(t *testing.T) {
	p := sqlite.New()
	state := migrate.NewSchemaState()
	op := &migrate.CreateTable{Name: "users", Fields: []migrate.Field{{Name: "id", Type: "integer"}}}
	sql, err := op.Down(p, state, nil)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty down SQL")
	}
}

func TestAddField_UpDown(t *testing.T) {
	p := sqlite.New()
	state := migrate.NewSchemaState()
	_ = state.AddTable("users", []migrate.Field{{Name: "id", Type: "integer"}}, nil)
	op := &migrate.AddField{
		Table: "users",
		Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
	}
	upSQL, err := op.Up(p, state, nil)
	if err != nil || upSQL == "" {
		t.Fatalf("Up: err=%v sql=%q", err, upSQL)
	}
	downSQL, err := op.Down(p, state, nil)
	if err != nil || downSQL == "" {
		t.Fatalf("Down: err=%v sql=%q", err, downSQL)
	}
	if err := op.Mutate(state); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(state.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(state.Tables["users"].Fields))
	}
}

func TestDropTable_IsDestructive(t *testing.T) {
	op := &migrate.DropTable{Name: "users"}
	if !op.IsDestructive() {
		t.Fatal("expected DropTable to be destructive")
	}
}

func TestDropField_IsDestructive(t *testing.T) {
	op := &migrate.DropField{Table: "users", Field: "email"}
	if !op.IsDestructive() {
		t.Fatal("expected DropField to be destructive")
	}
}

func TestCreateTable_IsNotDestructive(t *testing.T) {
	op := &migrate.CreateTable{Name: "t"}
	if op.IsDestructive() {
		t.Fatal("CreateTable should not be destructive")
	}
}

func TestRunSQL_UpDown(t *testing.T) {
	op := &migrate.RunSQL{
		ForwardSQL:  "UPDATE posts SET slug = 'x'",
		BackwardSQL: "UPDATE posts SET slug = NULL",
	}
	sql, _ := op.Up(nil, nil, nil)
	if sql != "UPDATE posts SET slug = 'x'" {
		t.Fatalf("expected forward SQL, got %q", sql)
	}
	back, _ := op.Down(nil, nil, nil)
	if back != "UPDATE posts SET slug = NULL" {
		t.Fatalf("expected backward SQL, got %q", back)
	}
	if op.Mutate(migrate.NewSchemaState()) != nil {
		t.Fatal("RunSQL.Mutate should not error")
	}
}

func TestRunSQL_TypeName(t *testing.T) {
	op := &migrate.RunSQL{}
	if op.TypeName() != "run_sql" {
		t.Fatalf("expected 'run_sql', got %q", op.TypeName())
	}
}

func TestDropField_Down_ReconstructsFromState(t *testing.T) {
	p := sqlite.New()
	state := migrate.NewSchemaState()
	_ = state.AddTable("users", []migrate.Field{
		{Name: "id", Type: "integer"},
		{Name: "email", Type: "varchar", Length: 255},
	}, nil)
	op := &migrate.DropField{Table: "users", Field: "email"}
	sql, err := op.Down(p, state, nil)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if sql == "" {
		t.Fatal("expected down SQL for DropField")
	}
}

func TestAllTypenames(t *testing.T) {
	cases := []struct {
		op       migrate.Operation
		expected string
	}{
		{&migrate.CreateTable{Name: "t"}, "create_table"},
		{&migrate.DropTable{Name: "t"}, "drop_table"},
		{&migrate.RenameTable{OldName: "a", NewName: "b"}, "rename_table"},
		{&migrate.AddField{Table: "t", Field: migrate.Field{Name: "f", Type: "text"}}, "add_field"},
		{&migrate.DropField{Table: "t", Field: "f"}, "drop_field"},
		{&migrate.AlterField{Table: "t"}, "alter_field"},
		{&migrate.RenameField{Table: "t", OldName: "a", NewName: "b"}, "rename_field"},
		{&migrate.AddIndex{Table: "t", Index: migrate.Index{Name: "i", Fields: []string{"f"}}}, "add_index"},
		{&migrate.DropIndex{Table: "t", Index: "i"}, "drop_index"},
		{&migrate.RunSQL{}, "run_sql"},
	}
	for _, tc := range cases {
		if tc.op.TypeName() != tc.expected {
			t.Errorf("%T: expected TypeName %q, got %q", tc.op, tc.expected, tc.op.TypeName())
		}
	}
}
