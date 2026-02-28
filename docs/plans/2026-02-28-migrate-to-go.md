# migrate-to-go Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `makemigrations migrate-to-go` — a command that converts legacy Goose `.sql` migrations to Go `RunSQL` migration files, migrates `goose_db_version` history to `makemigrations_history`, and deletes the old SQL files.

**Architecture:** A new Cobra command in `cmd/migrate_to_go.go` orchestrates three phases: (1) parse `.sql` files using a new `internal/gooseparser` package, (2) generate `.go` migration files via the existing `codegen.GoGenerator`, (3) optionally connect to the DB and migrate history using the existing `MigrationRecorder`. The `.sql` files are deleted only after both phases succeed.

**Tech Stack:** Cobra, `internal/codegen`, `internal/gooseparser` (new), `migrate.MigrationRecorder`, `database/sql` with existing DB drivers from `cmd/goose.go`.

---

### Task 1: Goose SQL parser

**Files:**
- Create: `internal/gooseparser/parser.go`
- Create: `internal/gooseparser/parser_test.go`

**Step 1: Write the failing tests**

Create `internal/gooseparser/parser_test.go`:

```go
package gooseparser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/gooseparser"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	return path
}

func TestParseFile_BasicUpDown(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if want := "CREATE TABLE users (id INTEGER PRIMARY KEY);"; got.ForwardSQL != want {
		t.Errorf("ForwardSQL:\ngot  %q\nwant %q", got.ForwardSQL, want)
	}
	if want := "DROP TABLE users;"; got.BackwardSQL != want {
		t.Errorf("BackwardSQL:\ngot  %q\nwant %q", got.BackwardSQL, want)
	}
}

func TestParseFile_StatementBeginEnd(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0002_complex.sql", `-- +goose Up
-- +goose StatementBegin
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    body TEXT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE posts;
-- +goose StatementEnd
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got.ForwardSQL == "" {
		t.Error("ForwardSQL should not be empty")
	}
	// StatementBegin/End markers should be stripped
	if contains(got.ForwardSQL, "StatementBegin") {
		t.Error("ForwardSQL should not contain StatementBegin marker")
	}
	if contains(got.BackwardSQL, "StatementEnd") {
		t.Error("BackwardSQL should not contain StatementEnd marker")
	}
}

func TestParseFile_NoDownSection(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0003_irreversible.sql", `-- +goose Up
INSERT INTO config (key, value) VALUES ('version', '2');
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got.ForwardSQL == "" {
		t.Error("ForwardSQL should not be empty")
	}
	if got.BackwardSQL != "" {
		t.Errorf("BackwardSQL should be empty, got %q", got.BackwardSQL)
	}
}

func TestParseFile_MultipleStatements(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0004_multi.sql", `-- +goose Up
CREATE TABLE a (id INTEGER PRIMARY KEY);
CREATE TABLE b (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE b;
DROP TABLE a;
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if !contains(got.ForwardSQL, "CREATE TABLE a") {
		t.Error("ForwardSQL missing CREATE TABLE a")
	}
	if !contains(got.ForwardSQL, "CREATE TABLE b") {
		t.Error("ForwardSQL missing CREATE TABLE b")
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"20240101120000_initial.sql", "initial"},
		{"00001_initial.sql", "initial"},
		{"20240102_add_phone_field.sql", "add_phone_field"},
		{"0001_my_migration.sql", "my_migration"},
	}
	for _, tt := range tests {
		got := gooseparser.ExtractDescription(tt.filename)
		if got != tt.want {
			t.Errorf("ExtractDescription(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestExtractVersionID(t *testing.T) {
	tests := []struct {
		filename string
		want     int64
		wantErr  bool
	}{
		{"20240101120000_initial.sql", 20240101120000, false},
		{"00001_initial.sql", 1, false},
		{"0003_add_phone.sql", 3, false},
		{"notanumber_bad.sql", 0, true},
	}
	for _, tt := range tests {
		got, err := gooseparser.ExtractVersionID(tt.filename)
		if (err != nil) != tt.wantErr {
			t.Errorf("ExtractVersionID(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ExtractVersionID(%q) = %d, want %d", tt.filename, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./internal/gooseparser/...
```

Expected: `cannot find package "github.com/ocomsoft/makemigrations/internal/gooseparser"`

**Step 3: Implement the parser**

Create `internal/gooseparser/parser.go`:

```go
/*
MIT License

Copyright (c) 2025 OcomSoft

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

// Package gooseparser parses Goose-format SQL migration files into forward
// and backward SQL strings for use in RunSQL migration operations.
package gooseparser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Migration holds the parsed content of a Goose SQL migration file.
type Migration struct {
	// ForwardSQL is the SQL from the -- +goose Up section.
	ForwardSQL string
	// BackwardSQL is the SQL from the -- +goose Down section.
	// Empty if no Down section is present.
	BackwardSQL string
}

// ParseFile reads a Goose-format .sql file and returns the parsed Migration.
// Goose markers (-- +goose Up, -- +goose Down, -- +goose StatementBegin,
// -- +goose StatementEnd) are stripped; all other content is preserved.
func ParseFile(path string) (Migration, error) {
	f, err := os.Open(path)
	if err != nil {
		return Migration{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	const (
		sectionNone    = 0
		sectionForward = 1
		sectionBackward = 2
	)

	var (
		forward  strings.Builder
		backward strings.Builder
		section  = sectionNone
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "-- +goose Up":
			section = sectionForward
		case trimmed == "-- +goose Down":
			section = sectionBackward
		case trimmed == "-- +goose StatementBegin" || trimmed == "-- +goose StatementEnd":
			// Strip these markers entirely
		default:
			switch section {
			case sectionForward:
				forward.WriteString(line)
				forward.WriteByte('\n')
			case sectionBackward:
				backward.WriteString(line)
				backward.WriteByte('\n')
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Migration{}, fmt.Errorf("scanning %s: %w", path, err)
	}

	return Migration{
		ForwardSQL:  strings.TrimSpace(forward.String()),
		BackwardSQL: strings.TrimSpace(backward.String()),
	}, nil
}

// ExtractDescription strips the leading numeric prefix and file extension from
// a Goose migration filename, returning just the description part.
//
// Examples:
//   "20240101120000_add_users.sql" → "add_users"
//   "00001_add_users.sql"          → "add_users"
func ExtractDescription(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), ".sql")
	idx := strings.Index(base, "_")
	if idx < 0 {
		return base
	}
	return base[idx+1:]
}

// ExtractVersionID parses the numeric prefix from a Goose migration filename
// as an int64. This matches the version_id stored in goose_db_version.
//
// Examples:
//   "20240101120000_add_users.sql" → 20240101120000
//   "00001_add_users.sql"          → 1
func ExtractVersionID(filename string) (int64, error) {
	base := strings.TrimSuffix(filepath.Base(filename), ".sql")
	idx := strings.Index(base, "_")
	prefix := base
	if idx >= 0 {
		prefix = base[:idx]
	}
	v, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("extracting version ID from %q: %w", filename, err)
	}
	return v, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./internal/gooseparser/... -v
```

Expected: all tests PASS.

**Step 5: Lint**

```bash
golangci-lint run --no-config ./internal/gooseparser/...
```

Expected: no issues on new code.

**Step 6: Commit**

```bash
git add internal/gooseparser/
git commit -m "feat(gooseparser): add Goose SQL file parser"
```

---

### Task 2: Core conversion logic

**Files:**
- Create: `cmd/migrate_to_go.go`
- Create: `cmd/migrate_to_go_test.go`

**Step 1: Write failing tests for the conversion logic**

Create `cmd/migrate_to_go_test.go`:

```go
package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/cmd"
)

func writeSQLFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestMigrateToGo_NoSQLFiles(t *testing.T) {
	dir := t.TempDir()
	err := cmd.ExecuteMigrateToGo(dir, true, true, false)
	if err == nil {
		t.Fatal("expected error when no SQL files present")
	}
}

func TestMigrateToGo_GeneratesGoFiles(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)
	writeSQLFile(t, dir, "00002_add_phone.sql", `-- +goose Up
ALTER TABLE users ADD COLUMN phone TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN phone;
`)

	err := cmd.ExecuteMigrateToGo(dir, true, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	// Go files should be created
	for _, name := range []string{"0001_initial.go", "0002_add_phone.go", "main.go", "go.mod"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to be created", name)
		}
	}
}

func TestMigrateToGo_SQLDeletedAfterConversion(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	// dry-run=false, no-history=true, no-delete=false → should delete
	err := cmd.ExecuteMigrateToGo(dir, false, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); !os.IsNotExist(err) {
		t.Error("expected SQL file to be deleted after successful conversion")
	}
}

func TestMigrateToGo_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	// dry-run=true → nothing should be written or deleted
	err := cmd.ExecuteMigrateToGo(dir, true, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo dry-run: %v", err)
	}

	goPath := filepath.Join(dir, "0001_initial.go")
	if _, err := os.Stat(goPath); !os.IsNotExist(err) {
		t.Error("dry-run: expected no .go files to be written")
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		t.Error("dry-run: expected SQL file to be preserved")
	}
}

func TestMigrateToGo_NoDelete(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	// dry-run=false, no-history=true, no-delete=true → SQL preserved
	err := cmd.ExecuteMigrateToGo(dir, false, true, true)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		t.Error("no-delete: expected SQL file to be preserved")
	}
}

func TestMigrateToGo_GeneratedContentIsValid(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)

	if err := cmd.ExecuteMigrateToGo(dir, false, true, true); err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "0001_initial.go"))
	if err != nil {
		t.Fatalf("reading 0001_initial.go: %v", err)
	}

	src := string(content)
	for _, want := range []string{
		"package main",
		`Name: "0001_initial"`,
		"RunSQL",
		"CREATE TABLE users",
		"DROP TABLE users",
	} {
		if !containsStr(src, want) {
			t.Errorf("generated file missing %q", want)
		}
	}
}

func TestMigrateToGo_ExistingGoFilesBlocked(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
`)
	// Simulate existing go migration file
	if err := os.WriteFile(filepath.Join(dir, "0001_initial.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	err := cmd.ExecuteMigrateToGo(dir, false, true, false)
	if err == nil {
		t.Fatal("expected error when .go migration files already exist")
	}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
```

Add `"strings"` to imports.

**Step 2: Run tests to verify they fail**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./cmd/... -run TestMigrateToGo -v
```

Expected: compile error — `cmd.ExecuteMigrateToGo` undefined.

**Step 3: Implement `cmd/migrate_to_go.go`**

Create `cmd/migrate_to_go.go`:

```go
/*
MIT License

Copyright (c) 2025 OcomSoft

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

package cmd

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/gooseparser"
	"github.com/spf13/cobra"
)

var (
	migrateToGoDryRun    bool
	migrateToGoNoHistory bool
	migrateToGoNoDelete  bool
	migrateToGoForce     bool
)

var migrateToGoCmd = &cobra.Command{
	Use:   "migrate-to-go",
	Short: "Convert legacy Goose SQL migrations to Go migration files",
	Long: `Converts legacy Goose .sql migration files to typed Go migration files
using RunSQL operations.

Auto-detects .sql files in the migrations directory. For each file, parses
the Goose Up/Down SQL sections and generates a corresponding .go migration
file. Also migrates the goose_db_version history table to makemigrations_history.

SQL files are deleted after successful conversion unless --no-delete is set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault(configFile)
		return ExecuteMigrateToGo(
			cfg.Migration.Directory,
			migrateToGoDryRun,
			migrateToGoNoHistory,
			migrateToGoNoDelete,
		)
	},
}

func init() {
	rootCmd.AddCommand(migrateToGoCmd)
	migrateToGoCmd.Flags().BoolVar(&migrateToGoDryRun, "dry-run", false,
		"Preview actions without writing files, migrating history, or deleting SQL files")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoHistory, "no-history", false,
		"Skip DB history migration (offline/CI use)")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoDelete, "no-delete", false,
		"Keep .sql files after successful conversion")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoForce, "force", false,
		"Overwrite existing .go migration files")
}

// ExecuteMigrateToGo performs the full SQL-to-Go migration for the given migrations directory.
// dryRun previews without writing. noHistory skips DB history migration.
// noDelete keeps .sql files after conversion.
func ExecuteMigrateToGo(migrationsDir string, dryRun, noHistory, noDelete bool) error {
	// Phase 1: discover SQL files
	sqlFiles, err := discoverSQLFiles(migrationsDir)
	if err != nil {
		return err
	}
	if len(sqlFiles) == 0 {
		return fmt.Errorf("no .sql migration files found in %s", migrationsDir)
	}

	// Guard: check for existing .go migration files (not main.go / go.mod)
	if !migrateToGoForce {
		if err := checkNoExistingGoMigrations(migrationsDir); err != nil {
			return err
		}
	}

	fmt.Printf("Detected %d SQL migration file(s) in %s\n\n", len(sqlFiles), migrationsDir)

	// Phase 2: parse and generate .go files
	type migEntry struct {
		sqlFile string
		goName  string
		versionID int64
	}

	entries := make([]migEntry, 0, len(sqlFiles))
	for i, sqlFile := range sqlFiles {
		desc := gooseparser.ExtractDescription(filepath.Base(sqlFile))
		goName := fmt.Sprintf("%04d_%s", i+1, desc)
		vid, err := gooseparser.ExtractVersionID(filepath.Base(sqlFile))
		if err != nil {
			return fmt.Errorf("extracting version ID from %s: %w", sqlFile, err)
		}
		entries = append(entries, migEntry{sqlFile: sqlFile, goName: goName, versionID: vid})
	}

	fmt.Println("Converting migrations:")
	goFiles := make([]string, 0, len(entries))
	for i, e := range entries {
		parsed, err := gooseparser.ParseFile(e.sqlFile)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", e.sqlFile, err)
		}

		deps := []string{}
		if i > 0 {
			deps = []string{entries[i-1].goName}
		}

		src, err := generateRunSQLMigration(e.goName, deps, parsed.ForwardSQL, parsed.BackwardSQL)
		if err != nil {
			return fmt.Errorf("generating %s.go: %w", e.goName, err)
		}

		goPath := filepath.Join(migrationsDir, e.goName+".go")
		fmt.Printf("  %-40s → %s", filepath.Base(e.sqlFile), goPath)

		if !dryRun {
			if err := os.WriteFile(goPath, []byte(src), 0644); err != nil {
				fmt.Println("  FAILED")
				return fmt.Errorf("writing %s: %w", goPath, err)
			}
			goFiles = append(goFiles, goPath)
		}
		fmt.Println("  ✓")
	}

	// Phase 3: generate main.go and go.mod if not present
	gen := codegen.NewGoGenerator()
	mainPath := filepath.Join(migrationsDir, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		fmt.Printf("\nGenerating %s", mainPath)
		if !dryRun {
			if err := os.WriteFile(mainPath, []byte(gen.GenerateMainGo()), 0644); err != nil {
				return fmt.Errorf("writing main.go: %w", err)
			}
		}
		fmt.Println("  ✓")
	}

	goModPath := filepath.Join(migrationsDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		moduleName := readModuleName() + "/migrations"
		fmt.Printf("Generating %s", goModPath)
		if !dryRun {
			if err := os.WriteFile(goModPath, []byte(gen.GenerateGoMod(moduleName, "v0.3.0")), 0644); err != nil {
				return fmt.Errorf("writing go.mod: %w", err)
			}
		}
		fmt.Println("  ✓")
	}

	// Phase 4: history migration
	if !noHistory {
		cfg := config.LoadOrDefault(configFile)
		if err := migrateGooseHistory(cfg, entries, dryRun); err != nil {
			fmt.Printf("\nWarning: history migration failed: %v\n", err)
			fmt.Println("Go files were generated successfully. History migration can be retried.")
			// Don't delete SQL files if history failed
			return err
		}
	}

	// Phase 5: delete SQL files
	if !noDelete && !dryRun {
		fmt.Println("\nRemoving SQL files:")
		for _, e := range entries {
			fmt.Printf("  %s", e.sqlFile)
			if err := os.Remove(e.sqlFile); err != nil {
				fmt.Println("  WARNING: failed to delete:", err)
			} else {
				fmt.Println("  ✓")
			}
		}
	} else if dryRun {
		fmt.Printf("\n[dry-run] Would delete %d SQL file(s)\n", len(entries))
	}

	fmt.Printf(`
Migration complete. Next steps:

  cd %s && go mod tidy && go build -o migrate .
  ./migrate status
`, migrationsDir)

	return nil
}

// discoverSQLFiles returns all *.sql files in migrationsDir, sorted alphabetically.
func discoverSQLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// checkNoExistingGoMigrations returns an error if .go migration files
// (files other than main.go) already exist in the directory.
func checkNoExistingGoMigrations(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".go") && name != "main.go" {
			return fmt.Errorf(
				"Go migration files already exist in %s (e.g. %s). "+
					"Use --force to overwrite", dir, name)
		}
	}
	return nil
}

// generateRunSQLMigration produces the Go source for a migration that wraps
// raw SQL in a RunSQL operation.
func generateRunSQLMigration(name string, deps []string, forwardSQL, backwardSQL string) (string, error) {
	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("%q", d)
	}
	depList := strings.Join(depStrs, ", ")

	src := fmt.Sprintf(`package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
	m.Register(&m.Migration{
		Name:         %q,
		Dependencies: []string{%s},
		Operations: []m.Operation{
			&m.RunSQL{
				ForwardSQL:  %s,
				BackwardSQL: %s,
			},
		},
	})
}
`, name, depList, goRawString(forwardSQL), goRawString(backwardSQL))

	formatted, err := format.Source([]byte(src))
	if err != nil {
		return src, fmt.Errorf("formatting generated code: %w", err)
	}
	return string(formatted), nil
}

// goRawString wraps s in a Go raw string literal (backtick), escaping any
// backticks in the content by splitting to concatenated strings.
func goRawString(s string) string {
	if s == "" {
		return `""`
	}
	// If no backticks in content, use raw string directly
	if !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	// Split on backticks and concatenate interpreted + raw strings
	parts := strings.Split(s, "`")
	result := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 {
			result = append(result, "\"` + \"`\" + `\"")
		}
		if p != "" {
			result = append(result, "`"+p+"`")
		}
	}
	return strings.Join(result, " + ")
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./cmd/... -run TestMigrateToGo -v
```

Expected: all TestMigrateToGo tests PASS.

**Step 5: Lint**

```bash
goimports -w cmd/migrate_to_go.go
golangci-lint run --no-config ./cmd/...
```

Fix any issues reported. Pre-existing issues in other files are acceptable.

**Step 6: Commit**

```bash
git add cmd/migrate_to_go.go cmd/migrate_to_go_test.go
git commit -m "feat(cmd): add migrate-to-go command — convert SQL migrations to Go RunSQL files"
```

---

### Task 3: History migration

**Files:**
- Modify: `cmd/migrate_to_go.go` — add `migrateGooseHistory` function
- Modify: `cmd/migrate_to_go_test.go` — add DB history tests

**Step 1: Write the failing DB history test**

Add to `cmd/migrate_to_go_test.go`:

```go
import (
    "database/sql"
    "strings"
    _ "github.com/mattn/go-sqlite3"
)

func TestMigrateGooseHistory_MarksAppliedMigrations(t *testing.T) {
	dir := t.TempDir()

	// Create SQL files
	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)
	writeSQLFile(t, dir, "00002_add_phone.sql", `-- +goose Up
ALTER TABLE users ADD COLUMN phone TEXT;
-- +goose Down
ALTER TABLE users DROP COLUMN phone;
`)

	// Set up SQLite DB with goose_db_version data
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("opening DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("creating goose_db_version: %v", err)
	}

	// version 1 is applied, version 2 is pending
	_, err = db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1), (2, 0)`)
	if err != nil {
		t.Fatalf("inserting goose history: %v", err)
	}

	// Run history migration
	if err := cmd.ExecuteMigrateGooseHistory(db, dir, false); err != nil {
		t.Fatalf("ExecuteMigrateGooseHistory: %v", err)
	}

	// Check makemigrations_history
	rows, err := db.Query("SELECT name FROM makemigrations_history ORDER BY name")
	if err != nil {
		t.Fatalf("querying history: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		names = append(names, n)
	}

	if len(names) != 1 {
		t.Fatalf("expected 1 applied migration, got %d: %v", len(names), names)
	}
	if names[0] != "0001_initial" {
		t.Errorf("expected 0001_initial, got %q", names[0])
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./cmd/... -run TestMigrateGooseHistory -v
```

Expected: compile error — `cmd.ExecuteMigrateGooseHistory` undefined.

**Step 3: Implement `migrateGooseHistory`**

Add to `cmd/migrate_to_go.go`:

```go
import (
    "database/sql"
    "github.com/ocomsoft/makemigrations/migrate"
)

// migrateEntry pairs a SQL filename's Goose version ID with the new Go migration name.
type migrateEntry struct {
    sqlFile   string
    goName    string
    versionID int64
}

// migrateGooseHistory reads the goose_db_version table and copies applied
// migration records into makemigrations_history using the new Go migration names.
// This is a separate exported function to allow testing with an injected DB.
func ExecuteMigrateGooseHistory(db *sql.DB, migrationsDir string, dryRun bool) error {
    sqlFiles, err := discoverSQLFiles(migrationsDir)
    if err != nil {
        return err
    }

    entries := make([]migrateEntry, 0, len(sqlFiles))
    for i, f := range sqlFiles {
        desc := gooseparser.ExtractDescription(filepath.Base(f))
        goName := fmt.Sprintf("%04d_%s", i+1, desc)
        vid, err := gooseparser.ExtractVersionID(filepath.Base(f))
        if err != nil {
            return err
        }
        entries = append(entries, migrateEntry{sqlFile: f, goName: goName, versionID: vid})
    }

    return migrateHistoryWithEntries(db, entries, dryRun)
}

// migrateGooseHistory is the internal version used by ExecuteMigrateToGo.
func migrateGooseHistory(cfg *config.Config, entries []struct {
    sqlFile   string
    goName    string
    versionID int64
}, dryRun bool) error {
    db, err := setupGooseDB(cfg)
    if err != nil {
        return fmt.Errorf("connecting to database: %w", err)
    }
    defer db.Close()

    mapped := make([]migrateEntry, len(entries))
    for i, e := range entries {
        mapped[i] = migrateEntry{sqlFile: e.sqlFile, goName: e.goName, versionID: e.versionID}
    }
    return migrateHistoryWithEntries(db, mapped, dryRun)
}

// migrateHistoryWithEntries does the actual history migration against an open DB.
func migrateHistoryWithEntries(db *sql.DB, entries []migrateEntry, dryRun bool) error {
    // Read all goose_db_version rows, last write per version_id wins
    rows, err := db.Query("SELECT version_id, is_applied FROM goose_db_version ORDER BY id ASC")
    if err != nil {
        return fmt.Errorf("querying goose_db_version: %w", err)
    }
    defer rows.Close()

    appliedVersions := map[int64]bool{}
    for rows.Next() {
        var vid int64
        var isApplied bool
        if err := rows.Scan(&vid, &isApplied); err != nil {
            return fmt.Errorf("scanning goose_db_version: %w", err)
        }
        appliedVersions[vid] = isApplied
    }
    if err := rows.Err(); err != nil {
        return fmt.Errorf("iterating goose_db_version: %w", err)
    }

    recorder := migrate.NewMigrationRecorder(db)
    if !dryRun {
        if err := recorder.EnsureTable(); err != nil {
            return err
        }
    }

    fmt.Println("\nMigrating history (goose_db_version → makemigrations_history):")
    for _, e := range entries {
        isApplied := appliedVersions[e.versionID]
        status := "pending"
        if isApplied {
            status = "applied"
        }
        fmt.Printf("  %-40s %s", e.goName, status)
        if isApplied && !dryRun {
            if err := recorder.RecordApplied(e.goName); err != nil {
                fmt.Println("  FAILED")
                return fmt.Errorf("recording %s as applied: %w", e.goName, err)
            }
        }
        fmt.Println("  ✓")
    }
    return nil
}
```

> **Note:** The internal `migrateGooseHistory` function needs to match the signature used in `ExecuteMigrateToGo`. Refactor the call in `ExecuteMigrateToGo` to pass a `[]migrateEntry` slice.

**Step 4: Refactor `ExecuteMigrateToGo` to use the shared entry type**

In `ExecuteMigrateToGo`, change the anonymous struct to use `migrateEntry`:

```go
entries := make([]migrateEntry, 0, len(sqlFiles))
// ... build entries ...

if !noHistory {
    cfg := config.LoadOrDefault(configFile)
    db, err := setupGooseDB(cfg)
    if err != nil {
        // warn and skip history
        fmt.Printf("\nWarning: could not connect to DB for history migration: %v\n", err)
        fmt.Println("Go files were generated. Re-run with correct DB env vars to migrate history.")
    } else {
        defer db.Close()
        if err := migrateHistoryWithEntries(db, entries, dryRun); err != nil {
            return err
        }
    }
}
```

**Step 5: Run all tests**

```bash
go test ./cmd/... -run TestMigrateToGo -v
go test ./cmd/... -run TestMigrateGooseHistory -v
```

Expected: all PASS.

**Step 6: Full test suite**

```bash
go test ./...
```

Expected: all PASS.

**Step 7: Lint**

```bash
goimports -w cmd/migrate_to_go.go cmd/migrate_to_go_test.go
golangci-lint run --no-config ./cmd/...
```

**Step 8: Commit**

```bash
git add cmd/migrate_to_go.go cmd/migrate_to_go_test.go
git commit -m "feat(cmd): add goose history migration to migrate-to-go command"
```

---

### Task 4: Documentation

**Files:**
- Create: `docs/commands/migrate_to_go.md`
- Modify: `docs/installation.md` — add migration path section

**Step 1: Create `docs/commands/migrate_to_go.md`**

```markdown
# migrate-to-go

Converts legacy Goose `.sql` migration files to Go migration files and migrates
the `goose_db_version` history table to `makemigrations_history`.

## Synopsis

    makemigrations migrate-to-go [flags]

## Detection

Auto-detects `.sql` files in the configured `migrations/` directory.
Stops with an error if none are found or if `.go` migration files already exist.

## What It Does

1. Sorts `.sql` files alphabetically
2. Parses each file's `-- +goose Up` / `-- +goose Down` sections
3. Generates `0001_description.go`, `0002_description.go`, ... with `RunSQL` operations
4. Generates `main.go` and `go.mod` if not present
5. Connects to the database and migrates `goose_db_version` → `makemigrations_history`
6. Deletes the original `.sql` files

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview all actions without writing, migrating, or deleting |
| `--no-history` | `false` | Skip the database history migration step |
| `--no-delete` | `false` | Keep `.sql` files after successful conversion |
| `--force` | `false` | Overwrite existing `.go` migration files |

## Database Connection

Uses the same `MAKEMIGRATIONS_DB_*` environment variables as `makemigrations goose`:

    MAKEMIGRATIONS_DB_HOST, MAKEMIGRATIONS_DB_PORT, MAKEMIGRATIONS_DB_USER,
    MAKEMIGRATIONS_DB_PASSWORD, MAKEMIGRATIONS_DB_NAME, MAKEMIGRATIONS_DB_SSLMODE

Set these before running (or use `--no-history` to skip the DB step).

## Example

    export MAKEMIGRATIONS_DB_HOST=localhost
    export MAKEMIGRATIONS_DB_USER=myuser
    export MAKEMIGRATIONS_DB_PASSWORD=secret
    export MAKEMIGRATIONS_DB_NAME=mydb

    makemigrations migrate-to-go

    # Then build and verify
    cd migrations && go mod tidy && go build -o migrate .
    ./migrate status

## After Migration

The `.schema_snapshot.yaml` is preserved — it is still used by
`makemigrations makemigrations` to diff against future schema changes.
```

**Step 2: Commit**

```bash
git add docs/commands/migrate_to_go.md
git commit -m "docs: add migrate-to-go command reference"
```

---

### Task 5: Final verification

**Step 1: Full test suite**

```bash
cd /workspaces/ocom/go/makemigrations
go test ./...
```

Expected: all PASS, no failures.

**Step 2: Build check**

```bash
go build ./...
```

Expected: no errors.

**Step 3: Lint all new code**

```bash
golangci-lint run --no-config ./internal/gooseparser/... ./cmd/...
```

Expected: no new issues.

**Step 4: Smoke test (manual)**

```bash
mkdir /tmp/smoke_test_migrations
cat > /tmp/smoke_test_migrations/00001_initial.sql << 'EOF'
-- +goose Up
CREATE TABLE smoke_users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE smoke_users;
EOF
cat > /tmp/smoke_test_migrations/00002_add_phone.sql << 'EOF'
-- +goose Up
ALTER TABLE smoke_users ADD COLUMN phone TEXT;
-- +goose Down
ALTER TABLE smoke_users DROP COLUMN phone;
EOF

# Preview
go run . migrate-to-go --dry-run --no-history

# Actual conversion (no-history since no DB available in dev)
go run . migrate-to-go --no-history --no-delete

# Inspect generated files
cat /tmp/smoke_test_migrations/0001_initial.go
cat /tmp/smoke_test_migrations/0002_add_phone.go
```

Expected: well-formatted Go files with `RunSQL` operations containing the original SQL.

**Step 5: Final commit if any fixes made**

```bash
git add -A
git commit -m "fix: address issues found during final verification"
```
