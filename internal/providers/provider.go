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
package providers

import (
	"github.com/ocomsoft/makemigrations/internal/types"
)

// Provider defines the interface for database-specific SQL generation
type Provider interface {
	// DDL Generation
	GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error)
	GenerateDropTable(tableName string) string
	GenerateAddColumn(tableName string, field *types.Field) string
	GenerateDropColumn(tableName, columnName string) string
	GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error)
	GenerateRenameTable(oldName, newName string) string
	GenerateRenameColumn(tableName, oldName, newName string) string

	// Index Operations
	GenerateCreateIndex(index *types.Index, tableName string) string
	GenerateDropIndex(indexName, tableName string) string

	// Foreign Key Operations
	GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string
	GenerateDropForeignKeyConstraint(tableName, constraintName string) string
	GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error)
	InferForeignKeyType(referencedTable string, schema *types.Schema) string

	// Type Conversion
	ConvertFieldType(field *types.Field) string
	GetDefaultValue(defaultRef string, defaults map[string]string) (string, error)

	// Utilities
	QuoteName(name string) string
	SupportsOperation(operation string) bool

	// Schema processing
	GenerateIndexes(schema *types.Schema) string
	GenerateForeignKeyConstraints(schema *types.Schema, junctionTables []types.Table) string

	// Database reverse engineering
	GetDatabaseSchema(connectionString string) (*types.Schema, error)
}
