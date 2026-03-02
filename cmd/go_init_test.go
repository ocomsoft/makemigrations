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
package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/cmd"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// TestExecuteGoMigrationInit_AutoUpgradeSQL verifies that when *.sql files exist
// in the migrations directory, init --go automatically delegates to migrate-to-go.
func TestExecuteGoMigrationInit_AutoUpgradeSQL(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testproject\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a Goose SQL migration
	sqlContent := "-- +goose Up\nCREATE TABLE users (id INTEGER PRIMARY KEY);\n\n-- +goose Down\nDROP TABLE users;\n"
	if err := os.WriteFile(filepath.Join(migrationsDir, "00001_initial.sql"), []byte(sqlContent), 0644); err != nil {
		t.Fatalf("WriteFile sql: %v", err)
	}

	// Write a snapshot so migrate-to-go can generate the schema-state migration
	schema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{Name: "users", Fields: []yamlpkg.Field{{Name: "id", Type: "integer"}}},
		},
	}
	sm := yamlpkg.NewStateManager(false)
	if err := sm.SaveSchemaSnapshot(schema, migrationsDir); err != nil {
		t.Fatalf("SaveSchemaSnapshot: %v", err)
	}

	if err := cmd.ExecuteGoMigrationInit("postgresql", false); err != nil {
		t.Fatalf("ExecuteGoMigrationInit: %v", err)
	}

	// migrate-to-go should have created Go files and removed the SQL file
	if _, err := os.Stat(filepath.Join(migrationsDir, "main.go")); os.IsNotExist(err) {
		t.Error("expected main.go to be created by migrate-to-go")
	}
	if _, err := os.Stat(filepath.Join(migrationsDir, "go.mod")); os.IsNotExist(err) {
		t.Error("expected go.mod to be created by migrate-to-go")
	}
	// The SQL file should have been deleted (deleteSQL=true)
	if _, err := os.Stat(filepath.Join(migrationsDir, "00001_initial.sql")); err == nil {
		t.Error("expected SQL file to be removed after conversion")
	}
}

// TestExecuteGoMigrationInit_NoSnapshot verifies that init --go creates main.go
// and go.mod in the migrations directory when no snapshot exists.
func TestExecuteGoMigrationInit_NoSnapshot(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	// Create go.mod so readModuleName doesn't return "myproject"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testproject\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "migrations"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := cmd.ExecuteGoMigrationInit("postgresql", false); err != nil {
		t.Fatalf("ExecuteGoMigrationInit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "migrations", "main.go")); os.IsNotExist(err) {
		t.Error("expected main.go to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, "migrations", "go.mod")); os.IsNotExist(err) {
		t.Error("expected go.mod to be created")
	}
	// No snapshot means no initial migration file
	entries, err := os.ReadDir(filepath.Join(dir, "migrations"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".go" && e.Name() != "main.go" {
			t.Errorf("unexpected migration file created: %s", e.Name())
		}
	}
}

// TestExecuteGoMigrationInit_Idempotent verifies that running init --go twice
// does not overwrite existing main.go or go.mod.
func TestExecuteGoMigrationInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testproject\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// First run
	if err := cmd.ExecuteGoMigrationInit("postgresql", false); err != nil {
		t.Fatalf("first ExecuteGoMigrationInit: %v", err)
	}

	// Overwrite main.go with sentinel content
	sentinel := "// sentinel\npackage main\n"
	if err := os.WriteFile(filepath.Join(migrationsDir, "main.go"), []byte(sentinel), 0644); err != nil {
		t.Fatalf("WriteFile sentinel: %v", err)
	}

	// Second run should NOT overwrite
	if err := cmd.ExecuteGoMigrationInit("postgresql", false); err != nil {
		t.Fatalf("second ExecuteGoMigrationInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(migrationsDir, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile main.go: %v", err)
	}
	if string(data) != sentinel {
		t.Error("expected main.go to remain unchanged on second run")
	}
}
