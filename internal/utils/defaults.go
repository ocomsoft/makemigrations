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
	"strings"

	"github.com/ocomsoft/morphic/internal/types"
)

// ConvertDefaultValue converts a YAML default value to database-specific SQL
// This is a common utility function that all providers can use
func ConvertDefaultValue(schema *types.Schema, databaseType string, defaultValue string) string {
	if schema == nil {
		return HandleFallbackDefault(defaultValue)
	}

	// Get the appropriate defaults mapping for the database type
	dbType := types.DatabaseType(databaseType)
	defaults := schema.Defaults.ForProvider(dbType)
	if defaults == nil && databaseType == "ydb" {
		// YDB (Yandex Database) - use PostgreSQL defaults as fallback since YDB is SQL-based
		// but has limited default value support
		defaults = schema.Defaults.ForProvider(types.DatabasePostgreSQL)
	}
	if defaults == nil {
		return HandleFallbackDefault(defaultValue)
	}

	// Look up the default value in the mapping
	if sqlDefault, exists := defaults[defaultValue]; exists {
		return sqlDefault
	}

	return HandleFallbackDefault(defaultValue)
}

// IsSQLExpression returns true when the value is already a valid SQL expression
// that should be emitted verbatim rather than wrapped in single quotes.
// This covers:
//   - Single-quoted string literals already in SQL form: 'value', '[]'::jsonb
//   - SQL function calls: gen_random_uuid(), uuid_generate_v4()
//   - Type cast expressions: '{}'::jsonb
//   - SQL keywords (all-uppercase, letters and underscores): CURRENT_TIMESTAMP
//   - NULL/null, TRUE/true, FALSE/false literals
func IsSQLExpression(value string) bool {
	if value == "" {
		return false
	}
	// Already-quoted SQL string literal
	if strings.HasPrefix(value, "'") {
		return true
	}
	// SQL function call or expression containing parentheses
	if strings.Contains(value, "(") {
		return true
	}
	// Type cast expression (e.g. '[]'::jsonb)
	if strings.Contains(value, "::") {
		return true
	}
	// SQL null / boolean literals
	lower := strings.ToLower(value)
	if lower == "null" || lower == "true" || lower == "false" {
		return true
	}
	// SQL keyword: all uppercase ASCII letters and underscores (e.g. CURRENT_TIMESTAMP)
	if len(value) > 1 {
		allUpper := true
		for _, ch := range value {
			if (ch < 'A' || ch > 'Z') && ch != '_' {
				allUpper = false
				break
			}
		}
		if allUpper {
			return true
		}
	}
	return false
}

// FormatDefaultValue formats a default value for use in SQL DDL.
// SQL expressions, numeric values, and boolean/null literals are returned verbatim.
// All other values are wrapped in single quotes as string literals.
func FormatDefaultValue(value string) string {
	if IsSQLExpression(value) {
		return value
	}
	// Numeric value
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return value
	}
	// Otherwise treat as a plain string literal
	return fmt.Sprintf("'%s'", value)
}

// HandleFallbackDefault handles cases where no schema mapping exists.
// It normalises NULL to uppercase and delegates to FormatDefaultValue for
// expression detection and proper formatting.
func HandleFallbackDefault(defaultValue string) string {
	// Normalise null to uppercase SQL keyword
	if strings.ToLower(defaultValue) == "null" {
		return "NULL"
	}
	return FormatDefaultValue(defaultValue)
}

// SafeConstraintName returns a PostgreSQL-safe constraint name (max 63 chars).
// If the full name exceeds 63 characters, it is truncated to 54 chars and an
// 8-hex-char FNV-32a hash of the original name is appended (54+1+8=63).
func SafeConstraintName(name string) string {
	const maxLen = 63
	if len(name) <= maxLen {
		return name
	}
	var h uint32
	for _, b := range []byte(name) {
		h ^= uint32(b)
		h *= 16777619
	}
	return fmt.Sprintf("%s_%08x", name[:54], h)
}
