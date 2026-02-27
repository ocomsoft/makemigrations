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

package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ocomsoft/makemigrations/migrate"
)

// --- RenderDAGASCII tests ---

func TestRenderDAGASCII_Linear(t *testing.T) {
	out := &migrate.DAGOutput{
		Roots:       []string{"0001_initial"},
		Leaves:      []string{"0002_add_phone"},
		HasBranches: false,
		Migrations: []migrate.MigrationSummary{
			{
				Name:         "0001_initial",
				Dependencies: []string{},
				Operations: []migrate.OperationSummary{
					{Type: "create_table", Table: "users", Description: "Create table users (2 fields)"},
				},
			},
			{
				Name:         "0002_add_phone",
				Dependencies: []string{"0001_initial"},
				Operations: []migrate.OperationSummary{
					{Type: "add_field", Table: "users", Description: "Add field users.phone varchar"},
				},
			},
		},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "0001_initial") {
		t.Error("expected 0001_initial in output")
	}
	if !strings.Contains(result, "0002_add_phone") {
		t.Error("expected 0002_add_phone in output")
	}
	if !strings.Contains(result, "No branches") {
		t.Error("expected 'No branches' message")
	}
	if !strings.Contains(result, "Migration Graph") {
		t.Error("expected 'Migration Graph' header")
	}
	if !strings.Contains(result, "Create table users (2 fields)") {
		t.Error("expected operation description in output")
	}
	if !strings.Contains(result, "Add field users.phone varchar") {
		t.Error("expected add_field operation description in output")
	}
}

func TestRenderDAGASCII_WithBranches(t *testing.T) {
	out := &migrate.DAGOutput{
		Roots:       []string{"0001_initial"},
		Leaves:      []string{"0002_feature_a", "0002_feature_b"},
		HasBranches: true,
		Migrations: []migrate.MigrationSummary{
			{Name: "0001_initial", Dependencies: []string{}},
			{Name: "0002_feature_a", Dependencies: []string{"0001_initial"}},
			{Name: "0002_feature_b", Dependencies: []string{"0001_initial"}},
		},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "Branches detected") {
		t.Error("expected 'Branches detected' warning")
	}
	if !strings.Contains(result, "0002_feature_a") {
		t.Error("expected 0002_feature_a in output")
	}
	if !strings.Contains(result, "0002_feature_b") {
		t.Error("expected 0002_feature_b in output")
	}
}

func TestRenderDAGASCII_Empty(t *testing.T) {
	result := migrate.RenderDAGASCII(nil)
	if !strings.Contains(result, "No migrations") {
		t.Error("expected 'No migrations' message for nil output")
	}
}

func TestRenderDAGASCII_EmptyMigrations(t *testing.T) {
	out := &migrate.DAGOutput{
		Migrations: []migrate.MigrationSummary{},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "No migrations") {
		t.Error("expected 'No migrations' message for empty migrations slice")
	}
}

func TestRenderDAGASCII_RootsAndLeaves(t *testing.T) {
	out := &migrate.DAGOutput{
		Roots:       []string{"0001_initial"},
		Leaves:      []string{"0003_add_slug"},
		HasBranches: false,
		Migrations: []migrate.MigrationSummary{
			{Name: "0001_initial", Dependencies: []string{}},
			{Name: "0002_add_phone", Dependencies: []string{"0001_initial"}},
			{Name: "0003_add_slug", Dependencies: []string{"0002_add_phone"}},
		},
	}
	result := migrate.RenderDAGASCII(out)
	if !strings.Contains(result, "Roots:  0001_initial") {
		t.Error("expected roots line")
	}
	if !strings.Contains(result, "Leaves: 0003_add_slug") {
		t.Error("expected leaves line")
	}
}

// --- NewApp tests ---

func TestNewApp_CreatesApp(t *testing.T) {
	app := migrate.NewApp(migrate.Config{
		DatabaseType: "sqlite",
	})
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestNewAppWithRegistry_CreatesApp(t *testing.T) {
	reg := migrate.NewRegistry()
	app := migrate.NewAppWithRegistry(migrate.Config{DatabaseType: "postgresql"}, reg)
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

// --- App.Run tests ---

func TestApp_Run_DAG_ASCII(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})

	app := migrate.NewAppWithRegistry(migrate.Config{}, reg)
	err := app.Run([]string{"dag", "--format", "ascii"})
	if err != nil {
		t.Fatalf("dag ascii command failed: %v", err)
	}
}

func TestApp_Run_DAG_JSON(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})

	app := migrate.NewAppWithRegistry(migrate.Config{}, reg)
	err := app.Run([]string{"dag", "--format", "json"})
	if err != nil {
		t.Fatalf("dag json command failed: %v", err)
	}
}

func TestApp_Run_DAG_EmptyRegistry(t *testing.T) {
	reg := migrate.NewRegistry()
	app := migrate.NewAppWithRegistry(migrate.Config{}, reg)
	// Empty registry should produce empty DAG output (no migrations),
	// but no error since an empty graph is valid
	err := app.Run([]string{"dag"})
	if err != nil {
		t.Fatalf("dag command with empty registry failed: %v", err)
	}
}

func TestApp_Run_Up_SQLite(t *testing.T) {
	// Use a temp file for SQLite so the App can open it via DSN
	dbPath := filepath.Join(t.TempDir(), "test_up.db")
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	})

	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	// Redirect stdout to suppress output during tests
	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	err := app.Run([]string{"up"})
	if err != nil {
		t.Fatalf("up command failed: %v", err)
	}
}

func TestApp_Run_Down_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_down.db")
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	})

	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	// Apply first, then roll back
	if err := app.Run([]string{"up"}); err != nil {
		t.Fatalf("up failed: %v", err)
	}
	if err := app.Run([]string{"down", "--steps", "1"}); err != nil {
		t.Fatalf("down failed: %v", err)
	}
}

func TestApp_Run_Status_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_status.db")
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
				},
			},
		},
	})

	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	err := app.Run([]string{"status"})
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}
}

func TestApp_Run_ShowSQL_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_showsql.db")
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name: "users",
				Fields: []migrate.Field{
					{Name: "id", Type: "integer", PrimaryKey: true},
				},
			},
		},
	})

	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	err := app.Run([]string{"showsql"})
	if err != nil {
		t.Fatalf("showsql command failed: %v", err)
	}
}

func TestApp_Run_Fake_SQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_fake.db")
	reg := migrate.NewRegistry()
	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	err := app.Run([]string{"fake", "0001_initial"})
	if err != nil {
		t.Fatalf("fake command failed: %v", err)
	}
}

func TestApp_Run_Fake_MissingArg(t *testing.T) {
	app := migrate.NewAppWithRegistry(migrate.Config{}, migrate.NewRegistry())
	err := app.Run([]string{"fake"})
	if err == nil {
		t.Fatal("expected error when fake called without migration name")
	}
}

func TestApp_Run_UnknownCommand(t *testing.T) {
	app := migrate.NewAppWithRegistry(migrate.Config{}, migrate.NewRegistry())
	err := app.Run([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// --- EnvOr tests ---

func TestEnvOr_Default(t *testing.T) {
	// Use a key that definitely won't be set
	val := migrate.EnvOr("__MAKEMIGRATIONS_TEST_KEY_XXXX__", "fallback")
	if val != "fallback" {
		t.Fatalf("expected 'fallback', got %q", val)
	}
}

func TestEnvOr_FromEnv(t *testing.T) {
	const key = "__MAKEMIGRATIONS_TEST_ENVVAL__"
	t.Setenv(key, "from_env")
	val := migrate.EnvOr(key, "fallback")
	if val != "from_env" {
		t.Fatalf("expected 'from_env', got %q", val)
	}
}

func TestEnvOr_EmptyEnvUsesDefault(t *testing.T) {
	const key = "__MAKEMIGRATIONS_TEST_EMPTY_ENV__"
	t.Setenv(key, "")
	// os.Setenv with empty string: Getenv returns "" which is "not set or empty"
	// so EnvOr should return the default
	val := migrate.EnvOr(key, "default_val")
	// After Setenv("", ""), Getenv returns "" which is treated as empty by EnvOr
	if val != "default_val" {
		t.Fatalf("expected 'default_val' for empty env, got %q", val)
	}
}

// --- Config tests ---

func TestConfig_FieldsAccessible(t *testing.T) {
	cfg := migrate.Config{
		DatabaseType: "postgresql",
		DatabaseURL:  "postgres://user:pass@localhost:5432/mydb",
		DBHost:       "localhost",
		DBPort:       "5432",
		DBUser:       "user",
		DBPassword:   "pass",
		DBName:       "mydb",
		DBSSLMode:    "disable",
	}
	if cfg.DatabaseType != "postgresql" {
		t.Fatalf("expected 'postgresql', got %q", cfg.DatabaseType)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/mydb" {
		t.Fatalf("expected full DSN, got %q", cfg.DatabaseURL)
	}
	if cfg.DBHost != "localhost" {
		t.Fatalf("expected 'localhost', got %q", cfg.DBHost)
	}
	if cfg.DBPort != "5432" {
		t.Fatalf("expected '5432', got %q", cfg.DBPort)
	}
	if cfg.DBUser != "user" {
		t.Fatalf("expected 'user', got %q", cfg.DBUser)
	}
	if cfg.DBPassword != "pass" {
		t.Fatalf("expected 'pass', got %q", cfg.DBPassword)
	}
	if cfg.DBName != "mydb" {
		t.Fatalf("expected 'mydb', got %q", cfg.DBName)
	}
	if cfg.DBSSLMode != "disable" {
		t.Fatalf("expected 'disable', got %q", cfg.DBSSLMode)
	}
}

// --- Integration: DAG command with multi-step chain ---

func TestApp_Run_DAG_MultiStep(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.CreateTable{
				Name:   "users",
				Fields: []migrate.Field{{Name: "id", Type: "uuid", PrimaryKey: true}},
			},
		},
	})
	reg.Register(&migrate.Migration{
		Name:         "0002_add_phone",
		Dependencies: []string{"0001_initial"},
		Operations: []migrate.Operation{
			&migrate.AddField{
				Table: "users",
				Field: migrate.Field{Name: "phone", Type: "varchar", Length: 20, Nullable: true},
			},
		},
	})
	reg.Register(&migrate.Migration{
		Name:         "0003_add_slug",
		Dependencies: []string{"0002_add_phone"},
		Operations: []migrate.Operation{
			&migrate.AddField{
				Table: "users",
				Field: migrate.Field{Name: "slug", Type: "varchar", Length: 255},
			},
		},
	})

	app := migrate.NewAppWithRegistry(migrate.Config{}, reg)

	// Redirect stdout to a temp file to suppress output noise during tests
	origStdout := os.Stdout
	tmpFile, err := os.CreateTemp(t.TempDir(), "dag_test_stdout_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			t.Logf("warning: failed to close temp file: %v", err)
		}
	}()
	os.Stdout = tmpFile
	defer func() { os.Stdout = origStdout }()

	err = app.Run([]string{"dag", "--format", "ascii"})
	if err != nil {
		t.Fatalf("dag ascii with multi-step chain failed: %v", err)
	}

	err = app.Run([]string{"dag", "--format", "json"})
	if err != nil {
		t.Fatalf("dag json with multi-step chain failed: %v", err)
	}
}
