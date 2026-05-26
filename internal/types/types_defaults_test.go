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
	if got := s.Defaults.PostgreSQL["uuid"]; got != "uuid_generate_v4()" {
		t.Errorf("postgresql uuid = %q, want %q", got, "uuid_generate_v4()")
	}
	if got := s.Defaults.PostgreSQL["timestamp"]; got != "now()" {
		t.Errorf("postgresql timestamp = %q, want %q", got, "now()")
	}
	if got := s.Defaults.MySQL["uuid"]; got != "UUID()" {
		t.Errorf("mysql uuid = %q, want %q", got, "UUID()")
	}
	if s.Defaults.SQLite != nil {
		t.Errorf("sqlite should be nil, got %v", s.Defaults.SQLite)
	}
}

func TestDefaults_ForProvider(t *testing.T) {
	d := Defaults{
		PostgreSQL: map[string]string{"uuid": "uuid_generate_v4()"},
		MySQL:      map[string]string{"uuid": "UUID()"},
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
	allProviders := []struct {
		dbType DatabaseType
		setter func(d *Defaults, m map[string]string)
	}{
		{DatabasePostgreSQL, func(d *Defaults, m map[string]string) { d.PostgreSQL = m }},
		{DatabaseMySQL, func(d *Defaults, m map[string]string) { d.MySQL = m }},
		{DatabaseSQLServer, func(d *Defaults, m map[string]string) { d.SQLServer = m }},
		{DatabaseSQLite, func(d *Defaults, m map[string]string) { d.SQLite = m }},
		{DatabaseRedshift, func(d *Defaults, m map[string]string) { d.Redshift = m }},
		{DatabaseClickHouse, func(d *Defaults, m map[string]string) { d.ClickHouse = m }},
		{DatabaseTiDB, func(d *Defaults, m map[string]string) { d.TiDB = m }},
		{DatabaseVertica, func(d *Defaults, m map[string]string) { d.Vertica = m }},
		{DatabaseYDB, func(d *Defaults, m map[string]string) { d.YDB = m }},
		{DatabaseTurso, func(d *Defaults, m map[string]string) { d.Turso = m }},
		{DatabaseStarRocks, func(d *Defaults, m map[string]string) { d.StarRocks = m }},
		{DatabaseAuroraDSQL, func(d *Defaults, m map[string]string) { d.AuroraDSQL = m }},
	}

	for _, tc := range allProviders {
		t.Run(string(tc.dbType), func(t *testing.T) {
			d := Defaults{}
			expected := map[string]string{"key": "value_" + string(tc.dbType)}
			tc.setter(&d, expected)

			got := d.ForProvider(tc.dbType)
			if got == nil {
				t.Fatalf("ForProvider(%s) returned nil", tc.dbType)
			}
			if got["key"] != expected["key"] {
				t.Errorf("ForProvider(%s)[key] = %q, want %q", tc.dbType, got["key"], expected["key"])
			}
		})
	}
}

func TestDefaults_SetForProvider(t *testing.T) {
	// Verify SetForProvider sets the correct field for all 12 providers
	allProviders := []struct {
		dbType DatabaseType
		getter func(d *Defaults) map[string]string
	}{
		{DatabasePostgreSQL, func(d *Defaults) map[string]string { return d.PostgreSQL }},
		{DatabaseMySQL, func(d *Defaults) map[string]string { return d.MySQL }},
		{DatabaseSQLServer, func(d *Defaults) map[string]string { return d.SQLServer }},
		{DatabaseSQLite, func(d *Defaults) map[string]string { return d.SQLite }},
		{DatabaseRedshift, func(d *Defaults) map[string]string { return d.Redshift }},
		{DatabaseClickHouse, func(d *Defaults) map[string]string { return d.ClickHouse }},
		{DatabaseTiDB, func(d *Defaults) map[string]string { return d.TiDB }},
		{DatabaseVertica, func(d *Defaults) map[string]string { return d.Vertica }},
		{DatabaseYDB, func(d *Defaults) map[string]string { return d.YDB }},
		{DatabaseTurso, func(d *Defaults) map[string]string { return d.Turso }},
		{DatabaseStarRocks, func(d *Defaults) map[string]string { return d.StarRocks }},
		{DatabaseAuroraDSQL, func(d *Defaults) map[string]string { return d.AuroraDSQL }},
	}

	for _, tc := range allProviders {
		t.Run(string(tc.dbType), func(t *testing.T) {
			d := Defaults{}
			expected := map[string]string{"key": "value_" + string(tc.dbType)}
			d.SetForProvider(tc.dbType, expected)

			got := tc.getter(&d)
			if got == nil {
				t.Fatalf("SetForProvider(%s) did not set the field", tc.dbType)
			}
			if got["key"] != expected["key"] {
				t.Errorf("SetForProvider(%s) field[key] = %q, want %q", tc.dbType, got["key"], expected["key"])
			}
		})
	}
}

func TestDefaults_SetForProvider_Unknown(t *testing.T) {
	// SetForProvider with unknown type should be a no-op
	d := Defaults{}
	d.SetForProvider(DatabaseType("unknown"), map[string]string{"key": "value"})

	// Verify nothing was set
	if d.PostgreSQL != nil || d.MySQL != nil || d.SQLServer != nil || d.SQLite != nil {
		t.Error("SetForProvider(unknown) should not set any field")
	}
}

func TestDefaults_ForProvider_SetForProvider_Roundtrip(t *testing.T) {
	// Verify that SetForProvider + ForProvider round-trips correctly
	d := Defaults{}
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
