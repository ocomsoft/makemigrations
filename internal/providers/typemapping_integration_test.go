package providers

import (
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func TestCustomTypeMapping_PostgreSQL_Float(t *testing.T) {
	mappings := map[string]string{"float": "NUMERIC(9,2)"}
	p, err := NewProvider(types.DatabasePostgreSQL, mappings)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	field := &types.Field{Name: "price", Type: "float"}
	got := p.ConvertFieldType(field)
	if got != "NUMERIC(9,2)" {
		t.Errorf("got %q, want %q", got, "NUMERIC(9,2)")
	}
}

func TestCustomTypeMapping_PostgreSQL_NoOverride(t *testing.T) {
	mappings := map[string]string{"float": "NUMERIC(9,2)"}
	p, err := NewProvider(types.DatabasePostgreSQL, mappings)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	field := &types.Field{Name: "count", Type: "integer"}
	got := p.ConvertFieldType(field)
	if got != "INTEGER" {
		t.Errorf("got %q, want %q", got, "INTEGER")
	}
}

func TestCustomTypeMapping_Template_Varchar(t *testing.T) {
	mappings := map[string]string{"varchar": "NVARCHAR({{.Length}})"}
	p, err := NewProvider(types.DatabasePostgreSQL, mappings)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	field := &types.Field{Name: "name", Type: "varchar", Length: 200}
	got := p.ConvertFieldType(field)
	if got != "NVARCHAR(200)" {
		t.Errorf("got %q, want %q", got, "NVARCHAR(200)")
	}
}

func TestCustomTypeMapping_Template_Decimal(t *testing.T) {
	mappings := map[string]string{"decimal": "NUMERIC({{.Precision}},{{.Scale}})"}
	p, err := NewProvider(types.DatabaseMySQL, mappings)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	field := &types.Field{Name: "amount", Type: "decimal", Precision: 19, Scale: 4}
	got := p.ConvertFieldType(field)
	if got != "NUMERIC(19,4)" {
		t.Errorf("got %q, want %q", got, "NUMERIC(19,4)")
	}
}

func TestCustomTypeMapping_NilMappings_UsesDefault(t *testing.T) {
	p, err := NewProvider(types.DatabasePostgreSQL, nil)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	field := &types.Field{Name: "price", Type: "float"}
	got := p.ConvertFieldType(field)
	if got != "REAL" {
		t.Errorf("got %q, want %q", got, "REAL")
	}
}

func TestCustomTypeMapping_CreateTable_UsesOverride(t *testing.T) {
	mappings := map[string]string{"float": "DOUBLE PRECISION"}
	p, err := NewProvider(types.DatabasePostgreSQL, mappings)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	schema := &types.Schema{
		Tables: []types.Table{{
			Name: "prices",
			Fields: []types.Field{
				{Name: "id", Type: "serial", PrimaryKey: true},
				{Name: "amount", Type: "float"},
			},
		}},
	}
	sql, err := p.GenerateCreateTable(schema, &schema.Tables[0])
	if err != nil {
		t.Fatalf("GenerateCreateTable: %v", err)
	}
	if !strings.Contains(sql, "DOUBLE PRECISION") {
		t.Errorf("expected DOUBLE PRECISION in SQL, got:\n%s", sql)
	}
	if strings.Contains(sql, "REAL") {
		t.Errorf("should not contain REAL when overridden, got:\n%s", sql)
	}
}

func TestCustomTypeMapping_AllProviders(t *testing.T) {
	dbTypes := []types.DatabaseType{
		types.DatabasePostgreSQL, types.DatabaseMySQL, types.DatabaseSQLite,
		types.DatabaseSQLServer, types.DatabaseRedshift, types.DatabaseClickHouse,
		types.DatabaseTiDB, types.DatabaseVertica, types.DatabaseYDB,
		types.DatabaseTurso, types.DatabaseStarRocks, types.DatabaseAuroraDSQL,
	}
	mappings := map[string]string{"boolean": "CUSTOM_BOOL"}
	for _, dbType := range dbTypes {
		t.Run(string(dbType), func(t *testing.T) {
			p, err := NewProvider(dbType, mappings)
			if err != nil {
				t.Fatalf("NewProvider(%s): %v", dbType, err)
			}
			field := &types.Field{Name: "flag", Type: "boolean"}
			got := p.ConvertFieldType(field)
			if got != "CUSTOM_BOOL" {
				t.Errorf("%s: got %q, want %q", dbType, got, "CUSTOM_BOOL")
			}
		})
	}
}
