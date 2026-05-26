package migrate

import (
	"testing"
)

// TestIsDropOp verifies that isDropOp correctly identifies operations that
// remove a database object (drop_table, drop_field, drop_index, drop_foreign_key)
// and returns false for all other operation types.
func TestIsDropOp(t *testing.T) {
	tests := []struct {
		op   Operation
		want bool
	}{
		{&DropTable{Name: "t"}, true},
		{&DropField{Table: "t", Field: "f"}, true},
		{&DropIndex{Table: "t", Index: "i"}, true},
		{&DropForeignKey{Table: "t", ConstraintName: "fk"}, true},
		{&CreateTable{Name: "t"}, false},
		{&AddField{Table: "t"}, false},
		{&AddIndex{Table: "t"}, false},
		{&AddForeignKey{Table: "t"}, false},
		{&AlterField{Table: "t"}, false},
		{&RenameField{Table: "t"}, false},
		{&RenameTable{OldName: "a", NewName: "b"}, false},
		{&RunSQL{ForwardSQL: "SELECT 1"}, false},
		{&SetDefaults{Defaults: map[string]string{}}, false},
		{&SetTypeMappings{TypeMappings: map[string]string{}}, false},
		{&UpsertData{Table: "t"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.op.TypeName(), func(t *testing.T) {
			got := isDropOp(tt.op)
			if got != tt.want {
				t.Errorf("isDropOp(%s) = %v, want %v", tt.op.TypeName(), got, tt.want)
			}
		})
	}
}

// TestIsCreateOp verifies that isCreateOp correctly identifies operations that
// create a database object in their Up direction (create_table, add_field,
// add_index, add_foreign_key) and returns false for all other operation types.
// During rollback, these operations' Down SQL drops objects.
func TestIsCreateOp(t *testing.T) {
	tests := []struct {
		op   Operation
		want bool
	}{
		{&CreateTable{Name: "t"}, true},
		{&AddField{Table: "t"}, true},
		{&AddIndex{Table: "t"}, true},
		{&AddForeignKey{Table: "t"}, true},
		{&DropTable{Name: "t"}, false},
		{&DropField{Table: "t", Field: "f"}, false},
		{&DropIndex{Table: "t", Index: "i"}, false},
		{&DropForeignKey{Table: "t", ConstraintName: "fk"}, false},
		{&AlterField{Table: "t"}, false},
		{&RenameField{Table: "t"}, false},
		{&RenameTable{OldName: "a", NewName: "b"}, false},
		{&RunSQL{ForwardSQL: "SELECT 1"}, false},
		{&SetDefaults{Defaults: map[string]string{}}, false},
		{&SetTypeMappings{TypeMappings: map[string]string{}}, false},
		{&UpsertData{Table: "t"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.op.TypeName(), func(t *testing.T) {
			got := isCreateOp(tt.op)
			if got != tt.want {
				t.Errorf("isCreateOp(%s) = %v, want %v", tt.op.TypeName(), got, tt.want)
			}
		})
	}
}

// TestIsDropOp_and_IsCreateOp_mutually_exclusive verifies that no operation
// is classified as both a drop op and a create op — these categories are
// mutually exclusive by design.
func TestIsDropOp_and_IsCreateOp_mutually_exclusive(t *testing.T) {
	ops := []Operation{
		&CreateTable{Name: "t"},
		&DropTable{Name: "t"},
		&AddField{Table: "t"},
		&DropField{Table: "t", Field: "f"},
		&AddIndex{Table: "t"},
		&DropIndex{Table: "t", Index: "i"},
		&AddForeignKey{Table: "t"},
		&DropForeignKey{Table: "t", ConstraintName: "fk"},
		&AlterField{Table: "t"},
		&RenameField{Table: "t"},
		&RenameTable{OldName: "a", NewName: "b"},
		&RunSQL{ForwardSQL: "SELECT 1"},
		&SetDefaults{Defaults: map[string]string{}},
		&SetTypeMappings{TypeMappings: map[string]string{}},
		&UpsertData{Table: "t"},
	}

	for _, op := range ops {
		drop := isDropOp(op)
		create := isCreateOp(op)
		if drop && create {
			t.Errorf("operation %s is classified as both drop and create", op.TypeName())
		}
	}
}
