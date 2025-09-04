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
package struct2schema

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// TypeMapping represents a mapping from Go type to SQL type
type TypeMapping struct {
	SQLType   string
	Length    int
	Precision int
	Scale     int
	Nullable  *bool
}

// TypeMapper handles mapping Go types to SQL types
type TypeMapper struct {
	config         *Config
	targetDB       string
	customMappings map[string]TypeMapping
	defaults       map[string]string
}

// NewTypeMapper creates a new type mapper
func NewTypeMapper(configFile, targetDB string) (*TypeMapper, error) {
	mapper := &TypeMapper{
		targetDB:       targetDB,
		customMappings: make(map[string]TypeMapping),
		defaults:       make(map[string]string),
	}

	// Load default mappings
	mapper.loadDefaultMappings()

	// Load configuration if provided
	if configFile != "" {
		config, err := LoadConfig(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		mapper.config = config
		mapper.loadCustomMappings(config)
	}

	return mapper, nil
}

// MapType maps a Go type to a SQL type
func (tm *TypeMapper) MapType(goType string, isPointer, isSlice bool, tagInfo TagInfo) (string, int, int, int, *bool) {
	// Handle ignore flag
	if tagInfo.Ignore {
		return "", 0, 0, 0, nil
	}

	// Use type from tag if specified
	if tagInfo.Type != "" {
		return tm.processTypeMapping(tagInfo.Type, tagInfo.Length, tagInfo.Precision, tagInfo.Scale, isPointer, tagInfo.Nullable)
	}

	// Handle slice types (many-to-many relationships are handled separately by generating junction tables)
	if isSlice {
		return "", 0, 0, 0, nil // Skip slice fields - they will be handled at the relationship level
	}

	// Check for custom mappings first
	if mapping, exists := tm.customMappings[goType]; exists {
		nullable := mapping.Nullable
		if isPointer && nullable == nil {
			ptrNullable := true
			nullable = &ptrNullable
		}
		return mapping.SQLType, mapping.Length, mapping.Precision, mapping.Scale, nullable
	}

	// Map standard Go types
	sqlType, length, precision, scale := tm.mapStandardType(goType)

	// Determine nullability
	var nullable *bool
	if tagInfo.Nullable != nil {
		nullable = tagInfo.Nullable
	} else if isPointer || tm.isNullableType(goType) {
		ptrNullable := true
		nullable = &ptrNullable
	}

	// Override with tag values if provided
	if tagInfo.Length > 0 {
		length = tagInfo.Length
	}
	if tagInfo.Precision > 0 {
		precision = tagInfo.Precision
	}
	if tagInfo.Scale > 0 {
		scale = tagInfo.Scale
	}

	return sqlType, length, precision, scale, nullable
}

// processTypeMapping processes a type mapping with tag overrides
func (tm *TypeMapper) processTypeMapping(sqlType string, length, precision, scale int, isPointer bool, nullable *bool) (string, int, int, int, *bool) {
	// Use tag nullable if specified, otherwise use pointer info
	if nullable == nil && isPointer {
		ptrNullable := true
		nullable = &ptrNullable
	}

	return sqlType, length, precision, scale, nullable
}

// mapStandardType maps standard Go types to SQL types
func (tm *TypeMapper) mapStandardType(goType string) (string, int, int, int) {
	// Clean the type (remove package prefixes for standard types)
	cleanType := tm.cleanGoType(goType)

	switch cleanType {
	// String types
	case "string":
		return "varchar", 255, 0, 0

	// Integer types
	case "int", "int32":
		return "integer", 0, 0, 0
	case "int8":
		return "integer", 0, 0, 0 // Most DBs don't have int8, use integer
	case "int16":
		return "integer", 0, 0, 0
	case "int64":
		return "bigint", 0, 0, 0
	case "uint", "uint32":
		return "integer", 0, 0, 0
	case "uint8":
		return "integer", 0, 0, 0
	case "uint16":
		return "integer", 0, 0, 0
	case "uint64":
		return "bigint", 0, 0, 0

	// Float types
	case "float32", "float64":
		return "float", 0, 0, 0

	// Boolean type
	case "bool":
		return "boolean", 0, 0, 0

	// Time types
	case "time.Time", "Time":
		return "timestamp", 0, 0, 0

	// UUID types (common in Go applications)
	case "uuid.UUID", "UUID":
		return "uuid", 0, 0, 0

	// Decimal types (for financial applications)
	case "decimal.Decimal", "Decimal":
		return "decimal", 0, 19, 2

	// JSON types
	case "interface{}":
		if tm.targetDB == "postgresql" {
			return "jsonb", 0, 0, 0
		}
		return "text", 0, 0, 0

	// Byte slice (for binary data)
	case "[]byte":
		return "text", 0, 0, 0

	// SQL null types
	case "sql.NullString", "NullString":
		return "varchar", 255, 0, 0
	case "sql.NullInt64", "NullInt64":
		return "bigint", 0, 0, 0
	case "sql.NullInt32", "NullInt32":
		return "integer", 0, 0, 0
	case "sql.NullFloat64", "NullFloat64":
		return "float", 0, 0, 0
	case "sql.NullBool", "NullBool":
		return "boolean", 0, 0, 0
	case "sql.NullTime", "NullTime":
		return "timestamp", 0, 0, 0

	// Default for unknown types (treat as foreign key reference)
	default:
		// If it looks like a struct type name (starts with uppercase), treat as foreign key
		if len(cleanType) > 0 && cleanType[0] >= 'A' && cleanType[0] <= 'Z' {
			return "foreign_key", 0, 0, 0
		}
		// Otherwise, default to text
		return "text", 0, 0, 0
	}
}

// cleanGoType removes package prefixes and pointer/slice indicators
func (tm *TypeMapper) cleanGoType(goType string) string {
	// Remove pointer indicator
	cleanType := strings.TrimPrefix(goType, "*")

	// Remove slice indicator
	cleanType = strings.TrimPrefix(cleanType, "[]")

	// Keep package prefixes for certain well-known types
	wellKnownPackages := []string{"time.", "uuid.", "sql.", "decimal."}
	for _, pkg := range wellKnownPackages {
		if strings.Contains(cleanType, pkg) {
			return cleanType
		}
	}

	// Remove package prefix for other types
	if idx := strings.LastIndex(cleanType, "."); idx != -1 {
		cleanType = cleanType[idx+1:]
	}

	return cleanType
}

// isNullableType checks if a Go type is inherently nullable
func (tm *TypeMapper) isNullableType(goType string) bool {
	nullableTypes := []string{
		"sql.NullString",
		"sql.NullInt64",
		"sql.NullInt32",
		"sql.NullFloat64",
		"sql.NullBool",
		"sql.NullTime",
		"NullString",
		"NullInt64",
		"NullInt32",
		"NullFloat64",
		"NullBool",
		"NullTime",
	}

	cleanType := tm.cleanGoType(goType)
	for _, nullType := range nullableTypes {
		if cleanType == nullType || goType == nullType {
			return true
		}
	}

	return false
}

// loadDefaultMappings loads default type mappings
func (tm *TypeMapper) loadDefaultMappings() {
	// Load database-specific defaults
	switch tm.targetDB {
	case "postgresql":
		tm.defaults = map[string]string{
			"blank":        "''",
			"array":        "'[]'::jsonb",
			"object":       "'{}'::jsonb",
			"zero":         "0",
			"current_time": "CURRENT_TIME",
			"new_uuid":     "gen_random_uuid()",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "CURRENT_DATE",
			"false":        "false",
			"null":         "null",
			"true":         "true",
		}
	case "mysql":
		tm.defaults = map[string]string{
			"blank":        "''",
			"array":        "('[]')",
			"object":       "('{}')",
			"zero":         "0",
			"current_time": "(CURTIME())",
			"new_uuid":     "(UUID())",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "(CURDATE())",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		}
	case "sqlite":
		tm.defaults = map[string]string{
			"blank":        "''",
			"array":        "'[]'",
			"object":       "'{}'",
			"zero":         "0",
			"current_time": "CURRENT_TIME",
			"new_uuid":     "",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "CURRENT_DATE",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		}
	case "sqlserver":
		tm.defaults = map[string]string{
			"blank":        "''",
			"array":        "'[]'",
			"object":       "'{}'",
			"zero":         "0",
			"current_time": "CAST(GETDATE() AS TIME)",
			"new_uuid":     "NEWID()",
			"now":          "GETDATE()",
			"today":        "CAST(GETDATE() AS DATE)",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		}
	}
}

// loadCustomMappings loads custom type mappings from config
func (tm *TypeMapper) loadCustomMappings(config *Config) {
	for goType, sqlType := range config.TypeMappings {
		tm.customMappings[goType] = TypeMapping{
			SQLType: sqlType,
		}
	}

	// Override defaults with custom defaults
	if config.CustomDefaults != nil {
		if dbDefaults, exists := config.CustomDefaults[tm.targetDB]; exists {
			for key, value := range dbDefaults {
				tm.defaults[key] = value
			}
		}
	}
}

// GetDefaults returns the default values for the target database
func (tm *TypeMapper) GetDefaults() map[string]string {
	return tm.defaults
}

// CreateDefaultsForAllDBs creates default mappings for all supported databases
func (tm *TypeMapper) CreateDefaultsForAllDBs() types.Defaults {
	return types.Defaults{
		PostgreSQL: map[string]string{
			"blank":        "''",
			"array":        "'[]'::jsonb",
			"object":       "'{}'::jsonb",
			"zero":         "0",
			"current_time": "CURRENT_TIME",
			"new_uuid":     "gen_random_uuid()",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "CURRENT_DATE",
			"false":        "false",
			"null":         "null",
			"true":         "true",
		},
		MySQL: map[string]string{
			"blank":        "''",
			"array":        "('[]')",
			"object":       "('{}')",
			"zero":         "0",
			"current_time": "(CURTIME())",
			"new_uuid":     "(UUID())",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "(CURDATE())",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		},
		SQLServer: map[string]string{
			"blank":        "''",
			"array":        "'[]'",
			"object":       "'{}'",
			"zero":         "0",
			"current_time": "CAST(GETDATE() AS TIME)",
			"new_uuid":     "NEWID()",
			"now":          "GETDATE()",
			"today":        "CAST(GETDATE() AS DATE)",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		},
		SQLite: map[string]string{
			"blank":        "''",
			"array":        "'[]'",
			"object":       "'{}'",
			"zero":         "0",
			"current_time": "CURRENT_TIME",
			"new_uuid":     "",
			"now":          "CURRENT_TIMESTAMP",
			"today":        "CURRENT_DATE",
			"false":        "0",
			"null":         "null",
			"true":         "1",
		},
	}
}
