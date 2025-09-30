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
package turso

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/utils"
)

// Provider implements the Provider interface for Turso
// Turso is a distributed SQLite-compatible database for edge computing
type Provider struct{}

// New creates a new Turso provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for Turso (same as SQLite)
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

// SupportsOperation checks if Turso supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "DROP_COLUMN", "RENAME_TABLE", "RENAME_COLUMN":
		return true
	case "ALTER_COLUMN":
		return false // Limited ALTER COLUMN support like SQLite
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to Turso-specific SQL type (same as SQLite)
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		return "TEXT"
	case "text":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "bigint":
		return "INTEGER"
	case "serial":
		return "INTEGER PRIMARY KEY AUTOINCREMENT"
	case "float":
		return "REAL"
	case "decimal":
		return "REAL"
	case "boolean":
		return "INTEGER" // SQLite uses INTEGER for boolean
	case "date":
		return "TEXT"
	case "time":
		return "TEXT"
	case "timestamp":
		return "TEXT"
	case "uuid":
		return "TEXT"
	case "jsonb":
		return "TEXT" // SQLite stores JSON as TEXT
	default:
		return "TEXT"
	}
}

// GetDefaultValue converts default value references to Turso-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for Turso
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	indexType := ""
	if index.Unique {
		indexType = "UNIQUE "
	}

	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);",
		indexType,
		p.QuoteName(index.Name),
		p.QuoteName(tableName),
		strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for Turso
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("DROP INDEX %s;", p.QuoteName(indexName))
}

// GenerateDropTable generates DROP TABLE statement
func (p *Provider) GenerateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", p.QuoteName(tableName))
}

// GenerateAddColumn generates ALTER TABLE ADD COLUMN statement
func (p *Provider) GenerateAddColumn(tableName string, field *types.Field) string {
	fieldDef := fmt.Sprintf("%s %s", p.QuoteName(field.Name), p.ConvertFieldType(field))
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", p.QuoteName(tableName), fieldDef)
}

// GenerateDropColumn generates DROP COLUMN statement (newer SQLite/Turso feature)
func (p *Provider) GenerateDropColumn(tableName, columnName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", p.QuoteName(tableName), p.QuoteName(columnName))
}

// GenerateRenameTable generates ALTER TABLE RENAME statement
func (p *Provider) GenerateRenameTable(oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateRenameColumn generates RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
		p.QuoteName(tableName), p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateCreateTable generates CREATE TABLE statement for Turso
func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string

	for _, field := range table.Fields {
		fieldDef, err := p.convertField(schema, &field)
		if err != nil {
			return "", fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		if fieldDef != "" {
			fieldDefs = append(fieldDefs, fieldDef)
		}
	}

	var sql strings.Builder
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.QuoteName(table.Name)))

	for i, def := range fieldDefs {
		sql.WriteString("    " + def)
		if i < len(fieldDefs)-1 {
			sql.WriteString(",")
		}
		sql.WriteString("\n")
	}

	sql.WriteString(");")
	return sql.String(), nil
}

func (p *Provider) convertField(schema *types.Schema, field *types.Field) (string, error) {
	if field.Type == "many_to_many" {
		return "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	// Handle serial/autoincrement specially
	if field.Type == "serial" {
		def.WriteString("INTEGER PRIMARY KEY AUTOINCREMENT")
		return def.String(), nil
	}

	sqlType := p.ConvertFieldType(field)
	def.WriteString(sqlType)

	if field.PrimaryKey && field.Type != "serial" {
		def.WriteString(" PRIMARY KEY")
	}

	if !field.IsNullable() && !field.PrimaryKey {
		def.WriteString(" NOT NULL")
	}

	if field.Default != "" {
		defaultValue := utils.ConvertDefaultValue(schema, "turso", field.Default)
		def.WriteString(" DEFAULT " + defaultValue)
	}

	return def.String(), nil
}

// Remaining interface methods
func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	return ""
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return ""
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return "INTEGER"
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	var indexes []string

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "foreign_key" {
				indexName := fmt.Sprintf("idx_%s_%s", table.Name, field.Name)
				indexSQL := fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
					p.QuoteName(indexName),
					p.QuoteName(table.Name),
					p.QuoteName(field.Name))
				indexes = append(indexes, indexSQL)
			}
		}

		for _, index := range table.Indexes {
			indexSQL := p.GenerateCreateIndex(&index, table.Name)
			indexes = append(indexes, indexSQL)
		}
	}

	if len(indexes) == 0 {
		return ""
	}

	return strings.Join(indexes, "\n")
}

func (p *Provider) GenerateForeignKeyConstraints(schema *types.Schema, junctionTables []types.Table) string {
	return ""
}

// GetDatabaseSchema extracts schema information from a Turso database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("Turso schema extraction not implemented yet")
}
