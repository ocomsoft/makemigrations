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

// Error-path tests — duplicate and missing-entity guards.

func TestSchemaState_AddTable_NilFields_Normalised(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.AddTable("things", nil, nil); err != nil {
		t.Fatalf("AddTable with nil fields: %v", err)
	}
	if s.Tables["things"].Fields == nil {
		t.Fatal("expected Fields to be normalised to empty slice, got nil")
	}
	if s.Tables["things"].Indexes == nil {
		t.Fatal("expected Indexes to be normalised to empty slice, got nil")
	}
}

func TestSchemaState_RenameTable_Collision(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("a", nil, nil)
	_ = s.AddTable("b", nil, nil)
	if err := s.RenameTable("a", "b"); err == nil {
		t.Fatal("expected error when renaming onto an existing table name")
	}
}

func TestSchemaState_RenameTable_NotExists(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.RenameTable("ghost", "new"); err == nil {
		t.Fatal("expected error when source table does not exist")
	}
}

func TestSchemaState_AddField_Duplicate(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", []migrate.Field{{Name: "id", Type: "uuid"}}, nil)
	if err := s.AddField("users", migrate.Field{Name: "id", Type: "uuid"}); err == nil {
		t.Fatal("expected error for duplicate field name")
	}
}

func TestSchemaState_AddField_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.AddField("ghost", migrate.Field{Name: "col", Type: "varchar"}); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_DropField_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.DropField("ghost", "col"); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_DropField_MissingField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.DropField("users", "ghost_col"); err == nil {
		t.Fatal("expected error when field does not exist")
	}
}

func TestSchemaState_AlterField_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.AlterField("ghost", migrate.Field{Name: "col", Type: "varchar"}); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_AlterField_MissingField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.AlterField("users", migrate.Field{Name: "ghost_col", Type: "varchar"}); err == nil {
		t.Fatal("expected error when field does not exist")
	}
}

func TestSchemaState_RenameField_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.RenameField("ghost", "old", "new"); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_RenameField_MissingField(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.RenameField("users", "ghost_col", "new_col"); err == nil {
		t.Fatal("expected error when field does not exist")
	}
}

func TestSchemaState_AddIndex_Duplicate(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	_ = s.AddIndex("users", migrate.Index{Name: "idx_email", Fields: []string{"email"}})
	if err := s.AddIndex("users", migrate.Index{Name: "idx_email", Fields: []string{"email"}}); err == nil {
		t.Fatal("expected error for duplicate index name")
	}
}

func TestSchemaState_AddIndex_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.AddIndex("ghost", migrate.Index{Name: "idx_x", Fields: []string{"x"}}); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_DropIndex_MissingTable(t *testing.T) {
	s := migrate.NewSchemaState()
	if err := s.DropIndex("ghost", "idx_x"); err == nil {
		t.Fatal("expected error when table does not exist")
	}
}

func TestSchemaState_DropIndex_MissingIndex(t *testing.T) {
	s := migrate.NewSchemaState()
	_ = s.AddTable("users", nil, nil)
	if err := s.DropIndex("users", "idx_ghost"); err == nil {
		t.Fatal("expected error when index does not exist")
	}
}

func TestSchemaState_AddDropForeignKey(t *testing.T) {
	state := migrate.NewSchemaState()
	_ = state.AddTable("orders", []migrate.Field{{Name: "id", Type: "integer"}}, nil)

	fk := migrate.ForeignKeyConstraint{
		Name:            "fk_orders_user_id",
		FieldName:       "user_id",
		ReferencedTable: "users",
		OnDelete:        "CASCADE",
	}

	// AddTable should initialise ForeignKeys to empty slice, not nil
	if state.Tables["orders"].ForeignKeys == nil {
		t.Fatal("expected ForeignKeys to be initialised as empty slice, got nil")
	}

	// Add
	if err := state.AddForeignKey("orders", fk); err != nil {
		t.Fatalf("AddForeignKey: %v", err)
	}
	ts := state.Tables["orders"]
	if len(ts.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(ts.ForeignKeys))
	}

	// Duplicate
	if err := state.AddForeignKey("orders", fk); err == nil {
		t.Fatal("expected error on duplicate FK")
	}

	// Drop
	if err := state.DropForeignKey("orders", fk.Name); err != nil {
		t.Fatalf("DropForeignKey: %v", err)
	}
	if len(state.Tables["orders"].ForeignKeys) != 0 {
		t.Fatal("expected 0 FKs after drop")
	}

	// Drop non-existent
	if err := state.DropForeignKey("orders", fk.Name); err == nil {
		t.Fatal("expected error dropping non-existent FK")
	}
}

// TestSchemaState_SetTypeMappings verifies that SetTypeMappings updates state.TypeMappings.
func TestSchemaState_SetTypeMappings(t *testing.T) {
	state := migrate.NewSchemaState()
	if state.TypeMappings != nil {
		t.Errorf("expected nil TypeMappings, got %v", state.TypeMappings)
	}
	m := map[string]string{"float": "DOUBLE PRECISION"}
	state.SetTypeMappings(m)
	if state.TypeMappings["float"] != "DOUBLE PRECISION" {
		t.Errorf("expected DOUBLE PRECISION, got %q", state.TypeMappings["float"])
	}
	// Overwrite
	state.SetTypeMappings(map[string]string{"text": "NVARCHAR(MAX)"})
	if _, ok := state.TypeMappings["float"]; ok {
		t.Error("expected float key to be gone after overwrite")
	}
}
