package types

import (
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestSchema_TypeMappings_Unmarshal(t *testing.T) {
	input := `
database:
  name: test
type_mappings:
  postgresql:
    float: "NUMERIC(9,2)"
    boolean: "SMALLINT"
  mysql:
    float: "DOUBLE"
tables:
  - name: t
    fields:
      - name: id
        type: integer
`
	var s Schema
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := s.TypeMappings.PostgreSQL["float"]; got != "NUMERIC(9,2)" {
		t.Errorf("postgresql float = %q, want %q", got, "NUMERIC(9,2)")
	}
	if got := s.TypeMappings.PostgreSQL["boolean"]; got != "SMALLINT" {
		t.Errorf("postgresql boolean = %q, want %q", got, "SMALLINT")
	}
	if got := s.TypeMappings.MySQL["float"]; got != "DOUBLE" {
		t.Errorf("mysql float = %q, want %q", got, "DOUBLE")
	}
	if s.TypeMappings.SQLite != nil {
		t.Errorf("sqlite should be nil, got %v", s.TypeMappings.SQLite)
	}
}

func TestTypeMappings_ForProvider(t *testing.T) {
	tm := TypeMappings{
		PostgreSQL: map[string]string{"float": "NUMERIC"},
		MySQL:      map[string]string{"float": "DOUBLE"},
	}
	got := tm.ForProvider(DatabasePostgreSQL)
	if got["float"] != "NUMERIC" {
		t.Errorf("ForProvider(postgresql) = %v", got)
	}
	got = tm.ForProvider(DatabaseMySQL)
	if got["float"] != "DOUBLE" {
		t.Errorf("ForProvider(mysql) = %v", got)
	}
	got = tm.ForProvider(DatabaseType("unknown"))
	if got != nil {
		t.Errorf("ForProvider(unknown) should be nil, got %v", got)
	}
}
