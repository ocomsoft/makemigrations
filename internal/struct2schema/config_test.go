package struct2schema

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig verifies loading a valid config file.
func TestLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `type_mappings:
  MyType: jsonb
  AnotherType: text
table_naming:
  convert_case: snake_case
  prefix: app_
  suffix: _tbl
custom_defaults:
  postgresql:
    my_default: my_value
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Check type mappings
	if got, ok := cfg.TypeMappings["MyType"]; !ok || got != "jsonb" {
		t.Errorf("TypeMappings[MyType] = %q, want %q", got, "jsonb")
	}
	if got, ok := cfg.TypeMappings["AnotherType"]; !ok || got != "text" {
		t.Errorf("TypeMappings[AnotherType] = %q, want %q", got, "text")
	}

	// Check table naming
	if cfg.TableNaming.ConvertCase != "snake_case" {
		t.Errorf("ConvertCase = %q, want %q", cfg.TableNaming.ConvertCase, "snake_case")
	}
	if cfg.TableNaming.Prefix != "app_" {
		t.Errorf("Prefix = %q, want %q", cfg.TableNaming.Prefix, "app_")
	}
	if cfg.TableNaming.Suffix != "_tbl" {
		t.Errorf("Suffix = %q, want %q", cfg.TableNaming.Suffix, "_tbl")
	}

	// Check custom defaults
	if got, ok := cfg.CustomDefaults["postgresql"]["my_default"]; !ok || got != "my_value" {
		t.Errorf("CustomDefaults[postgresql][my_default] = %q, want %q", got, "my_value")
	}
}

// TestLoadConfigDefaults verifies that defaults are applied for missing fields.
func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "minimal.yaml")

	// Minimal YAML with no convert_case or type_mappings
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.TableNaming.ConvertCase != "snake_case" {
		t.Errorf("ConvertCase default = %q, want %q", cfg.TableNaming.ConvertCase, "snake_case")
	}
	if cfg.TypeMappings == nil {
		t.Error("TypeMappings should be initialized to non-nil map")
	}
}

// TestLoadConfigError verifies error handling for non-existent and invalid files.
func TestLoadConfigError(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent file", func(t *testing.T) {
		t.Parallel()
		_, err := LoadConfig("/nonexistent/file.yaml")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})
}

// TestGetDefaultConfig verifies the default config has expected structure.
func TestGetDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := GetDefaultConfig()

	if cfg.TableNaming.ConvertCase != "snake_case" {
		t.Errorf("ConvertCase = %q, want %q", cfg.TableNaming.ConvertCase, "snake_case")
	}

	// Check all four DB defaults exist
	for _, db := range []string{"postgresql", "mysql", "sqlite", "sqlserver"} {
		if _, ok := cfg.CustomDefaults[db]; !ok {
			t.Errorf("missing default config for %s", db)
		}
	}

	// Spot-check postgresql
	if got := cfg.CustomDefaults["postgresql"]["new_uuid"]; got != "gen_random_uuid()" {
		t.Errorf("postgresql new_uuid = %q, want %q", got, "gen_random_uuid()")
	}
}

// TestSaveConfig verifies round-trip save and load.
func TestSaveConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "saved.yaml")

	original := GetDefaultConfig()
	original.AddTypeMapping("CustomType", "citext")

	if err := original.SaveConfig(cfgPath); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if got, ok := loaded.TypeMappings["CustomType"]; !ok || got != "citext" {
		t.Errorf("round-trip TypeMappings[CustomType] = %q, want %q", got, "citext")
	}
}

// TestAddTypeMapping verifies adding a type mapping.
func TestAddTypeMapping(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.AddTypeMapping("Go", "SQL")

	got, ok := cfg.GetTypeMapping("Go")
	if !ok || got != "SQL" {
		t.Errorf("GetTypeMapping(Go) = %q, %v; want %q, true", got, ok, "SQL")
	}
}

// TestGetTypeMappingMissing verifies lookup on nil/missing mappings.
func TestGetTypeMappingMissing(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	got, ok := cfg.GetTypeMapping("Missing")
	if ok || got != "" {
		t.Errorf("GetTypeMapping(Missing) = %q, %v; want \"\", false", got, ok)
	}
}

// TestAddCustomDefault verifies adding custom defaults.
func TestAddCustomDefault(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.AddCustomDefault("postgresql", "key1", "value1")

	got, ok := cfg.GetCustomDefault("postgresql", "key1")
	if !ok || got != "value1" {
		t.Errorf("GetCustomDefault = %q, %v; want %q, true", got, ok, "value1")
	}
}

// TestGetCustomDefaultMissing verifies lookup on missing/nil custom defaults.
func TestGetCustomDefaultMissing(t *testing.T) {
	t.Parallel()

	cfg := &Config{}

	// nil CustomDefaults
	got, ok := cfg.GetCustomDefault("postgresql", "key1")
	if ok || got != "" {
		t.Errorf("expected empty for nil defaults, got %q, %v", got, ok)
	}

	// existing db but missing key
	cfg.AddCustomDefault("postgresql", "key1", "value1")
	got, ok = cfg.GetCustomDefault("postgresql", "missing")
	if ok || got != "" {
		t.Errorf("expected empty for missing key, got %q, %v", got, ok)
	}

	// missing db
	got, ok = cfg.GetCustomDefault("oracle", "key1")
	if ok || got != "" {
		t.Errorf("expected empty for missing db, got %q, %v", got, ok)
	}
}
