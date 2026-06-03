package types

import (
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestSchema_Defaults_Unmarshal(t *testing.T) {
	input := `
database:
  name: test
defaults:
  postgresql:
    uuid: "uuid_generate_v4()"
    timestamp: "now()"
  mysql:
    uuid: "UUID()"
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
	if got := s.Defaults[DatabasePostgreSQL]["uuid"]; got != "uuid_generate_v4()" {
		t.Errorf("postgresql uuid = %q, want %q", got, "uuid_generate_v4()")
	}
	if got := s.Defaults[DatabasePostgreSQL]["timestamp"]; got != "now()" {
		t.Errorf("postgresql timestamp = %q, want %q", got, "now()")
	}
	if got := s.Defaults[DatabaseMySQL]["uuid"]; got != "UUID()" {
		t.Errorf("mysql uuid = %q, want %q", got, "UUID()")
	}
	if s.Defaults[DatabaseSQLite] != nil {
		t.Errorf("sqlite should be nil, got %v", s.Defaults[DatabaseSQLite])
	}
}

func TestDefaults_ForProvider(t *testing.T) {
	d := Defaults{
		DatabasePostgreSQL: map[string]string{"uuid": "uuid_generate_v4()"},
		DatabaseMySQL:      map[string]string{"uuid": "UUID()"},
	}

	// Test known providers return correct maps
	got := d.ForProvider(DatabasePostgreSQL)
	if got["uuid"] != "uuid_generate_v4()" {
		t.Errorf("ForProvider(postgresql) = %v", got)
	}
	got = d.ForProvider(DatabaseMySQL)
	if got["uuid"] != "UUID()" {
		t.Errorf("ForProvider(mysql) = %v", got)
	}

	// Test unknown provider returns nil
	got = d.ForProvider(DatabaseType("unknown"))
	if got != nil {
		t.Errorf("ForProvider(unknown) should be nil, got %v", got)
	}
}

func TestDefaults_ForProvider_AllProviders(t *testing.T) {
	// Verify all 12 providers are handled by ForProvider
	allProviders := []DatabaseType{
		DatabasePostgreSQL, DatabaseMySQL, DatabaseSQLServer, DatabaseSQLite,
		DatabaseRedshift, DatabaseClickHouse, DatabaseTiDB, DatabaseVertica,
		DatabaseYDB, DatabaseTurso, DatabaseStarRocks, DatabaseAuroraDSQL,
	}

	for _, dbType := range allProviders {
		t.Run(string(dbType), func(t *testing.T) {
			d := make(Defaults)
			expected := map[string]string{"key": "value_" + string(dbType)}
			d[dbType] = expected

			got := d.ForProvider(dbType)
			if got == nil {
				t.Fatalf("ForProvider(%s) returned nil", dbType)
			}
			if got["key"] != expected["key"] {
				t.Errorf("ForProvider(%s)[key] = %q, want %q", dbType, got["key"], expected["key"])
			}
		})
	}
}

func TestDefaults_SetForProvider(t *testing.T) {
	// Verify SetForProvider sets the correct value for all 12 providers
	allProviders := []DatabaseType{
		DatabasePostgreSQL, DatabaseMySQL, DatabaseSQLServer, DatabaseSQLite,
		DatabaseRedshift, DatabaseClickHouse, DatabaseTiDB, DatabaseVertica,
		DatabaseYDB, DatabaseTurso, DatabaseStarRocks, DatabaseAuroraDSQL,
	}

	for _, dbType := range allProviders {
		t.Run(string(dbType), func(t *testing.T) {
			d := make(Defaults)
			expected := map[string]string{"key": "value_" + string(dbType)}
			d.SetForProvider(dbType, expected)

			got := d.ForProvider(dbType)
			if got == nil {
				t.Fatalf("SetForProvider(%s) did not set the field", dbType)
			}
			if got["key"] != expected["key"] {
				t.Errorf("SetForProvider(%s) field[key] = %q, want %q", dbType, got["key"], expected["key"])
			}
		})
	}
}

func TestDefaults_SetForProvider_Unknown(t *testing.T) {
	// SetForProvider with unknown type stores the value under the unknown key
	d := make(Defaults)
	d.SetForProvider(DatabaseType("unknown"), map[string]string{"key": "value"})

	// With map-based Defaults, unknown keys are stored but ForProvider for
	// known providers should return nil
	if d.ForProvider(DatabasePostgreSQL) != nil || d.ForProvider(DatabaseMySQL) != nil {
		t.Error("SetForProvider(unknown) should not affect known providers")
	}
}

func TestDefaults_ForProvider_SetForProvider_Roundtrip(t *testing.T) {
	// Verify that SetForProvider + ForProvider round-trips correctly
	d := make(Defaults)
	expected := map[string]string{"uuid": "gen_random_uuid()", "timestamp": "now()"}
	d.SetForProvider(DatabasePostgreSQL, expected)

	got := d.ForProvider(DatabasePostgreSQL)
	if len(got) != len(expected) {
		t.Fatalf("roundtrip length mismatch: got %d, want %d", len(got), len(expected))
	}
	for k, v := range expected {
		if got[k] != v {
			t.Errorf("roundtrip[%s] = %q, want %q", k, got[k], v)
		}
	}
}
