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

// Package codegen generates Go source files for the migration framework.
// It converts yaml.SchemaDiff changes into compilable Go code that registers
// migrate.Migration objects via init() functions.
package codegen

import (
	"fmt"
	"go/format"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/yaml"
)

// GoGenerator produces Go source code for migration files, main.go, and go.mod.
type GoGenerator struct{}

// NewGoGenerator creates a new GoGenerator instance.
func NewGoGenerator() *GoGenerator {
	return &GoGenerator{}
}

// MigrationFileName returns the .go file name for a migration name.
func MigrationFileName(name string) string {
	return name + ".go"
}

// NextMigrationNumber returns a zero-padded 4-digit number for the next migration.
func NextMigrationNumber(count int) string {
	return fmt.Sprintf("%04d", count+1)
}

// GenerateMigration generates a complete .go file that registers a single Migration
// with the global registry via an init() function. The file is in package main and
// imports the migrate package aliased as "m". currentSchema and previousSchema are
// optional and used for AlterField operations when available.
func (g *GoGenerator) GenerateMigration(
	name string,
	deps []string,
	diff *yaml.SchemaDiff,
	currentSchema, previousSchema *yaml.Schema,
) (string, error) {
	if diff == nil || !diff.HasChanges {
		return "", fmt.Errorf("no changes to generate migration for")
	}

	var b strings.Builder

	// File header
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString("\tm \"github.com/ocomsoft/makemigrations/migrate\"\n")
	b.WriteString(")\n\n")

	// init() function
	b.WriteString("func init() {\n")
	b.WriteString("\tm.Register(&m.Migration{\n")
	b.WriteString(fmt.Sprintf("\t\tName: %q,\n", name))

	// Dependencies
	b.WriteString("\t\tDependencies: []string{")
	if len(deps) > 0 {
		depStrs := make([]string, len(deps))
		for i, d := range deps {
			depStrs[i] = fmt.Sprintf("%q", d)
		}
		b.WriteString(strings.Join(depStrs, ", "))
	}
	b.WriteString("},\n")

	// Operations
	b.WriteString("\t\tOperations: []m.Operation{\n")
	for _, change := range diff.Changes {
		op, err := g.generateOperation(change, currentSchema, previousSchema)
		if err != nil {
			return "", fmt.Errorf("generating operation for change %s on %s: %w",
				change.Type, change.TableName, err)
		}
		b.WriteString(op)
	}
	b.WriteString("\t\t},\n")

	b.WriteString("\t})\n")
	b.WriteString("}\n")

	// Format the output using go/format so the result is always valid gofmt output
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String(), fmt.Errorf("formatting generated code: %w", err)
	}
	return string(formatted), nil
}

// generateOperation converts a single yaml.Change into the Go source literal
// for one migrate.Operation (e.g. &m.CreateTable{...}).
func (g *GoGenerator) generateOperation(
	change yaml.Change,
	currentSchema, previousSchema *yaml.Schema,
) (string, error) {
	switch change.Type {
	case yaml.ChangeTypeTableAdded:
		return g.generateCreateTable(change)
	case yaml.ChangeTypeTableRemoved:
		return g.generateDropTable(change)
	case yaml.ChangeTypeTableRenamed:
		return g.generateRenameTable(change)
	case yaml.ChangeTypeFieldAdded:
		return g.generateAddField(change)
	case yaml.ChangeTypeFieldRemoved:
		return g.generateDropField(change)
	case yaml.ChangeTypeFieldModified:
		return g.generateAlterField(change, currentSchema, previousSchema)
	case yaml.ChangeTypeFieldRenamed:
		return g.generateRenameField(change)
	case yaml.ChangeTypeIndexAdded:
		return g.generateAddIndex(change)
	case yaml.ChangeTypeIndexRemoved:
		return g.generateDropIndex(change)
	default:
		return "", fmt.Errorf("unsupported change type: %s", change.Type)
	}
}

// generateCreateTable emits a &m.CreateTable{...} literal.
func (g *GoGenerator) generateCreateTable(change yaml.Change) (string, error) {
	table, ok := change.NewValue.(yaml.Table)
	if !ok {
		return "", fmt.Errorf("expected yaml.Table for NewValue, got %T", change.NewValue)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\t\t\t&m.CreateTable{\n\t\t\t\tName: %q,\n", table.Name))

	// Fields
	if len(table.Fields) > 0 {
		b.WriteString("\t\t\t\tFields: []m.Field{\n")
		for _, f := range table.Fields {
			b.WriteString("\t\t\t\t\t")
			b.WriteString(generateFieldLiteral(f))
			b.WriteString(",\n")
		}
		b.WriteString("\t\t\t\t},\n")
	}

	// Indexes
	if len(table.Indexes) > 0 {
		b.WriteString("\t\t\t\tIndexes: []m.Index{\n")
		for _, idx := range table.Indexes {
			b.WriteString("\t\t\t\t\t")
			b.WriteString(generateIndexLiteral(idx))
			b.WriteString(",\n")
		}
		b.WriteString("\t\t\t\t},\n")
	}

	b.WriteString("\t\t\t},\n")
	return b.String(), nil
}

// generateDropTable emits a &m.DropTable{...} literal.
func (g *GoGenerator) generateDropTable(change yaml.Change) (string, error) {
	return fmt.Sprintf("\t\t\t&m.DropTable{Name: %q},\n", change.TableName), nil
}

// generateRenameTable emits a &m.RenameTable{...} literal.
func (g *GoGenerator) generateRenameTable(change yaml.Change) (string, error) {
	newName, ok := change.NewValue.(string)
	if !ok {
		return "", fmt.Errorf("expected string for NewValue in table rename, got %T", change.NewValue)
	}
	return fmt.Sprintf("\t\t\t&m.RenameTable{OldName: %q, NewName: %q},\n",
		change.TableName, newName), nil
}

// generateAddField emits a &m.AddField{...} literal.
func (g *GoGenerator) generateAddField(change yaml.Change) (string, error) {
	field, ok := change.NewValue.(yaml.Field)
	if !ok {
		return "", fmt.Errorf("expected yaml.Field for NewValue, got %T", change.NewValue)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\t\t\t&m.AddField{\n\t\t\t\tTable: %q,\n", change.TableName))
	b.WriteString("\t\t\t\tField: ")
	b.WriteString(generateFieldLiteral(field))
	b.WriteString(",\n\t\t\t},\n")
	return b.String(), nil
}

// generateDropField emits a &m.DropField{...} literal.
func (g *GoGenerator) generateDropField(change yaml.Change) (string, error) {
	return fmt.Sprintf("\t\t\t&m.DropField{Table: %q, Field: %q},\n",
		change.TableName, change.FieldName), nil
}

// generateAlterField emits a &m.AlterField{...} literal. When schemas are available,
// it looks up the full old and new field definitions. Otherwise it falls back to
// constructing a minimal field from the change metadata.
func (g *GoGenerator) generateAlterField(
	change yaml.Change,
	currentSchema, previousSchema *yaml.Schema,
) (string, error) {
	var oldField, newField *yaml.Field

	// Try to get full field definitions from the schemas
	if currentSchema != nil && previousSchema != nil {
		nf := g.lookupField(currentSchema, change.TableName, change.FieldName)
		newField = &nf
		of := g.lookupField(previousSchema, change.TableName, change.FieldName)
		oldField = &of
	}

	// For the no-schema fallback, only string OldValue/NewValue are reliable.
	// Other types (bool, int, *ForeignKey) in OldValue/NewValue indicate a complex
	// change that requires schema context to generate correctly. We emit an empty
	// Type in those cases — the caller should always provide schemas for field_modified.
	oldType := ""
	if s, ok := change.OldValue.(string); ok {
		oldType = s
	}
	newType := ""
	if s, ok := change.NewValue.(string); ok {
		newType = s
	}

	if oldField == nil {
		f := yaml.Field{Name: change.FieldName, Type: oldType}
		oldField = &f
	}
	if newField == nil {
		f := yaml.Field{Name: change.FieldName, Type: newType}
		newField = &f
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\t\t\t&m.AlterField{\n\t\t\t\tTable: %q,\n", change.TableName))
	b.WriteString("\t\t\t\tOldField: ")
	b.WriteString(generateFieldLiteral(*oldField))
	b.WriteString(",\n\t\t\t\tNewField: ")
	b.WriteString(generateFieldLiteral(*newField))
	b.WriteString(",\n\t\t\t},\n")
	return b.String(), nil
}

// lookupField finds a field by table and field name in a schema.
// Returns a zero-value Field if not found.
func (g *GoGenerator) lookupField(schema *yaml.Schema, tableName, fieldName string) yaml.Field {
	if schema == nil {
		return yaml.Field{Name: fieldName}
	}
	table := schema.GetTableByName(tableName)
	if table == nil {
		return yaml.Field{Name: fieldName}
	}
	f := table.GetFieldByName(fieldName)
	if f == nil {
		return yaml.Field{Name: fieldName}
	}
	return *f
}

// generateRenameField emits a &m.RenameField{...} literal.
func (g *GoGenerator) generateRenameField(change yaml.Change) (string, error) {
	newName, ok := change.NewValue.(string)
	if !ok {
		return "", fmt.Errorf("expected string for NewValue in field rename, got %T", change.NewValue)
	}
	return fmt.Sprintf("\t\t\t&m.RenameField{Table: %q, OldName: %q, NewName: %q},\n",
		change.TableName, change.FieldName, newName), nil
}

// generateAddIndex emits a &m.AddIndex{...} literal.
func (g *GoGenerator) generateAddIndex(change yaml.Change) (string, error) {
	idx, ok := change.NewValue.(yaml.Index)
	if !ok {
		return "", fmt.Errorf("expected yaml.Index for NewValue, got %T", change.NewValue)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\t\t\t&m.AddIndex{\n\t\t\t\tTable: %q,\n", change.TableName))
	b.WriteString("\t\t\t\tIndex: ")
	b.WriteString(generateIndexLiteral(idx))
	b.WriteString(",\n\t\t\t},\n")
	return b.String(), nil
}

// generateDropIndex emits a &m.DropIndex{...} literal.
func (g *GoGenerator) generateDropIndex(change yaml.Change) (string, error) {
	if change.FieldName == "" {
		return "", fmt.Errorf("drop_index change for table %q has empty index name", change.TableName)
	}
	return fmt.Sprintf("\t\t\t&m.DropIndex{Table: %q, Index: %q},\n",
		change.TableName, change.FieldName), nil
}

// generateFieldLiteral converts a yaml.Field to a m.Field{...} Go literal string.
// Only non-zero/non-false fields are included to keep the output clean.
func generateFieldLiteral(f yaml.Field) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Name: %q", f.Name))

	if f.Type != "" {
		parts = append(parts, fmt.Sprintf("Type: %q", f.Type))
	}
	if f.PrimaryKey {
		parts = append(parts, "PrimaryKey: true")
	}
	// yaml.Field.Nullable is *bool: nil means "nullable by default" (true),
	// explicit false means NOT NULL. migrate.Field.Nullable is bool with false as zero-value.
	// We must emit Nullable: true for nil (default) and *true; omit for *false.
	if f.Nullable == nil || *f.Nullable {
		parts = append(parts, "Nullable: true")
	}
	if f.Default != "" {
		parts = append(parts, fmt.Sprintf("Default: %q", f.Default))
	}
	if f.Length > 0 {
		parts = append(parts, fmt.Sprintf("Length: %d", f.Length))
	}
	if f.Precision > 0 {
		parts = append(parts, fmt.Sprintf("Precision: %d", f.Precision))
	}
	if f.Scale > 0 {
		parts = append(parts, fmt.Sprintf("Scale: %d", f.Scale))
	}
	if f.AutoCreate {
		parts = append(parts, "AutoCreate: true")
	}
	if f.AutoUpdate {
		parts = append(parts, "AutoUpdate: true")
	}
	if f.ForeignKey != nil {
		fkParts := []string{fmt.Sprintf("Table: %q", f.ForeignKey.Table)}
		if f.ForeignKey.OnDelete != "" {
			fkParts = append(fkParts, fmt.Sprintf("OnDelete: %q", f.ForeignKey.OnDelete))
		}
		parts = append(parts, fmt.Sprintf("ForeignKey: &m.ForeignKey{%s}", strings.Join(fkParts, ", ")))
	}
	if f.ManyToMany != nil {
		parts = append(parts, fmt.Sprintf("ManyToMany: &m.ManyToMany{Table: %q}", f.ManyToMany.Table))
	}

	return fmt.Sprintf("m.Field{%s}", strings.Join(parts, ", "))
}

// generateIndexLiteral converts a yaml.Index to a m.Index{...} Go literal string.
func generateIndexLiteral(idx yaml.Index) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Name: %q", idx.Name))

	// Fields slice
	fieldStrs := make([]string, len(idx.Fields))
	for i, f := range idx.Fields {
		fieldStrs[i] = fmt.Sprintf("%q", f)
	}
	parts = append(parts, fmt.Sprintf("Fields: []string{%s}", strings.Join(fieldStrs, ", ")))

	if idx.Unique {
		parts = append(parts, "Unique: true")
	}

	return fmt.Sprintf("m.Index{%s}", strings.Join(parts, ", "))
}

// GenerateMainGo returns the source for a migrations/main.go file that serves as
// the entry point for running migrations. It references m.NewApp and m.Config which
// are provided by the migrate package (implemented in Task 6).
func (g *GoGenerator) GenerateMainGo() string {
	return `package main

import (
	"fmt"
	"os"

	m "github.com/ocomsoft/makemigrations/migrate"
)

func main() {
	app := m.NewApp(m.Config{
		Registry: m.GlobalRegistry(),
	})
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
`
}

// GenerateGoMod returns a go.mod file content string for the generated migrations
// module. moduleName is the module path (e.g. "myproject/migrations") and version
// is the makemigrations version to require (e.g. "v0.3.0").
func (g *GoGenerator) GenerateGoMod(moduleName, version string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("module %s\n\n", moduleName))
	b.WriteString("go 1.24\n\n")
	b.WriteString("require (\n")
	b.WriteString(fmt.Sprintf("\tgithub.com/ocomsoft/makemigrations %s\n", version))
	b.WriteString(")\n")
	return b.String()
}
