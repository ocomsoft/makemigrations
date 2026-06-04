package fkutils

import (
	"testing"

	"github.com/ocomsoft/morphic/internal/types"
)

// newResolver returns a ForeignKeyTypeResolver with typical PostgreSQL types for testing.
func newResolver() *ForeignKeyTypeResolver {
	return &ForeignKeyTypeResolver{
		UUIDType:    "UUID",
		IntegerType: "INTEGER",
		BigIntType:  "BIGINT",
		SerialType:  "INTEGER",
	}
}

// helper to build a minimal schema with the given tables.
func schemaWith(tables ...types.Table) *types.Schema {
	return &types.Schema{Tables: tables}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UserProfile", "user_profile"},
		{"user_profile", "user_profile"},
		{"ID", "i_d"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"simpleword", "simpleword"},
		{"A", "a"},
		{"", ""},
		{"ABCDef", "a_b_c_def"},
		{"alreadyLower", "already_lower"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CamelToSnake(tt.input)
			if got != tt.want {
				t.Errorf("CamelToSnake(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSnakeToCamel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_profile", "UserProfile"},
		{"UserProfile", "UserProfile"},
		{"id", "Id"},
		{"", ""},
		{"a", "A"},
		{"hello_world_test", "HelloWorldTest"},
		{"__leading", "Leading"},
		{"trailing_", "Trailing"},
		{"double__underscore", "DoubleUnderscore"},
		{"ALL_CAPS", "ALLCAPS"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SnakeToCamel(tt.input)
			if got != tt.want {
				t.Errorf("SnakeToCamel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetForeignKeyTypeFromPrimaryKey(t *testing.T) {
	r := newResolver()

	tests := []struct {
		name     string
		pkType   string
		wantType string
	}{
		{"serial PK", "serial", r.SerialType},
		{"uuid PK", "uuid", r.UUIDType},
		{"integer PK", "integer", r.IntegerType},
		{"bigint PK", "bigint", r.BigIntType},
		{"unknown type defaults to UUID", "varchar", r.UUIDType},
		{"empty type defaults to UUID", "", r.UUIDType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &types.Field{Name: "id", Type: tt.pkType}
			got := r.GetForeignKeyTypeFromPrimaryKey(field)
			if got != tt.wantType {
				t.Errorf("GetForeignKeyTypeFromPrimaryKey(type=%q) = %q, want %q", tt.pkType, got, tt.wantType)
			}
		})
	}
}

func TestGetForeignKeyType_ExplicitPrimaryKey(t *testing.T) {
	r := newResolver()

	schema := schemaWith(types.Table{
		Name: "users",
		Fields: []types.Field{
			{Name: "id", Type: "serial", PrimaryKey: true},
			{Name: "name", Type: "varchar"},
		},
	})

	got := r.GetForeignKeyType(schema, "users")
	if got != r.SerialType {
		t.Errorf("expected %q for serial PK, got %q", r.SerialType, got)
	}
}

func TestGetForeignKeyType_IDFieldFallback(t *testing.T) {
	r := newResolver()

	// Table with "id" field but PrimaryKey flag is not set
	schema := schemaWith(types.Table{
		Name: "orders",
		Fields: []types.Field{
			{Name: "id", Type: "bigint"},
			{Name: "total", Type: "decimal"},
		},
	})

	got := r.GetForeignKeyType(schema, "orders")
	if got != r.BigIntType {
		t.Errorf("expected %q for bigint id field, got %q", r.BigIntType, got)
	}
}

func TestGetForeignKeyType_NoPrimaryKeyOrID(t *testing.T) {
	r := newResolver()

	// Table with no PK and no "id" field
	schema := schemaWith(types.Table{
		Name: "settings",
		Fields: []types.Field{
			{Name: "key", Type: "varchar"},
			{Name: "value", Type: "text"},
		},
	})

	got := r.GetForeignKeyType(schema, "settings")
	if got != r.UUIDType {
		t.Errorf("expected default %q when no PK or id, got %q", r.UUIDType, got)
	}
}

func TestGetForeignKeyType_TableNotFound(t *testing.T) {
	r := newResolver()
	schema := schemaWith() // empty schema

	got := r.GetForeignKeyType(schema, "nonexistent")
	if got != r.UUIDType {
		t.Errorf("expected default %q for missing table, got %q", r.UUIDType, got)
	}
}

func TestGetForeignKeyType_NamespacedTable(t *testing.T) {
	r := newResolver()

	schema := schemaWith(types.Table{
		Name: "User",
		Fields: []types.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
		},
	})

	got := r.GetForeignKeyType(schema, "auth.User")
	if got != r.UUIDType {
		t.Errorf("expected %q for namespaced uuid PK, got %q", r.UUIDType, got)
	}
}

func TestGetForeignKeyType_DeepNamespace(t *testing.T) {
	r := newResolver()

	schema := schemaWith(types.Table{
		Name: "FileMetaData",
		Fields: []types.Field{
			{Name: "id", Type: "integer", PrimaryKey: true},
		},
	})

	got := r.GetForeignKeyType(schema, "filesystem.storage.FileMetaData")
	if got != r.IntegerType {
		t.Errorf("expected %q for deep-namespaced integer PK, got %q", r.IntegerType, got)
	}
}

func TestGetForeignKeyType_CamelCaseToSnakeMatch(t *testing.T) {
	r := newResolver()

	// Table stored as snake_case, referenced as CamelCase
	schema := schemaWith(types.Table{
		Name: "user_profile",
		Fields: []types.Field{
			{Name: "id", Type: "integer", PrimaryKey: true},
		},
	})

	got := r.GetForeignKeyType(schema, "UserProfile")
	if got != r.IntegerType {
		t.Errorf("expected %q for CamelCase->snake_case match, got %q", r.IntegerType, got)
	}
}

func TestGetForeignKeyType_MultipleTablesFirstMatch(t *testing.T) {
	r := newResolver()

	// Ensure first matching table is used when multiple tables exist
	schema := schemaWith(
		types.Table{
			Name: "accounts",
			Fields: []types.Field{
				{Name: "id", Type: "serial", PrimaryKey: true},
			},
		},
		types.Table{
			Name: "orders",
			Fields: []types.Field{
				{Name: "id", Type: "bigint", PrimaryKey: true},
			},
		},
	)

	got := r.GetForeignKeyType(schema, "orders")
	if got != r.BigIntType {
		t.Errorf("expected %q for orders table, got %q", r.BigIntType, got)
	}
}

func TestGetForeignKeyType_CaseInsensitiveMatch(t *testing.T) {
	r := newResolver()

	schema := schemaWith(types.Table{
		Name: "Users",
		Fields: []types.Field{
			{Name: "id", Type: "serial", PrimaryKey: true},
		},
	})

	got := r.GetForeignKeyType(schema, "users")
	if got != r.SerialType {
		t.Errorf("expected %q for case-insensitive match, got %q", r.SerialType, got)
	}
}

func TestGetForeignKeyType_ExplicitPKTakesPrecedenceOverIDField(t *testing.T) {
	r := newResolver()

	// Table where "pk_col" has PrimaryKey=true and there's also an "id" field
	schema := schemaWith(types.Table{
		Name: "mixed",
		Fields: []types.Field{
			{Name: "pk_col", Type: "bigint", PrimaryKey: true},
			{Name: "id", Type: "serial"},
		},
	})

	got := r.GetForeignKeyType(schema, "mixed")
	if got != r.BigIntType {
		t.Errorf("expected explicit PK type %q over id field, got %q", r.BigIntType, got)
	}
}

func TestGetForeignKeyType_IDFieldCaseInsensitive(t *testing.T) {
	r := newResolver()

	schema := schemaWith(types.Table{
		Name: "items",
		Fields: []types.Field{
			{Name: "ID", Type: "uuid"},
		},
	})

	got := r.GetForeignKeyType(schema, "items")
	if got != r.UUIDType {
		t.Errorf("expected %q for uppercase ID field, got %q", r.UUIDType, got)
	}
}
