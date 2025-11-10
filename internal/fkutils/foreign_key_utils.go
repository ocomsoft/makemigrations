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
package fkutils

import (
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// ForeignKeyTypeResolver helps resolve the appropriate SQL type for foreign key fields
// by looking up the referenced table's primary key type
type ForeignKeyTypeResolver struct {
	// UUIDType is the SQL type to use for UUID foreign keys (e.g., "UUID" for PostgreSQL, "CHAR(36)" for MySQL)
	UUIDType string
	// IntegerType is the SQL type to use for integer foreign keys (e.g., "INTEGER" for PostgreSQL, "INT" for MySQL)
	IntegerType string
	// BigIntType is the SQL type to use for bigint foreign keys
	BigIntType string
	// SerialType is the SQL type to use for serial/auto-increment foreign keys
	SerialType string
}

// GetForeignKeyType determines the appropriate SQL type for a foreign key field
// by looking up the referenced table's primary key type
func (r *ForeignKeyTypeResolver) GetForeignKeyType(schema *types.Schema, referencedTableName string) string {
	// If namespaced table reference (like "auth.User" or "filesystem.FileMetaData"),
	// extract the table name from the namespace
	if strings.Contains(referencedTableName, ".") {
		parts := strings.Split(referencedTableName, ".")
		if len(parts) > 1 {
			referencedTableName = parts[len(parts)-1] // Get the last part
		}
	}

	// Find the referenced table
	var referencedTable *types.Table
	for i := range schema.Tables {
		table := &schema.Tables[i]
		if strings.EqualFold(table.Name, referencedTableName) {
			referencedTable = table
			break
		}
		// Also try converting CamelCase to snake_case
		if strings.EqualFold(table.Name, CamelToSnake(referencedTableName)) {
			referencedTable = table
			break
		}
		// And try converting snake_case to CamelCase
		if strings.EqualFold(SnakeToCamel(table.Name), referencedTableName) {
			referencedTable = table
			break
		}
	}

	if referencedTable == nil {
		// Table not found, assume UUID primary key as default
		return r.UUIDType
	}

	// Look for explicit primary key field
	for i := range referencedTable.Fields {
		field := &referencedTable.Fields[i]
		if field.PrimaryKey {
			// Found explicit primary key, return appropriate foreign key type
			return r.GetForeignKeyTypeFromPrimaryKey(field)
		}
	}

	// Look for "id" field
	for i := range referencedTable.Fields {
		field := &referencedTable.Fields[i]
		if strings.EqualFold(field.Name, "id") {
			// Found id field, return appropriate foreign key type
			return r.GetForeignKeyTypeFromPrimaryKey(field)
		}
	}

	// No explicit primary key or id field found - assume UUID with default generator
	return r.UUIDType
}

// GetForeignKeyTypeFromPrimaryKey returns the appropriate foreign key type for a primary key field
func (r *ForeignKeyTypeResolver) GetForeignKeyTypeFromPrimaryKey(pkField *types.Field) string {
	switch pkField.Type {
	case "serial":
		return r.SerialType
	case "uuid":
		return r.UUIDType
	case "integer":
		return r.IntegerType
	case "bigint":
		return r.BigIntType
	default:
		// For other types, default to UUID
		return r.UUIDType
	}
}

// CamelToSnake converts CamelCase to snake_case
func CamelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// SnakeToCamel converts snake_case to CamelCase
func SnakeToCamel(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' {
			capitalizeNext = true
		} else {
			if capitalizeNext && 'a' <= r && r <= 'z' {
				result.WriteRune(r - 32) // Convert to uppercase
				capitalizeNext = false
			} else {
				result.WriteRune(r)
				capitalizeNext = false
			}
		}
	}
	return result.String()
}
