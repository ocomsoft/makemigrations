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

// TestDiff_ForeignKeyAdded_ExistingTable verifies that adding a foreign_key field to an
// existing table emits a ChangeTypeForeignKeyAdded change in the diff.
func TestDiff_ForeignKeyAdded_ExistingTable(t *testing.T) {
	de := NewDiffEngine(false)
	old := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{{Name: "id", Type: "integer", PrimaryKey: true}}},
		},
	}
	newSchema := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{
				{Name: "id", Type: "integer", PrimaryKey: true},
				{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "auth_user", OnDelete: "PROTECT"}},
			}},
		},
	}
	diff, err := de.CompareSchemas(old, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}
	var hasFKChange bool
	for _, c := range diff.Changes {
		if c.Type == ChangeTypeForeignKeyAdded {
			hasFKChange = true
		}
	}
	if !hasFKChange {
		t.Errorf("expected ChangeTypeForeignKeyAdded in diff, changes: %v", diff.Changes)
	}
}

// TestDiff_ForeignKeyAdded_NewTable verifies that a brand-new table with a foreign_key field
// emits a ChangeTypeForeignKeyAdded change alongside the ChangeTypeTableAdded change.
func TestDiff_ForeignKeyAdded_NewTable(t *testing.T) {
	de := NewDiffEngine(false)
	old := &Schema{Tables: []Table{}}
	newSchema := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{
				{Name: "id", Type: "uuid", PrimaryKey: true},
				{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "auth_user", OnDelete: "PROTECT"}},
			}},
		},
	}
	diff, err := de.CompareSchemas(old, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}

	// Verify ordering: TableAdded must precede ForeignKeyAdded for the same table
	tableAddedIdx := -1
	fkAddedIdx := -1
	for i, c := range diff.Changes {
		if c.Type == ChangeTypeTableAdded && c.TableName == "orders" {
			tableAddedIdx = i
		}
		if c.Type == ChangeTypeForeignKeyAdded && c.TableName == "orders" {
			fkAddedIdx = i
		}
	}
	if tableAddedIdx == -1 {
		t.Error("expected ChangeTypeTableAdded in diff")
	}
	if fkAddedIdx == -1 {
		t.Error("expected ChangeTypeForeignKeyAdded in diff")
	}
	if tableAddedIdx != -1 && fkAddedIdx != -1 && tableAddedIdx >= fkAddedIdx {
		t.Errorf("expected TableAdded (idx %d) before ForeignKeyAdded (idx %d)", tableAddedIdx, fkAddedIdx)
	}
}

// TestDiff_ForeignKeyAdded_NilOldSchema verifies the oldSchema==nil fast-path also emits
// ChangeTypeForeignKeyAdded for tables that contain foreign_key fields.
func TestDiff_ForeignKeyAdded_NilOldSchema(t *testing.T) {
	de := NewDiffEngine(false)
	newSchema := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{
				{Name: "id", Type: "uuid", PrimaryKey: true},
				{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "auth_user", OnDelete: "CASCADE"}},
			}},
		},
	}
	diff, err := de.CompareSchemas(nil, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}

	// Verify ordering: ChangeTypeTableAdded must appear before ChangeTypeForeignKeyAdded
	tableAddedIdx := -1
	fkAddedIdx := -1
	for i, c := range diff.Changes {
		if c.Type == ChangeTypeTableAdded && tableAddedIdx == -1 {
			tableAddedIdx = i
		}
		if c.Type == ChangeTypeForeignKeyAdded && fkAddedIdx == -1 {
			fkAddedIdx = i
		}
	}

	if fkAddedIdx == -1 {
		t.Errorf("expected ChangeTypeForeignKeyAdded for nil-old-schema path, changes: %v", diff.Changes)
	}
	if tableAddedIdx == -1 {
		t.Errorf("expected ChangeTypeTableAdded for nil-old-schema path, changes: %v", diff.Changes)
	}
	if tableAddedIdx != -1 && fkAddedIdx != -1 && tableAddedIdx > fkAddedIdx {
		t.Errorf("ChangeTypeTableAdded (idx %d) must come before ChangeTypeForeignKeyAdded (idx %d)", tableAddedIdx, fkAddedIdx)
	}
}

// TestDiff_ForeignKeyRemoved_ExistingTable verifies that removing a foreign_key field from an
// existing table emits a ChangeTypeForeignKeyRemoved change in the diff.
func TestDiff_ForeignKeyRemoved_ExistingTable(t *testing.T) {
	de := NewDiffEngine(false)
	old := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{
				{Name: "id", Type: "integer", PrimaryKey: true},
				{Name: "user_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "auth_user", OnDelete: "PROTECT"}},
			}},
		},
	}
	newSchema := &Schema{
		Tables: []Table{
			{Name: "orders", Fields: []Field{
				{Name: "id", Type: "integer", PrimaryKey: true},
			}},
		},
	}
	diff, err := de.CompareSchemas(old, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}
	var hasFKRemoved bool
	for _, c := range diff.Changes {
		if c.Type == ChangeTypeForeignKeyRemoved {
			hasFKRemoved = true
			if !c.Destructive {
				t.Errorf("expected ChangeTypeForeignKeyRemoved to be marked Destructive")
			}
		}
	}
	if !hasFKRemoved {
		t.Errorf("expected ChangeTypeForeignKeyRemoved in diff, changes: %v", diff.Changes)
	}
}

// TestDiff_TopologicalOrder_TwoNewTablesWithFK verifies that when two new tables are added
// and one has a FK to the other, the referenced table's CreateTable comes before the
// referencing table's CreateTable in the diff changes.
func TestDiff_TopologicalOrder_TwoNewTablesWithFK(t *testing.T) {
	de := NewDiffEngine(false)
	old := &Schema{Tables: []Table{}}
	// child has FK to parent — parent must be created first
	newSchema := &Schema{
		Tables: []Table{
			{
				Name: "child",
				Fields: []Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "parent_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "parent", OnDelete: "CASCADE"}},
				},
			},
			{
				Name:   "parent",
				Fields: []Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	}
	diff, err := de.CompareSchemas(old, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}

	childCreateIdx := -1
	parentCreateIdx := -1
	childFKIdx := -1
	for i, c := range diff.Changes {
		if c.Type == ChangeTypeTableAdded && c.TableName == "child" {
			childCreateIdx = i
		}
		if c.Type == ChangeTypeTableAdded && c.TableName == "parent" {
			parentCreateIdx = i
		}
		if c.Type == ChangeTypeForeignKeyAdded && c.TableName == "child" {
			childFKIdx = i
		}
	}

	if parentCreateIdx == -1 {
		t.Fatal("expected ChangeTypeTableAdded for 'parent'")
	}
	if childCreateIdx == -1 {
		t.Fatal("expected ChangeTypeTableAdded for 'child'")
	}
	if childFKIdx == -1 {
		t.Fatal("expected ChangeTypeForeignKeyAdded for 'child'")
	}
	// parent must be created before child (topological order)
	if parentCreateIdx > childCreateIdx {
		t.Errorf("'parent' CreateTable (idx %d) must come before 'child' CreateTable (idx %d) because child has FK to parent",
			parentCreateIdx, childCreateIdx)
	}
	// FK constraint must come after both tables are created
	if childFKIdx < childCreateIdx {
		t.Errorf("'child' AddForeignKey (idx %d) must come after 'child' CreateTable (idx %d)",
			childFKIdx, childCreateIdx)
	}
	if childFKIdx < parentCreateIdx {
		t.Errorf("'child' AddForeignKey (idx %d) must come after 'parent' CreateTable (idx %d)",
			childFKIdx, parentCreateIdx)
	}
}

// TestDiff_TopologicalOrder_NilOldSchema verifies the oldSchema==nil path also topologically
// sorts tables so referenced tables come before referencing tables.
func TestDiff_TopologicalOrder_NilOldSchema_TwoNewTablesWithFK(t *testing.T) {
	de := NewDiffEngine(false)
	newSchema := &Schema{
		Tables: []Table{
			{
				Name: "child",
				Fields: []Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "parent_id", Type: "foreign_key", ForeignKey: &ForeignKey{Table: "parent", OnDelete: "CASCADE"}},
				},
			},
			{
				Name:   "parent",
				Fields: []Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	}
	diff, err := de.CompareSchemas(nil, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}

	childCreateIdx := -1
	parentCreateIdx := -1
	for i, c := range diff.Changes {
		if c.Type == ChangeTypeTableAdded && c.TableName == "child" {
			childCreateIdx = i
		}
		if c.Type == ChangeTypeTableAdded && c.TableName == "parent" {
			parentCreateIdx = i
		}
	}

	if parentCreateIdx == -1 {
		t.Fatal("expected ChangeTypeTableAdded for 'parent'")
	}
	if childCreateIdx == -1 {
		t.Fatal("expected ChangeTypeTableAdded for 'child'")
	}
	if parentCreateIdx > childCreateIdx {
		t.Errorf("'parent' CreateTable (idx %d) must come before 'child' CreateTable (idx %d)",
			parentCreateIdx, childCreateIdx)
	}
}
