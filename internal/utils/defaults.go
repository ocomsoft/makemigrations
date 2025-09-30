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
package utils

import (
	"fmt"
	"strconv"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// ConvertDefaultValue converts a YAML default value to database-specific SQL
// This is a common utility function that all providers can use
func ConvertDefaultValue(schema *types.Schema, databaseType string, defaultValue string) string {
	if schema == nil {
		return HandleFallbackDefault(defaultValue)
	}

	// Get the appropriate defaults mapping for the database type
	var defaults map[string]string
	switch databaseType {
	case "postgresql":
		defaults = schema.Defaults.PostgreSQL
	case "mysql":
		defaults = schema.Defaults.MySQL
	case "sqlserver":
		defaults = schema.Defaults.SQLServer
	case "sqlite":
		defaults = schema.Defaults.SQLite
	case "ydb":
		// YDB (Yandex Database) - use PostgreSQL defaults as fallback since YDB is SQL-based
		// but has limited default value support
		defaults = schema.Defaults.PostgreSQL
		if defaults == nil {
			return HandleFallbackDefault(defaultValue)
		}
	default:
		return HandleFallbackDefault(defaultValue)
	}

	// Look up the default value in the mapping
	if sqlDefault, exists := defaults[defaultValue]; exists {
		return sqlDefault
	}

	return HandleFallbackDefault(defaultValue)
}

// HandleFallbackDefault handles cases where no schema mapping exists
func HandleFallbackDefault(defaultValue string) string {
	// Check if it's a numeric value
	if _, err := strconv.ParseFloat(defaultValue, 64); err == nil {
		return defaultValue // Return numeric values as-is
	}

	// Check for boolean values
	if defaultValue == "true" || defaultValue == "false" {
		return defaultValue // Most databases accept true/false
	}

	// Check for NULL
	if defaultValue == "null" || defaultValue == "NULL" {
		return "NULL"
	}

	// Otherwise treat as string literal
	return fmt.Sprintf("'%s'", defaultValue)
}
