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

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestSchemaState_AddTable(t *testing.T) {
	s := migrate.NewSchemaState()
	err := s.AddTable("users", []migrate.Field{
		{Name: "id", Type: "uuid", PrimaryKey: true},
		{Name: "email", Type: "varchar", Length: 255},
	}, nil)
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if _, ok := s.Tables["users"]; !ok {
		t.Fatal("expected 'users' in Tables")
	}
	if len(s.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.Tables["users"].Fields))
	}
}

func TestSchemaState_AddTable_Duplicate(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.AddTable("users", nil, nil); err == nil {
		t.Fatal("expected error for duplicate table")
	}
}

func TestSchemaState_DropTable(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.DropTable("users"); err != nil {
		t.Fatalf("DropTable: %v", err)
	}
	if _, ok := s.Tables["users"]; ok {
		t.Fatal("expected 'users' to be removed")
	}
}

func TestSchemaState_DropTable_NotExists(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.DropTable("missing"); err == nil {
		t.Fatal("expected error for missing table")
	}
}

func TestSchemaState_RenameTable(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("old", nil, nil)
	if err := s.RenameTable("old", "new"); err != nil {
		t.Fatalf("RenameTable: %v", err)
	}
	if _, ok := s.Tables["new"]; !ok {
		t.Fatal("expected 'new' table")
	}
	if _, ok := s.Tables["old"]; ok {
		t.Fatal("expected 'old' to be removed")
	}
}

func TestSchemaState_AddDropField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "id", Type: "uuid"}}, nil)
	if err := s.AddField("users", migrate.Field{Name: "email", Type: "varchar"}); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	if len(s.Tables["users"].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(s.Tables["users"].Fields))
	}
	if err := s.DropField("users", "email"); err != nil {
		t.Fatalf("DropField: %v", err)
	}
	if len(s.Tables["users"].Fields) != 1 {
		t.Fatalf("expected 1 field after drop, got %d", len(s.Tables["users"].Fields))
	}
}

func TestSchemaState_AlterField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "email", Type: "varchar", Length: 100}}, nil)
	if err := s.AlterField("users", migrate.Field{Name: "email", Type: "varchar", Length: 255}); err != nil {
		t.Fatalf("AlterField: %v", err)
	}
	if s.Tables["users"].Fields[0].Length != 255 {
		t.Fatalf("expected length 255, got %d", s.Tables["users"].Fields[0].Length)
	}
}

func TestSchemaState_RenameField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "old_col", Type: "varchar"}}, nil)
	if err := s.RenameField("users", "old_col", "new_col"); err != nil {
		t.Fatalf("RenameField: %v", err)
	}
	if s.Tables["users"].Fields[0].Name != "new_col" {
		t.Fatalf("expected 'new_col', got %q", s.Tables["users"].Fields[0].Name)
	}
}

func TestSchemaState_AddDropIndex(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.AddIndex("users", migrate.Index{Name: "idx_email", Fields: []string{"email"}, Unique: true}); err != nil {
		t.Fatalf("AddIndex: %v", err)
	}
	if len(s.Tables["users"].Indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(s.Tables["users"].Indexes))
	}
	if err := s.DropIndex("users", "idx_email"); err != nil {
		t.Fatalf("DropIndex: %v", err)
	}
	if len(s.Tables["users"].Indexes) != 0 {
		t.Fatalf("expected 0 indexes after drop, got %d", len(s.Tables["users"].Indexes))
	}
}
