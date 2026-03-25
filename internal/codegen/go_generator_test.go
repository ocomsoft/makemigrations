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

package codegen_test

import (
	"go/format"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/yaml"
)

func TestGoGenerator_GenerateMigration_CreateTable(t *testing.T) {
	g := codegen.NewGoGenerator()

	table := yaml.Table{
		Name: "users",
		Fields: []yaml.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "email", Type: "varchar", Length: 255},
		},
	}
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeTableAdded,
				TableName: "users",
				NewValue:  table,
			},
		},
	}

	src, err := g.GenerateMigration("0001_initial", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if src == "" {
		t.Fatal("expected non-empty source")
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "0001_initial") {
		t.Error("expected migration name in output")
	}
	if !strings.Contains(src, "CreateTable") {
		t.Error("expected CreateTable in output")
	}
	if !strings.Contains(src, `"users"`) {
		t.Error("expected table name 'users' in output")
	}
}

func TestGoGenerator_GenerateMigration_AddField(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "phone",
				NewValue:  yaml.Field{Name: "phone", Type: "varchar", Length: 20},
			},
		},
	}
	src, err := g.GenerateMigration("0002_add_phone", []string{"0001_initial"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "AddField") {
		t.Error("expected AddField in output")
	}
	if !strings.Contains(src, `"0001_initial"`) {
		t.Error("expected dependency in output")
	}
}

func TestGoGenerator_GenerateMigration_ValidGoFormat(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableRemoved, TableName: "old_table", OldValue: yaml.Table{Name: "old_table"}},
		},
	}
	src, err := g.GenerateMigration("0003_drop_table", []string{"0002_add_phone"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
}

func TestGoGenerator_GenerateMainGo(t *testing.T) {
	g := codegen.NewGoGenerator()
	src := g.GenerateMainGo()
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("main.go is not valid Go: %v", err)
	}
	if !strings.Contains(src, "func main()") {
		t.Error("expected func main() in output")
	}
	if !strings.Contains(src, "m.NewApp") {
		t.Error("expected m.NewApp in output")
	}
}

func TestGoGenerator_GenerateGoMod(t *testing.T) {
	g := codegen.NewGoGenerator()
	src := g.GenerateGoMod("myproject/migrations", "main", "1.25")
	if !strings.Contains(src, "module myproject/migrations") {
		t.Error("expected module declaration")
	}
	if !strings.Contains(src, "github.com/ocomsoft/makemigrations") {
		t.Error("expected makemigrations dependency")
	}
}

func TestGoGenerator_GenerateMigration_DropField(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldRemoved,
				TableName: "users",
				FieldName: "phone",
				OldValue:  yaml.Field{Name: "phone", Type: "varchar", Length: 20},
			},
		},
	}
	src, err := g.GenerateMigration("0004_drop_phone", []string{"0003_drop_table"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "DropField") {
		t.Error("expected DropField in output")
	}
	if !strings.Contains(src, `"phone"`) {
		t.Error("expected field name 'phone' in output")
	}
}

func TestGoGenerator_GenerateMigration_AlterField_NoSchemas(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldModified,
				TableName: "users",
				FieldName: "email",
				OldValue:  "varchar",
				NewValue:  "text",
			},
		},
	}
	src, err := g.GenerateMigration("0005_alter_email", []string{"0004_drop_phone"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "AlterField") {
		t.Error("expected AlterField in output")
	}
}

func TestGoGenerator_GenerateMigration_AlterField_WithSchemas(t *testing.T) {
	g := codegen.NewGoGenerator()

	prevSchema := &yaml.Schema{
		Tables: []yaml.Table{
			{
				Name: "users",
				Fields: []yaml.Field{
					{Name: "email", Type: "varchar", Length: 100},
				},
			},
		},
	}
	currSchema := &yaml.Schema{
		Tables: []yaml.Table{
			{
				Name: "users",
				Fields: []yaml.Field{
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}

	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldModified,
				TableName: "users",
				FieldName: "email",
				OldValue:  100,
				NewValue:  255,
			},
		},
	}
	src, err := g.GenerateMigration("0006_widen_email", []string{"0005_alter_email"}, diff, currSchema, prevSchema, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "AlterField") {
		t.Error("expected AlterField in output")
	}
	if !strings.Contains(src, "Length: 255") {
		t.Error("expected Length: 255 in new field")
	}
	if !strings.Contains(src, "Length: 100") {
		t.Error("expected Length: 100 in old field")
	}
}

func TestGoGenerator_GenerateMigration_AddIndex(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeIndexAdded,
				TableName: "users",
				FieldName: "idx_users_email",
				NewValue:  yaml.Index{Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
			},
		},
	}
	src, err := g.GenerateMigration("0007_add_index", []string{"0006_widen_email"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "AddIndex") {
		t.Error("expected AddIndex in output")
	}
	if !strings.Contains(src, "idx_users_email") {
		t.Error("expected index name in output")
	}
	if !strings.Contains(src, "Unique: true") {
		t.Error("expected Unique: true in output")
	}
}

func TestGoGenerator_GenerateMigration_DropIndex(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeIndexRemoved,
				TableName: "users",
				FieldName: "idx_users_email",
				OldValue:  yaml.Index{Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
			},
		},
	}
	src, err := g.GenerateMigration("0008_drop_index", []string{"0007_add_index"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "DropIndex") {
		t.Error("expected DropIndex in output")
	}
}

func TestGoGenerator_GenerateMigration_RenameTable(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeTableRenamed,
				TableName: "users",
				NewValue:  "accounts",
			},
		},
	}
	src, err := g.GenerateMigration("0009_rename_table", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "RenameTable") {
		t.Error("expected RenameTable in output")
	}
	if !strings.Contains(src, `"accounts"`) {
		t.Error("expected new table name in output")
	}
}

func TestGoGenerator_GenerateMigration_RenameField(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldRenamed,
				TableName: "users",
				FieldName: "email",
				NewValue:  "email_address",
			},
		},
	}
	src, err := g.GenerateMigration("0010_rename_field", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "RenameField") {
		t.Error("expected RenameField in output")
	}
	if !strings.Contains(src, `"email_address"`) {
		t.Error("expected new field name in output")
	}
}

func TestGoGenerator_GenerateMigration_NilDiff(t *testing.T) {
	g := codegen.NewGoGenerator()
	_, err := g.GenerateMigration("test", nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil diff")
	}
}

func TestGoGenerator_GenerateMigration_NoChanges(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{HasChanges: false}
	_, err := g.GenerateMigration("test", nil, diff, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for no changes")
	}
}

func TestGoGenerator_GenerateMigration_CreateTableWithIndexes(t *testing.T) {
	g := codegen.NewGoGenerator()

	table := yaml.Table{
		Name: "products",
		Fields: []yaml.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "name", Type: "varchar", Length: 100},
			{Name: "price", Type: "decimal", Precision: 10, Scale: 2},
		},
		Indexes: []yaml.Index{
			{Name: "idx_products_name", Fields: []string{"name"}, Unique: false},
		},
	}
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableAdded, TableName: "products", NewValue: table},
		},
	}

	src, err := g.GenerateMigration("0011_products", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "idx_products_name") {
		t.Error("expected index name in output")
	}
	if !strings.Contains(src, "Precision: 10") {
		t.Error("expected Precision: 10 in output")
	}
	if !strings.Contains(src, "Scale: 2") {
		t.Error("expected Scale: 2 in output")
	}
}

func TestGoGenerator_GenerateMigration_FieldWithForeignKey(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "orders",
				FieldName: "user_id",
				NewValue: yaml.Field{
					Name: "user_id",
					Type: "foreign_key",
					ForeignKey: &yaml.ForeignKey{
						Table:    "users",
						OnDelete: "CASCADE",
					},
				},
			},
		},
	}
	src, err := g.GenerateMigration("0012_fk", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "ForeignKey") {
		t.Error("expected ForeignKey in output")
	}
	if !strings.Contains(src, "CASCADE") {
		t.Error("expected CASCADE in output")
	}
}

func TestGoGenerator_GenerateMigration_MultipleDependencies(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableRemoved, TableName: "temp"},
		},
	}
	src, err := g.GenerateMigration("0013_multi_deps", []string{"0001_initial", "0002_add_phone"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, `"0001_initial"`) {
		t.Error("expected first dependency")
	}
	if !strings.Contains(src, `"0002_add_phone"`) {
		t.Error("expected second dependency")
	}
}

func TestGoGenerator_GenerateMigration_NullableDefaultIsTrue(t *testing.T) {
	g := codegen.NewGoGenerator()
	// When yaml.Field.Nullable is nil (the default), it means nullable=true.
	// The generator must emit Nullable: true in the m.Field{} literal.
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "bio",
				NewValue:  yaml.Field{Name: "bio", Type: "text"}, // Nullable is nil
			},
		},
	}
	src, err := g.GenerateMigration("0010_add_bio", []string{"0001_initial"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "Nullable: true") {
		t.Errorf("expected 'Nullable: true' in output for nil Nullable field, got:\n%s", src)
	}
}

func TestGoGenerator_GenerateMigration_ExplicitNotNullable(t *testing.T) {
	g := codegen.NewGoGenerator()
	notNullable := false
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "email",
				NewValue:  yaml.Field{Name: "email", Type: "varchar", Nullable: &notNullable},
			},
		},
	}
	src, err := g.GenerateMigration("0011_add_email", []string{"0001_initial"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if strings.Contains(src, "Nullable: true") {
		t.Errorf("expected no 'Nullable: true' for explicit false Nullable, got:\n%s", src)
	}
}

func TestGoGenerator_GenerateMigration_DropIndex_EmptyName(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeIndexRemoved,
				TableName: "users",
				FieldName: "", // empty index name
			},
		},
	}
	_, err := g.GenerateMigration("0012_bad_drop_index", []string{}, diff, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty index name in drop_index")
	}
	if !strings.Contains(err.Error(), "empty index name") {
		t.Errorf("expected 'empty index name' in error, got: %v", err)
	}
}

func TestMigrationFileName(t *testing.T) {
	got := codegen.MigrationFileName("0001_initial")
	want := "0001_initial.go"
	if got != want {
		t.Errorf("MigrationFileName: got %q, want %q", got, want)
	}
}

func TestNextMigrationNumber(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "0001"},
		{1, "0002"},
		{9, "0010"},
		{99, "0100"},
		{9999, "10000"},
	}
	for _, tt := range tests {
		got := codegen.NextMigrationNumber(tt.count)
		if got != tt.want {
			t.Errorf("NextMigrationNumber(%d): got %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestGoGenerator_DropField_PromptOmit_EmitsSchemaOnly(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeFieldRemoved, TableName: "users", FieldName: "phone"},
		},
	}
	decisions := map[int]yaml.PromptResponse{0: yaml.PromptOmit}
	src, err := g.GenerateMigration("0020_drop_phone_deferred", []string{}, diff, nil, nil, decisions)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "SchemaOnly: true") {
		t.Errorf("expected SchemaOnly: true in output, got:\n%s", src)
	}
}

func TestGoGenerator_DropTable_PromptOmit_EmitsSchemaOnly(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableRemoved, TableName: "old_table"},
		},
	}
	decisions := map[int]yaml.PromptResponse{0: yaml.PromptOmit}
	src, err := g.GenerateMigration("0021_drop_table_deferred", []string{}, diff, nil, nil, decisions)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "SchemaOnly: true") {
		t.Errorf("expected SchemaOnly: true in output, got:\n%s", src)
	}
}

func TestGoGenerator_DropField_PromptReview_EmitsComment(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeFieldRemoved, TableName: "users", FieldName: "phone"},
		},
	}
	decisions := map[int]yaml.PromptResponse{0: yaml.PromptReview}
	src, err := g.GenerateMigration("0022_drop_phone_review", []string{}, diff, nil, nil, decisions)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "// REVIEW:") {
		t.Errorf("expected // REVIEW: comment in output, got:\n%s", src)
	}
	// Should NOT have SchemaOnly for a Review decision.
	if strings.Contains(src, "SchemaOnly") {
		t.Error("expected no SchemaOnly for PromptReview")
	}
}

func TestGoGenerator_DropField_NilDecisions_Normal(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeFieldRemoved, TableName: "users", FieldName: "phone"},
		},
	}
	src, err := g.GenerateMigration("0023_drop_phone_normal", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if strings.Contains(src, "SchemaOnly") {
		t.Error("expected no SchemaOnly when decisions is nil")
	}
	if strings.Contains(src, "// REVIEW:") {
		t.Error("expected no REVIEW comment when decisions is nil")
	}
}

func TestGoGenerator_CreateTable_PromptOmit_EmitsSchemaOnly(t *testing.T) {
	g := codegen.NewGoGenerator()
	table := yaml.Table{
		Name:   "users",
		Fields: []yaml.Field{{Name: "id", Type: "integer"}},
	}
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{Type: yaml.ChangeTypeTableAdded, TableName: "users", NewValue: table},
		},
	}
	decisions := map[int]yaml.PromptResponse{0: yaml.PromptOmit}
	src, err := g.GenerateMigration("0030_schema_state", []string{}, diff, nil, nil, decisions)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "SchemaOnly: true") {
		t.Errorf("expected SchemaOnly: true in CreateTable output, got:\n%s", src)
	}
}

func TestGoGenerator_AddField_PromptOmit_EmitsSchemaOnly(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:      yaml.ChangeTypeFieldAdded,
				TableName: "users",
				FieldName: "email",
				NewValue:  yaml.Field{Name: "email", Type: "varchar"},
			},
		},
	}
	decisions := map[int]yaml.PromptResponse{0: yaml.PromptOmit}
	src, err := g.GenerateMigration("0031_schema_state_field", []string{}, diff, nil, nil, decisions)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if !strings.Contains(src, "SchemaOnly: true") {
		t.Errorf("expected SchemaOnly: true in AddField output, got:\n%s", src)
	}
}

// TestGoGenerator_SetTypeMappings verifies that a ChangeTypeTypeMappingsModified change
// generates a valid &m.SetTypeMappings{...} literal with sorted keys.
func TestGoGenerator_SetTypeMappings(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:        yaml.ChangeTypeTypeMappingsModified,
				Description: "Update schema type mappings",
				NewValue: map[string]string{
					"float": "DOUBLE PRECISION",
					"text":  "NVARCHAR(MAX)",
				},
			},
		},
	}
	src, err := g.GenerateMigration("0001_set_type_mappings", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "SetTypeMappings") {
		t.Error("expected SetTypeMappings in output")
	}
	if !strings.Contains(src, `"float"`) || !strings.Contains(src, `"DOUBLE PRECISION"`) {
		t.Errorf("expected float mapping in output, got:\n%s", src)
	}
	if !strings.Contains(src, `"text"`) || !strings.Contains(src, `"NVARCHAR(MAX)"`) {
		t.Errorf("expected text mapping in output, got:\n%s", src)
	}
	// Keys must be sorted: "float" before "text"
	floatIdx := strings.Index(src, `"float"`)
	textIdx := strings.Index(src, `"text"`)
	if floatIdx > textIdx {
		t.Errorf("expected keys sorted alphabetically ('float' before 'text'), got:\n%s", src)
	}
}

// TestGoGenerator_AddForeignKey verifies that a ChangeTypeForeignKeyAdded change
// generates a valid &m.AddForeignKey{...} literal with the correct fields.
func TestGoGenerator_AddForeignKey(t *testing.T) {
	g := codegen.NewGoGenerator()
	change := yaml.Change{
		Type:      yaml.ChangeTypeForeignKeyAdded,
		TableName: "orders",
		FieldName: "user_id",
		NewValue: yaml.Field{
			Name:       "user_id",
			Type:       "foreign_key",
			ForeignKey: &yaml.ForeignKey{Table: "auth_user", OnDelete: "PROTECT"},
		},
	}
	diff := &yaml.SchemaDiff{HasChanges: true, Changes: []yaml.Change{change}}
	code, err := g.GenerateMigration("0035_add_user_fk", []string{"0034_update_schema"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, code)
	}
	if !strings.Contains(code, "&m.AddForeignKey{") {
		t.Errorf("expected &m.AddForeignKey{} in output, got:\n%s", code)
	}
	if !strings.Contains(code, "ReferencedTable:") || !strings.Contains(code, `"auth_user"`) {
		t.Errorf("expected ReferencedTable: \"auth_user\", got:\n%s", code)
	}
	if !strings.Contains(code, "OnDelete:") || !strings.Contains(code, `"PROTECT"`) {
		t.Errorf("expected OnDelete: \"PROTECT\", got:\n%s", code)
	}
	if !strings.Contains(code, "ConstraintName:") || !strings.Contains(code, `"fk_orders_user_id"`) {
		t.Errorf("expected ConstraintName: \"fk_orders_user_id\", got:\n%s", code)
	}
}

// TestGoGenerator_DropForeignKey verifies that a ChangeTypeForeignKeyRemoved change
// generates a valid &m.DropForeignKey{...} literal with the constraint name.
func TestGoGenerator_DropForeignKey(t *testing.T) {
	g := codegen.NewGoGenerator()
	change := yaml.Change{
		Type:      yaml.ChangeTypeForeignKeyRemoved,
		TableName: "orders",
		FieldName: "user_id",
		OldValue: yaml.Field{
			Name:       "user_id",
			Type:       "foreign_key",
			ForeignKey: &yaml.ForeignKey{Table: "auth_user", OnDelete: "PROTECT"},
		},
	}
	diff := &yaml.SchemaDiff{HasChanges: true, Changes: []yaml.Change{change}}
	code, err := g.GenerateMigration("0036_drop_user_fk", []string{"0035_add_user_fk"}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, code)
	}
	if !strings.Contains(code, "&m.DropForeignKey{") {
		t.Errorf("expected &m.DropForeignKey{} in output, got:\n%s", code)
	}
	if !strings.Contains(code, `ConstraintName: "fk_orders_user_id"`) {
		t.Errorf("expected ConstraintName in output, got:\n%s", code)
	}
}

// TestGoGenerator_SetDefaults verifies that a ChangeTypeDefaultsModified change
// generates a valid &m.SetDefaults{...} literal with sorted keys.
func TestGoGenerator_SetDefaults(t *testing.T) {
	g := codegen.NewGoGenerator()
	diff := &yaml.SchemaDiff{
		HasChanges: true,
		Changes: []yaml.Change{
			{
				Type:        yaml.ChangeTypeDefaultsModified,
				Description: "Set initial schema defaults",
				NewValue: map[string]string{
					"uuid": "uuid_generate_v4()",
					"now":  "CURRENT_TIMESTAMP",
				},
			},
		},
	}
	src, err := g.GenerateMigration("0001_set_defaults", []string{}, diff, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, src)
	}
	if !strings.Contains(src, "SetDefaults") {
		t.Error("expected SetDefaults in output")
	}
	// gofmt aligns map values with spaces, so check key and value separately
	if !strings.Contains(src, `"uuid"`) || !strings.Contains(src, `"uuid_generate_v4()"`) {
		t.Errorf("expected uuid mapping in output, got:\n%s", src)
	}
	if !strings.Contains(src, `"now"`) || !strings.Contains(src, `"CURRENT_TIMESTAMP"`) {
		t.Errorf("expected now mapping in output, got:\n%s", src)
	}
	// Keys must be sorted: "now" before "uuid"
	nowIdx := strings.Index(src, `"now"`)
	uuidIdx := strings.Index(src, `"uuid"`)
	if nowIdx > uuidIdx {
		t.Errorf("expected keys sorted alphabetically ('now' before 'uuid'), got:\n%s", src)
	}
}

// TestGoGenerator_NewTableWithForeignKey_CorrectOrder verifies that when creating a new table
// with foreign key fields, the diff engine emits ChangeTypeTableAdded followed by
// ChangeTypeForeignKeyAdded, and the generator produces CreateTable before AddForeignKey.
func TestGoGenerator_NewTableWithForeignKey_CorrectOrder(t *testing.T) {
	g := codegen.NewGoGenerator()
	de := yaml.NewDiffEngine(false)
	old := &yaml.Schema{Tables: []yaml.Table{}}
	newSchema := &yaml.Schema{
		Tables: []yaml.Table{
			{
				Name: "orders",
				Fields: []yaml.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key",
						ForeignKey: &yaml.ForeignKey{Table: "auth_user", OnDelete: "PROTECT"}},
				},
			},
		},
	}
	diff, err := de.CompareSchemas(old, newSchema)
	if err != nil {
		t.Fatalf("CompareSchemas: %v", err)
	}
	code, err := g.GenerateMigration("0001_initial", []string{}, diff, newSchema, old, nil)
	if err != nil {
		t.Fatalf("GenerateMigration: %v", err)
	}
	if _, err := format.Source([]byte(code)); err != nil {
		t.Fatalf("output is not valid Go: %v\nSource:\n%s", err, code)
	}
	if !strings.Contains(code, "&m.CreateTable{") {
		t.Errorf("expected CreateTable in output, got:\n%s", code)
	}
	if !strings.Contains(code, "&m.AddForeignKey{") {
		t.Errorf("expected AddForeignKey in output, got:\n%s", code)
	}
	// CreateTable must appear before AddForeignKey
	ctPos := strings.Index(code, "&m.CreateTable{")
	fkPos := strings.Index(code, "&m.AddForeignKey{")
	if ctPos >= fkPos {
		t.Errorf("expected CreateTable before AddForeignKey; ctPos=%d fkPos=%d\n%s", ctPos, fkPos, code)
	}
}
