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
package auroradsql

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// Provider implements the Provider interface for Aurora DSQL
// Aurora DSQL is AWS's serverless, distributed SQL database
type Provider struct{}

// New creates a new Aurora DSQL provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for Aurora DSQL (double quotes like PostgreSQL)
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

// SupportsOperation checks if Aurora DSQL supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "RENAME_TABLE", "DROP_COLUMN", "ALTER_COLUMN", "RENAME_COLUMN":
		return true
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to Aurora DSQL-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "TEXT"
	case "text":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "serial":
		return "SERIAL" // Aurora DSQL supports SERIAL
	case "float":
		return "REAL"
	case "decimal":
		if field.Precision > 0 && field.Scale >= 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case "boolean":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case "timestamp":
		return "TIMESTAMP"
	case "uuid":
		return "UUID" // Aurora DSQL has native UUID support
	case "jsonb":
		return "JSONB" // Aurora DSQL supports JSONB
	default:
		return "TEXT"
	}
}

// GetDefaultValue converts default value references to Aurora DSQL-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for Aurora DSQL
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	indexType := "INDEX"
	if index.Unique {
		indexType = "UNIQUE INDEX"
	}

	return fmt.Sprintf("CREATE %s %s ON %s (%s);",
		indexType,
		p.QuoteName(index.Name),
		p.QuoteName(tableName),
		strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for Aurora DSQL
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

	if !field.IsNullable() {
		fieldDef += " NOT NULL"
	}

	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", p.QuoteName(tableName), fieldDef)
}

// GenerateDropColumn generates ALTER TABLE DROP COLUMN statement
func (p *Provider) GenerateDropColumn(tableName, columnName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", p.QuoteName(tableName), p.QuoteName(columnName))
}

// GenerateRenameTable generates ALTER TABLE RENAME statement
func (p *Provider) GenerateRenameTable(oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateRenameColumn generates ALTER TABLE RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
		p.QuoteName(tableName), p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateCreateTable generates CREATE TABLE statement for Aurora DSQL
func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string
	var constraints []string

	for _, field := range table.Fields {
		fieldDef, constraint, err := p.convertField(&field)
		if err != nil {
			return "", fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		if fieldDef != "" {
			fieldDefs = append(fieldDefs, fieldDef)
		}
		if constraint != "" {
			constraints = append(constraints, constraint)
		}
	}

	allDefs := append(fieldDefs, constraints...)

	var sql strings.Builder
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.QuoteName(table.Name)))

	for i, def := range allDefs {
		sql.WriteString("    " + def)
		if i < len(allDefs)-1 {
			sql.WriteString(",")
		}
		sql.WriteString("\n")
	}

	sql.WriteString(");")
	return sql.String(), nil
}

func (p *Provider) convertField(field *types.Field) (string, string, error) {
	if field.Type == "many_to_many" {
		return "", "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	sqlType := p.ConvertFieldType(field)
	def.WriteString(sqlType)

	if !field.IsNullable() || field.PrimaryKey {
		def.WriteString(" NOT NULL")
	}

	if field.AutoCreate && field.Type == "timestamp" {
		def.WriteString(" DEFAULT CURRENT_TIMESTAMP")
	} else if field.Default != "" {
		def.WriteString(" DEFAULT " + field.Default)
	}

	var constraint string
	if field.PrimaryKey {
		constraint = fmt.Sprintf("PRIMARY KEY (%s)", p.QuoteName(field.Name))
	}

	return def.String(), constraint, nil
}

// Remaining interface methods
func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	constraintName := fmt.Sprintf("fk_%s_%s", tableName, fieldName)
	onDeleteClause := ""
	if onDelete != "" {
		onDeleteClause = fmt.Sprintf(" ON DELETE %s", strings.ToUpper(onDelete))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s%s;",
		p.QuoteName(tableName), p.QuoteName(constraintName), p.QuoteName(fieldName), p.QuoteName(referencedTable), onDeleteClause)
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", p.QuoteName(tableName), p.QuoteName(constraintName))
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return "BIGINT"
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
	var constraints []string

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "foreign_key" && field.ForeignKey != nil {
				constraint := p.GenerateForeignKeyConstraint(table.Name, field.Name, field.ForeignKey.Table, field.ForeignKey.OnDelete)
				if constraint != "" {
					constraints = append(constraints, constraint)
				}
			}
		}
	}

	for _, junctionTable := range junctionTables {
		for _, field := range junctionTable.Fields {
			if field.Type == "foreign_key" && field.ForeignKey != nil {
				constraint := p.GenerateForeignKeyConstraint(junctionTable.Name, field.Name, field.ForeignKey.Table, field.ForeignKey.OnDelete)
				if constraint != "" {
					constraints = append(constraints, constraint)
				}
			}
		}
	}

	if len(constraints) == 0 {
		return ""
	}

	return strings.Join(constraints, "\n")
}

// GetDatabaseSchema extracts schema information from an Aurora DSQL database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("Aurora DSQL schema extraction not implemented yet")
}
