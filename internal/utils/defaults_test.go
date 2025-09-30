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
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func TestConvertDefaultValue(t *testing.T) {
	// Test schema with defaults for all database types
	schema := &types.Schema{
		Defaults: types.Defaults{
			PostgreSQL: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "CURRENT_DATE",
				"new_uuid": "gen_random_uuid()",
				"true":     "true",
				"false":    "false",
				"zero":     "0",
				"blank":    "''",
				"null":     "NULL",
				"array":    "'[]'::jsonb",
				"object":   "'{}'::jsonb",
			},
			MySQL: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "(CURDATE())",
				"new_uuid": "(UUID())",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
				"array":    "('[]')",
				"object":   "('{}')",
			},
			SQLServer: map[string]string{
				"now":      "GETDATE()",
				"today":    "CAST(GETDATE() AS DATE)",
				"new_uuid": "NEWID()",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
				"array":    "'[]'",
				"object":   "'{}'",
			},
			SQLite: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "CURRENT_DATE",
				"new_uuid": "''",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
				"array":    "'[]'",
				"object":   "'{}'",
			},
		},
	}

	tests := []struct {
		name         string
		databaseType string
		input        string
		expected     string
		description  string
	}{
		// PostgreSQL tests
		{"PostgreSQL now", "postgresql", "now", "CURRENT_TIMESTAMP", "Should map to PostgreSQL current timestamp"},
		{"PostgreSQL new_uuid", "postgresql", "new_uuid", "gen_random_uuid()", "Should map to PostgreSQL UUID generation"},
		{"PostgreSQL zero", "postgresql", "zero", "0", "Should map to PostgreSQL zero"},
		{"PostgreSQL true", "postgresql", "true", "true", "Should map to PostgreSQL true"},
		{"PostgreSQL false", "postgresql", "false", "false", "Should map to PostgreSQL false"},
		{"PostgreSQL array", "postgresql", "array", "'[]'::jsonb", "Should map to PostgreSQL JSONB array"},
		{"PostgreSQL literal number", "postgresql", "42", "42", "Should return numeric literals as-is"},
		{"PostgreSQL literal string", "postgresql", "hello", "'hello'", "Should wrap string literals in quotes"},

		// MySQL tests
		{"MySQL now", "mysql", "now", "CURRENT_TIMESTAMP", "Should map to MySQL current timestamp"},
		{"MySQL today", "mysql", "today", "(CURDATE())", "Should map to MySQL current date"},
		{"MySQL new_uuid", "mysql", "new_uuid", "(UUID())", "Should map to MySQL UUID generation"},
		{"MySQL true", "mysql", "true", "1", "Should map to MySQL boolean true (1)"},
		{"MySQL false", "mysql", "false", "0", "Should map to MySQL boolean false (0)"},
		{"MySQL zero", "mysql", "zero", "0", "Should map to MySQL zero"},

		// SQL Server tests
		{"SQL Server now", "sqlserver", "now", "GETDATE()", "Should map to SQL Server current timestamp"},
		{"SQL Server today", "sqlserver", "today", "CAST(GETDATE() AS DATE)", "Should map to SQL Server current date"},
		{"SQL Server new_uuid", "sqlserver", "new_uuid", "NEWID()", "Should map to SQL Server UUID generation"},
		{"SQL Server true", "sqlserver", "true", "1", "Should map to SQL Server boolean true (1)"},
		{"SQL Server false", "sqlserver", "false", "0", "Should map to SQL Server boolean false (0)"},

		// SQLite tests
		{"SQLite now", "sqlite", "now", "CURRENT_TIMESTAMP", "Should map to SQLite current timestamp"},
		{"SQLite true", "sqlite", "true", "1", "Should map to SQLite boolean true (1)"},
		{"SQLite false", "sqlite", "false", "0", "Should map to SQLite boolean false (0)"},

		// YDB tests (uses PostgreSQL defaults as fallback)
		{"YDB now", "ydb", "now", "CURRENT_TIMESTAMP", "Should map to YDB current timestamp"},
		{"YDB new_uuid", "ydb", "new_uuid", "gen_random_uuid()", "Should map to YDB UUID generation"},
		{"YDB true", "ydb", "true", "true", "Should map to YDB boolean true"},
		{"YDB false", "ydb", "false", "false", "Should map to YDB boolean false"},

		// Unknown database type tests
		{"Unknown DB numeric", "unknown", "42", "42", "Should return numeric literals as-is for unknown DB"},
		{"Unknown DB boolean", "unknown", "true", "true", "Should return boolean as-is for unknown DB"},
		{"Unknown DB string", "unknown", "hello", "'hello'", "Should wrap strings in quotes for unknown DB"},

		// Unmapped values tests
		{"PostgreSQL unmapped", "postgresql", "custom_value", "'custom_value'", "Should wrap unmapped values in quotes"},
		{"MySQL unmapped", "mysql", "custom_value", "'custom_value'", "Should wrap unmapped values in quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertDefaultValue(schema, tt.databaseType, tt.input)
			if result != tt.expected {
				t.Errorf("ConvertDefaultValue(%s, %s, %s) = %s; want %s\nDescription: %s",
					tt.databaseType, tt.input, tt.input, result, tt.expected, tt.description)
			}
		})
	}
}

func TestConvertDefaultValueWithNilSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Nil schema numeric", "42", "42"},
		{"Nil schema boolean true", "true", "true"},
		{"Nil schema boolean false", "false", "false"},
		{"Nil schema NULL", "null", "NULL"},
		{"Nil schema NULL uppercase", "NULL", "NULL"},
		{"Nil schema string", "hello", "'hello'"},
		{"Nil schema decimal", "3.14", "3.14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertDefaultValue(nil, "postgresql", tt.input)
			if result != tt.expected {
				t.Errorf("ConvertDefaultValue(nil, postgresql, %s) = %s; want %s",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertDefaultValueWithEmptyDefaults(t *testing.T) {
	emptySchema := &types.Schema{
		Defaults: types.Defaults{
			PostgreSQL: map[string]string{},
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty defaults numeric", "42", "42"},
		{"Empty defaults boolean", "true", "true"},
		{"Empty defaults string", "hello", "'hello'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertDefaultValue(emptySchema, "postgresql", tt.input)
			if result != tt.expected {
				t.Errorf("ConvertDefaultValue(emptySchema, postgresql, %s) = %s; want %s",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestHandleFallbackDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Integer", "42", "42"},
		{"Float", "3.14", "3.14"},
		{"Negative number", "-10", "-10"},
		{"Boolean true", "true", "true"},
		{"Boolean false", "false", "false"},
		{"NULL lowercase", "null", "NULL"},
		{"NULL uppercase", "NULL", "NULL"},
		{"String", "hello", "'hello'"},
		{"String with spaces", "hello world", "'hello world'"},
		{"Empty string", "", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HandleFallbackDefault(tt.input)
			if result != tt.expected {
				t.Errorf("HandleFallbackDefault(%s) = %s; want %s",
					tt.input, result, tt.expected)
			}
		})
	}
}
