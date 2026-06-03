package struct2schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
	"gopkg.in/yaml.v3"
)

// TestNewTypeMapper verifies that NewTypeMapper initialises correctly
// with default mappings for various databases.
func TestNewTypeMapper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		targetDB string
		wantKey  string // a default key we expect to exist
		wantVal  string // its expected value
	}{
		{name: "postgresql defaults", targetDB: "postgresql", wantKey: "new_uuid", wantVal: "gen_random_uuid()"},
		{name: "mysql defaults", targetDB: "mysql", wantKey: "new_uuid", wantVal: "(UUID())"},
		{name: "sqlite defaults", targetDB: "sqlite", wantKey: "now", wantVal: "CURRENT_TIMESTAMP"},
		{name: "sqlserver defaults", targetDB: "sqlserver", wantKey: "now", wantVal: "GETDATE()"},
		{name: "unknown db has empty defaults", targetDB: "oracle", wantKey: "", wantVal: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tm, err := NewTypeMapper("", tt.targetDB)
			if err != nil {
				t.Fatalf("NewTypeMapper(%q) returned error: %v", tt.targetDB, err)
			}
			if tm == nil {
				t.Fatal("NewTypeMapper returned nil")
			}

			defaults := tm.GetDefaults()
			if tt.wantKey != "" {
				if got, ok := defaults[tt.wantKey]; !ok || got != tt.wantVal {
					t.Errorf("defaults[%q] = %q, want %q", tt.wantKey, got, tt.wantVal)
				}
			}
		})
	}
}

// TestNewTypeMapperWithConfig verifies config-based custom mappings.
func TestNewTypeMapperWithConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfg := Config{
		TypeMappings: map[string]string{
			"MyCustomType": "jsonb",
		},
		CustomDefaults: map[string]map[string]string{
			"postgresql": {
				"custom_key": "custom_value",
			},
		},
		TableNaming: TableNamingConfig{ConvertCase: "snake_case"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(cfgFile, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	tm, err := NewTypeMapper(cfgFile, "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper with config: %v", err)
	}

	// Custom mapping should be loaded
	sqlType, _, _, _, _ := tm.MapType("MyCustomType", false, false, TagInfo{})
	if sqlType != "jsonb" {
		t.Errorf("custom mapping: got %q, want %q", sqlType, "jsonb")
	}

	// Custom default should be merged
	defaults := tm.GetDefaults()
	if got := defaults["custom_key"]; got != "custom_value" {
		t.Errorf("custom default: got %q, want %q", got, "custom_value")
	}

	// Built-in defaults should still be present
	if got := defaults["now"]; got != "CURRENT_TIMESTAMP" {
		t.Errorf("built-in default: got %q, want %q", got, "CURRENT_TIMESTAMP")
	}
}

// TestNewTypeMapperInvalidConfig verifies error on missing config file.
func TestNewTypeMapperInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := NewTypeMapper("/nonexistent/config.yaml", "postgresql")
	if err == nil {
		t.Error("expected error for nonexistent config file, got nil")
	}
}

// TestMapTypeStandardTypes verifies mapping of common Go types to SQL types.
func TestMapTypeStandardTypes(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	tests := []struct {
		name          string
		goType        string
		isPointer     bool
		isSlice       bool
		tagInfo       TagInfo
		wantSQLType   string
		wantLength    int
		wantPrecision int
		wantScale     int
		wantNullable  *bool
	}{
		{
			name: "string maps to varchar 255", goType: "string",
			wantSQLType: "varchar", wantLength: 255,
		},
		{
			name: "int maps to integer", goType: "int",
			wantSQLType: "integer",
		},
		{
			name: "int64 maps to bigint", goType: "int64",
			wantSQLType: "bigint",
		},
		{
			name: "float64 maps to float", goType: "float64",
			wantSQLType: "float",
		},
		{
			name: "bool maps to boolean", goType: "bool",
			wantSQLType: "boolean",
		},
		{
			name: "time.Time maps to timestamp", goType: "time.Time",
			wantSQLType: "timestamp",
		},
		{
			name: "uuid.UUID maps to uuid", goType: "uuid.UUID",
			wantSQLType: "uuid",
		},
		{
			name: "decimal.Decimal maps to decimal", goType: "decimal.Decimal",
			wantSQLType: "decimal", wantPrecision: 19, wantScale: 2,
		},
		{
			name: "interface{} maps to jsonb on postgresql", goType: "interface{}",
			wantSQLType: "jsonb",
		},
		{
			name: "[]byte maps to text", goType: "[]byte",
			wantSQLType: "text",
		},
		{
			name: "sql.NullString maps to varchar", goType: "sql.NullString",
			wantSQLType: "varchar", wantLength: 255, wantNullable: boolPtr(true),
		},
		{
			name: "sql.NullInt64 maps to bigint", goType: "sql.NullInt64",
			wantSQLType: "bigint", wantNullable: boolPtr(true),
		},
		{
			name: "sql.NullBool maps to boolean", goType: "sql.NullBool",
			wantSQLType: "boolean", wantNullable: boolPtr(true),
		},
		{
			name: "uint64 maps to bigint", goType: "uint64",
			wantSQLType: "bigint",
		},
		{
			name: "int16 maps to integer", goType: "int16",
			wantSQLType: "integer",
		},
		{
			name: "int8 maps to integer", goType: "int8",
			wantSQLType: "integer",
		},
		{
			name: "uint maps to integer", goType: "uint",
			wantSQLType: "integer",
		},
		{
			name: "float32 maps to float", goType: "float32",
			wantSQLType: "float",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sqlType, length, precision, scale, nullable := tm.MapType(tt.goType, tt.isPointer, tt.isSlice, tt.tagInfo)
			if sqlType != tt.wantSQLType {
				t.Errorf("sqlType = %q, want %q", sqlType, tt.wantSQLType)
			}
			if tt.wantLength != 0 && length != tt.wantLength {
				t.Errorf("length = %d, want %d", length, tt.wantLength)
			}
			if tt.wantPrecision != 0 && precision != tt.wantPrecision {
				t.Errorf("precision = %d, want %d", precision, tt.wantPrecision)
			}
			if tt.wantScale != 0 && scale != tt.wantScale {
				t.Errorf("scale = %d, want %d", scale, tt.wantScale)
			}
			if tt.wantNullable != nil {
				if nullable == nil || *nullable != *tt.wantNullable {
					t.Errorf("nullable = %v, want %v", nullable, *tt.wantNullable)
				}
			}
		})
	}
}

// TestMapTypePointerNullability verifies that pointer types produce nullable fields.
func TestMapTypePointerNullability(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	_, _, _, _, nullable := tm.MapType("string", true, false, TagInfo{})
	if nullable == nil || !*nullable {
		t.Error("pointer type should be nullable")
	}
}

// TestMapTypeTagOverrides verifies that tag info overrides default type mapping.
func TestMapTypeTagOverrides(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	tagInfo := TagInfo{
		Type:      "text",
		Length:    500,
		Precision: 10,
		Scale:     5,
		Nullable:  boolPtr(false),
	}
	sqlType, length, precision, scale, nullable := tm.MapType("string", false, false, tagInfo)
	if sqlType != "text" {
		t.Errorf("sqlType = %q, want %q", sqlType, "text")
	}
	if length != 500 {
		t.Errorf("length = %d, want 500", length)
	}
	if precision != 10 {
		t.Errorf("precision = %d, want 10", precision)
	}
	if scale != 5 {
		t.Errorf("scale = %d, want 5", scale)
	}
	if nullable == nil || *nullable != false {
		t.Error("nullable should be false as set in tag")
	}
}

// TestMapTypeIgnored verifies that the Ignore flag results in empty output.
func TestMapTypeIgnored(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	sqlType, _, _, _, _ := tm.MapType("string", false, false, TagInfo{Ignore: true})
	if sqlType != "" {
		t.Errorf("ignored field should return empty sqlType, got %q", sqlType)
	}
}

// TestMapTypeSliceReturnsEmpty verifies that slice types return empty
// (handled at the relationship level).
func TestMapTypeSliceReturnsEmpty(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	sqlType, _, _, _, _ := tm.MapType("string", false, true, TagInfo{})
	if sqlType != "" {
		t.Errorf("slice type should return empty sqlType, got %q", sqlType)
	}
}

// TestMapTypeForeignKeyInference verifies that uppercase struct types
// are inferred as foreign keys.
func TestMapTypeForeignKeyInference(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	sqlType, _, _, _, _ := tm.MapType("Organization", false, false, TagInfo{})
	if sqlType != "foreign_key" {
		t.Errorf("uppercase struct type should map to foreign_key, got %q", sqlType)
	}
}

// TestMapTypeUnknownLowercase verifies that unknown lowercase types default to text.
func TestMapTypeUnknownLowercase(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	sqlType, _, _, _, _ := tm.MapType("somecustomtype", false, false, TagInfo{})
	if sqlType != "text" {
		t.Errorf("unknown lowercase type should map to text, got %q", sqlType)
	}
}

// TestMapTypeInterfaceMySQL verifies interface{} maps to text on non-postgresql.
func TestMapTypeInterfaceMySQL(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "mysql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	sqlType, _, _, _, _ := tm.MapType("interface{}", false, false, TagInfo{})
	if sqlType != "text" {
		t.Errorf("interface{} on mysql should map to text, got %q", sqlType)
	}
}

// TestCleanGoType verifies type cleaning logic.
func TestCleanGoType(t *testing.T) {
	t.Parallel()

	tm := &TypeMapper{}
	tests := []struct {
		input string
		want  string
	}{
		{"*string", "string"},
		{"[]byte", "byte"},
		{"time.Time", "time.Time"},
		{"uuid.UUID", "uuid.UUID"},
		{"mypackage.MyType", "MyType"},
		{"sql.NullString", "sql.NullString"},
		{"decimal.Decimal", "decimal.Decimal"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := tm.cleanGoType(tt.input)
			if got != tt.want {
				t.Errorf("cleanGoType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsNullableType verifies nullable type detection.
func TestIsNullableType(t *testing.T) {
	t.Parallel()

	tm := &TypeMapper{}

	nullableTypes := []string{
		"sql.NullString", "sql.NullInt64", "sql.NullInt32",
		"sql.NullFloat64", "sql.NullBool", "sql.NullTime",
		"NullString", "NullInt64",
	}
	for _, nt := range nullableTypes {
		if !tm.isNullableType(nt) {
			t.Errorf("isNullableType(%q) = false, want true", nt)
		}
	}

	nonNullableTypes := []string{"string", "int", "bool", "time.Time"}
	for _, nt := range nonNullableTypes {
		if tm.isNullableType(nt) {
			t.Errorf("isNullableType(%q) = true, want false", nt)
		}
	}
}

// TestCreateDefaultsForAllDBs verifies all DB defaults are populated.
func TestCreateDefaultsForAllDBs(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	defaults := tm.CreateDefaultsForAllDBs()

	if defaults.ForProvider(types.DatabasePostgreSQL) == nil {
		t.Error("PostgreSQL defaults should not be nil")
	}
	if defaults.ForProvider(types.DatabaseMySQL) == nil {
		t.Error("MySQL defaults should not be nil")
	}
	if defaults.ForProvider(types.DatabaseSQLServer) == nil {
		t.Error("SQLServer defaults should not be nil")
	}
	if defaults.ForProvider(types.DatabaseSQLite) == nil {
		t.Error("SQLite defaults should not be nil")
	}

	// Spot-check some values
	if got := defaults.ForProvider(types.DatabasePostgreSQL)["new_uuid"]; got != "gen_random_uuid()" {
		t.Errorf("PostgreSQL new_uuid = %q, want %q", got, "gen_random_uuid()")
	}
	if got := defaults.ForProvider(types.DatabaseMySQL)["false"]; got != "0" {
		t.Errorf("MySQL false = %q, want %q", got, "0")
	}
}

// TestMapTypeTagLengthOverridesDefault verifies that tag length overrides
// the default length from type mapping.
func TestMapTypeTagLengthOverridesDefault(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}

	tagInfo := TagInfo{Length: 100}
	_, length, _, _, _ := tm.MapType("string", false, false, tagInfo)
	if length != 100 {
		t.Errorf("tag length override: got %d, want 100", length)
	}
}

// boolPtr is a helper to create a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}
