# `db-diff` Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `db-diff` command that compares the live database schema against the schema that the migration DAG says _should_ exist, and reports the differences.

**Architecture:** The command uses two existing code paths already present in the project: (1) `queryDAG()` + `schemaStateToYAMLSchema()` (from `go_migrations.go`) to build the "expected" schema from the migration chain, and (2) `provider.GetDatabaseSchema()` (from the `db2schema` flow) to read the live database schema. These two `*yamlpkg.Schema` values are fed into the existing `DiffEngine.CompareSchemas()` to produce a diff, which a new formatter displays in a human-readable, color-coded form. Both `queryDAG` and `schemaStateToYAMLSchema` are unexported functions in package `cmd`, so they are directly accessible from the new file.

**Tech Stack:** Go, Cobra, `fatih/color`, existing `internal/yaml`, `internal/providers`, `internal/config`, `migrate` packages.

---

## Context for the Implementer

### Key existing helpers (all in package `cmd`, directly usable):

| Function | File | What it does |
|---|---|---|
| `queryDAG(migrationsDir, verbose)` | `cmd/go_migrations.go:362` | Builds migration binary, runs `dag --format json`, returns `*migrate.DAGOutput` |
| `schemaStateToYAMLSchema(state)` | `cmd/go_migrations.go:385` | Converts `*migrate.SchemaState` → `*yamlpkg.Schema` |
| `buildConnectionString(dbType)` | `cmd/db2schema.go:207` | Builds DB connection string from package-level flag vars |
| `host, port, database, username, password, sslmode` | `cmd/db2schema.go:41-51` | Package-level flag vars for connection (reuse, don't redeclare) |
| `databaseType` | `cmd/dump_sql.go:32` | Package-level flag var for DB type (reuse, don't redeclare) |
| `verbose` | `cmd/root.go` | Package-level verbose flag var (reuse, don't redeclare) |
| `configFile` | `cmd/root.go` | Package-level config file flag var (reuse, don't redeclare) |

### Type compatibility:

`yamlpkg.Schema = types.Schema` (type alias in `internal/yaml/types.go:32`). Therefore `*types.Schema` returned by `provider.GetDatabaseSchema()` is the same type as `*yamlpkg.Schema` accepted by `DiffEngine.CompareSchemas()`. No conversion needed.

### Diff direction convention:

```
DiffEngine.CompareSchemas(dagSchema, dbSchema)
```

- `ChangeTypeTableAdded` → table exists in DB but NOT in DAG (extra/unexpected table in DB)
- `ChangeTypeTableRemoved` → table in DAG but NOT in DB (migration not applied yet)
- `ChangeTypeFieldAdded` → field in DB but NOT in DAG (extra field in live DB)
- `ChangeTypeFieldRemoved` → field in DAG but NOT in DB (missing from live DB)
- `ChangeTypeFieldModified` → field exists in both but has different properties

### Type representation caveat:

The DAG schema field types use YAML-normalized names (e.g., `varchar`, `integer`, `timestamp`). The DB schema from `GetDatabaseSchema()` may return SQL-native types (e.g., `character varying`, `int4`, `timestamp without time zone`). The implementer should add a `normalizeDBSchema()` function that maps known SQL-native types back to YAML types before comparison. A mapping table is provided in Task 5.

### No migrations yet?

If the migrations directory has no `*.go` files (excluding `main.go`), the DAG schema is empty (`&yamlpkg.Schema{}`). In this case every table in the DB appears as `ChangeTypeTableAdded` (extra/unexpected).

---

## Task 1: Create the git worktree

> This worktree already exists at `.worktrees/db-diff` on branch `feature/db-diff`. All work happens there.

**Step 1: Change to the worktree**

```bash
cd /workspaces/ocom/go/makemigrations/.worktrees/db-diff
```

**Step 2: Verify you are on the right branch**

```bash
git branch
```

Expected: `* feature/db-diff`

**Step 3: Confirm the project compiles before starting**

```bash
go build ./...
```

Expected: no output (success)

**Step 4: Run existing tests**

```bash
go test ./...
```

Expected: all pass

**Step 5: Commit baseline**

```bash
git add .
git commit -m "chore: start feature/db-diff worktree" --allow-empty
```

---

## Task 2: Write failing tests for the `db-diff` command

**Files:**
- Create: `cmd/db_diff_test.go`

The command itself doesn't exist yet, so these tests will fail to compile. That is correct — this is TDD.

**Step 1: Write the failing test file**

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
	"bytes"
	"strings"
	"testing"

	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// TestDBDiffCommandRegistered verifies that the db-diff command is registered
// with the root command.
func TestDBDiffCommandRegistered(t *testing.T) {
	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "db-diff" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("db-diff command is not registered with rootCmd")
	}
}

// TestDBDiffCommandHasRequiredFlags verifies that the db-diff command exposes
// the expected flags.
func TestDBDiffCommandHasRequiredFlags(t *testing.T) {
	var cmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "db-diff" {
			cmd = sub
			break
		}
	}
	if cmd == nil {
		t.Fatal("db-diff command not found")
	}

	requiredFlags := []string{"host", "port", "database", "username", "password", "sslmode", "db-type", "format"}
	for _, name := range requiredFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("db-diff command missing flag: --%s", name)
		}
	}
}

// TestNormalizeDBSchema verifies that SQL-native type names are mapped to
// YAML-normalized type names used by the DAG schema.
func TestNormalizeDBSchema(t *testing.T) {
	nullable := true
	schema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "character varying"},
					{Name: "age", Type: "int4"},
					{Name: "score", Type: "double precision"},
					{Name: "active", Type: "bool"},
					{Name: "created", Type: "timestamp without time zone"},
					{Name: "notes", Type: "text"},   // already YAML-normalized
					{Name: "price", Type: "numeric"}, // maps to decimal
					{Name: "data", Type: "jsonb"},    // already OK
					{Name: "uid", Type: "uuid"},      // already OK
					{Name: "count", Type: "int8"},
					{Name: "small", Type: "int2"},
				},
			},
		},
	}
	// Nullable ptr is set to avoid nil dereference in other code paths
	for i := range schema.Tables[0].Fields {
		schema.Tables[0].Fields[i].Nullable = &nullable
	}

	normalizeDBSchema(schema)

	expected := map[string]string{
		"id":      "varchar",
		"age":     "integer",
		"score":   "float",
		"active":  "boolean",
		"created": "timestamp",
		"notes":   "text",
		"price":   "decimal",
		"data":    "jsonb",
		"uid":     "uuid",
		"count":   "bigint",
		"small":   "integer",
	}

	for _, f := range schema.Tables[0].Fields {
		want, ok := expected[f.Name]
		if !ok {
			t.Errorf("unexpected field %q in test schema", f.Name)
			continue
		}
		if f.Type != want {
			t.Errorf("field %q: got type %q, want %q", f.Name, f.Type, want)
		}
	}
}

// TestFormatDBDiff_NoChanges verifies that the formatter correctly reports
// "no differences" when the diff is empty.
func TestFormatDBDiff_NoChanges(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: false,
		Changes:    nil,
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "No differences") {
		t.Errorf("expected 'No differences' in output, got:\n%s", output)
	}
}

// TestFormatDBDiff_MissingTable verifies that a table_removed change
// (table in DAG but missing from DB) is reported correctly.
func TestFormatDBDiff_MissingTable(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeTableRemoved,
				TableName:   "audit_log",
				Description: "Table audit_log removed",
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "audit_log") {
		t.Errorf("expected 'audit_log' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Missing") {
		t.Errorf("expected 'Missing' section in output, got:\n%s", output)
	}
}

// TestFormatDBDiff_ExtraTable verifies that a table_added change
// (table in DB but not in DAG) is reported correctly.
func TestFormatDBDiff_ExtraTable(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeTableAdded,
				TableName:   "temp_cache",
				Description: "Table temp_cache added",
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "temp_cache") {
		t.Errorf("expected 'temp_cache' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Extra") {
		t.Errorf("expected 'Extra' section in output, got:\n%s", output)
	}
}

// TestFormatDBDiff_FieldDiff verifies that field-level differences are shown
// under the correct table name.
func TestFormatDBDiff_FieldDiff(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeFieldRemoved,
				TableName:   "users",
				FieldName:   "deleted_at",
				Description: "Field deleted_at removed from users",
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "users") {
		t.Errorf("expected 'users' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "deleted_at") {
		t.Errorf("expected 'deleted_at' in output, got:\n%s", output)
	}
}

// TestFormatDBDiff_Summary verifies that the summary line at the bottom
// contains counts matching the number of changes in each category.
func TestFormatDBDiff_Summary(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{Type: yamlpkg.ChangeTypeTableRemoved, TableName: "t1"},
			{Type: yamlpkg.ChangeTypeTableAdded, TableName: "t2"},
			{Type: yamlpkg.ChangeTypeFieldRemoved, TableName: "t3", FieldName: "f1"},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	// Summary line should mention 3 differences total
	if !strings.Contains(output, "3") {
		t.Errorf("expected total count '3' in summary, got:\n%s", output)
	}
}
```

**Step 2: Add missing import to the test file**

The test references `cobra` — add it:
```go
import (
    "bytes"
    "strings"
    "testing"

    "github.com/spf13/cobra"
    yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)
```

**Step 3: Attempt to compile (expect failure)**

```bash
go test ./cmd/... 2>&1 | head -30
```

Expected: compile errors because `db-diff` command, `normalizeDBSchema`, and `formatDBDiff` don't exist yet.

---

## Task 3: Create the command skeleton — make registration tests pass

**Files:**
- Create: `cmd/db_diff.go`

The goal of this task is to make `TestDBDiffCommandRegistered` and `TestDBDiffCommandHasRequiredFlags` pass. Implement the full file structure but leave `runDBDiff` as a stub.

**Step 1: Write `cmd/db_diff.go`**

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
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/providers"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// dbDiffFormat is the output format for the db-diff command (text or json).
var dbDiffFormat string

// dbDiffCmd compares the live database schema against the expected schema
// derived from the migration DAG.
var dbDiffCmd = &cobra.Command{
	Use:   "db-diff",
	Short: "Compare live database schema against migration DAG state",
	Long: `Compare the schema currently in the live database against the schema
that should exist based on the migration DAG (the accumulated state of all
applied migration files).

Differences are grouped into three categories:

  Missing from DB  - Tables or columns that exist in the migrations but are
                     absent from the live database. These are likely pending
                     migrations that have not been applied yet.

  Extra in DB      - Tables or columns that exist in the live database but are
                     not tracked by any migration. These may be manually created
                     objects or legacy schema elements.

  Field Differences - Columns that exist in both schemas but with different
                     definitions (type, nullability, defaults, etc.).

Database Connection:
  Specify connection details via flags (same flags as db2schema). Config file
  settings are used as fallback. Database type is set with --db-type.

Examples:
  # Compare against a local PostgreSQL database
  makemigrations db-diff --host=localhost --port=5432 --database=myapp \
      --username=myuser --password=mypass

  # Use config file connection settings
  makemigrations db-diff --config=migrations/makemigrations.config.yaml

  # JSON output for scripting
  makemigrations db-diff --format=json --host=localhost --database=myapp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDBDiff(cmd)
	},
}

// runDBDiff is the main implementation of the db-diff command.
// It loads the expected schema from the migration DAG and the actual schema
// from the live database, diffs them, and prints the results.
func runDBDiff(cmd *cobra.Command) error {
	// Load config for migrations directory
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory

	// Determine database type from --db-type flag
	dbType, err := yamlpkg.ParseDatabaseType(databaseType)
	if err != nil {
		return fmt.Errorf("invalid database type %q: %w", databaseType, err)
	}

	// ── Step 1: Load the "expected" schema from the migration DAG ──────────
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "1. Loading migration DAG schema state...\n")
	}

	var dagSchema *yamlpkg.Schema

	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	// Filter out main.go — it is not a migration file
	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	if len(migFiles) > 0 {
		dagOut, err := queryDAG(migrationsDir, verbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		dagSchema = schemaStateToYAMLSchema(dagOut.SchemaState)
	} else {
		if verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "   No migration files found; treating DAG schema as empty.\n")
		}
		dagSchema = &yamlpkg.Schema{}
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "   DAG schema: %d table(s)\n", len(dagSchema.Tables))
	}

	// ── Step 2: Load the "actual" schema from the live database ───────────
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "2. Connecting to live database...\n")
	}

	connectionString := buildConnectionString(dbType)
	provider, err := providers.NewProvider(dbType, nil)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	dbSchema, err := provider.GetDatabaseSchema(connectionString)
	if err != nil {
		return fmt.Errorf("extracting database schema: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "   DB schema: %d table(s)\n", len(dbSchema.Tables))
	}

	// ── Step 3: Normalize SQL-native type names in the DB schema ──────────
	// The DAG schema uses YAML-normalized type names (varchar, integer, …).
	// The DB provider may return SQL-native names (character varying, int4, …).
	// Normalizing makes the diff meaningful rather than full of type-name noise.
	normalizeDBSchema(dbSchema)

	// ── Step 4: Diff ──────────────────────────────────────────────────────
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "3. Computing diff (DAG → DB)...\n")
	}

	diffEngine := yamlpkg.NewDiffEngine(verbose)
	diff, err := diffEngine.CompareSchemas(dagSchema, dbSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	// ── Step 5: Format and print results ──────────────────────────────────
	if dbDiffFormat == "json" {
		return formatDBDiffJSON(cmd.OutOrStdout(), diff)
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, verbose)
	fmt.Fprint(cmd.OutOrStdout(), buf.String())

	// Exit with code 1 if differences were found so that CI can detect drift.
	if diff.HasChanges {
		return fmt.Errorf("schema drift detected: %d difference(s) found", len(diff.Changes))
	}

	return nil
}

// normalizeDBSchema converts SQL-native field type names to the YAML-normalized
// names used by the migration DAG schema, so that the two schemas can be
// compared without spurious type-name differences.
//
// The mapping covers the most common PostgreSQL and SQLite type aliases.
// Unknown types are left unchanged.
func normalizeDBSchema(schema *yamlpkg.Schema) {
	typeMap := map[string]string{
		// PostgreSQL aliases for varchar
		"character varying": "varchar",
		"character":         "varchar",
		"char":              "varchar",
		// Integer variants
		"int":     "integer",
		"int2":    "integer",
		"int4":    "integer",
		"int8":    "bigint",
		"int16":   "bigint",
		"smallint": "integer",
		// Float variants
		"float4":           "float",
		"float8":           "float",
		"double precision": "float",
		"real":             "float",
		// Decimal / numeric
		"numeric": "decimal",
		// Boolean
		"bool": "boolean",
		// Timestamp variants
		"timestamp without time zone": "timestamp",
		"timestamp with time zone":    "timestamp",
		"timestamptz":                 "timestamp",
		// Date / time (keep as-is, already matches YAML names)
		// Serial (PostgreSQL sequences that back serial columns)
		"serial4": "serial",
		"serial8": "serial",
		// JSON
		"json": "json", // already OK
	}

	for ti := range schema.Tables {
		for fi := range schema.Tables[ti].Fields {
			normalized, ok := typeMap[strings.ToLower(schema.Tables[ti].Fields[fi].Type)]
			if ok {
				schema.Tables[ti].Fields[fi].Type = normalized
			}
		}
	}
}

// formatDBDiff writes a human-readable, color-coded diff report to w.
//
// The report groups changes into three sections:
//   - Missing from DB (ChangeTypeTableRemoved / ChangeTypeFieldRemoved): expected
//     by migrations but absent from the live DB.
//   - Extra in DB (ChangeTypeTableAdded / ChangeTypeFieldAdded): present in the
//     live DB but not tracked by any migration.
//   - Field differences (ChangeTypeFieldModified / other): columns exist in
//     both schemas but with different definitions.
func formatDBDiff(w io.Writer, diff *yamlpkg.SchemaDiff, verbose bool) {
	bold := color.New(color.Bold)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)

	bold.Fprintln(w, "DB-Diff Report")
	bold.Fprintln(w, "==============")

	if !diff.HasChanges {
		green.Fprintln(w, "\nNo differences found. The live database matches the migration DAG schema.")
		return
	}

	// Partition changes by category
	var missingTables, extraTables []yamlpkg.Change
	fieldChanges := make(map[string][]yamlpkg.Change) // keyed by table name

	for _, ch := range diff.Changes {
		switch ch.Type {
		case yamlpkg.ChangeTypeTableRemoved:
			missingTables = append(missingTables, ch)
		case yamlpkg.ChangeTypeTableAdded:
			extraTables = append(extraTables, ch)
		default:
			// Field-level changes are grouped under their table name
			fieldChanges[ch.TableName] = append(fieldChanges[ch.TableName], ch)
		}
	}

	// Sort table names in field-changes map for deterministic output
	var fieldTables []string
	for t := range fieldChanges {
		fieldTables = append(fieldTables, t)
	}
	sort.Strings(fieldTables)

	// ── Missing from DB ────────────────────────────────────────────────────
	fmt.Fprintln(w)
	bold.Fprintf(w, "Missing from DB (%d):\n", len(missingTables))
	if len(missingTables) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, ch := range missingTables {
			red.Fprintf(w, "  ✗ %s\n", ch.TableName)
			fmt.Fprintf(w, "    → Table exists in migrations but is absent from the live database.\n")
			fmt.Fprintf(w, "    → Run pending migrations or check if the migration has been applied.\n")
		}
	}

	// ── Extra in DB ────────────────────────────────────────────────────────
	fmt.Fprintln(w)
	bold.Fprintf(w, "Extra in DB (%d):\n", len(extraTables))
	if len(extraTables) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, ch := range extraTables {
			yellow.Fprintf(w, "  + %s\n", ch.TableName)
			fmt.Fprintf(w, "    → Table exists in the live database but is not tracked by any migration.\n")
		}
	}

	// ── Field Differences ─────────────────────────────────────────────────
	totalFieldChanges := 0
	for _, changes := range fieldChanges {
		totalFieldChanges += len(changes)
	}

	fmt.Fprintln(w)
	bold.Fprintf(w, "Field Differences (%d change(s) across %d table(s)):\n", totalFieldChanges, len(fieldTables))
	if len(fieldTables) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, tbl := range fieldTables {
			cyan.Fprintf(w, "  Table: %s\n", tbl)
			for _, ch := range fieldChanges[tbl] {
				prefix := "    ~"
				switch ch.Type {
				case yamlpkg.ChangeTypeFieldRemoved:
					prefix = "    ✗"
					red.Fprintf(w, "%s %s\n", prefix, ch.Description)
				case yamlpkg.ChangeTypeFieldAdded:
					prefix = "    +"
					yellow.Fprintf(w, "%s %s\n", prefix, ch.Description)
				default:
					fmt.Fprintf(w, "%s %s\n", prefix, ch.Description)
				}
			}
		}
	}

	// ── Summary ───────────────────────────────────────────────────────────
	fmt.Fprintln(w)
	bold.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  Total differences: %d\n", len(diff.Changes))
	fmt.Fprintf(w, "    Missing tables:  %d\n", len(missingTables))
	fmt.Fprintf(w, "    Extra tables:    %d\n", len(extraTables))
	fmt.Fprintf(w, "    Field changes:   %d\n", totalFieldChanges)

	if diff.IsDestructive {
		red.Fprintln(w, "\n⚠ One or more differences are flagged as destructive (data loss risk).")
	}
}

// formatDBDiffJSON writes the diff as JSON to w.
// This output is suitable for scripting or CI pipelines.
func formatDBDiffJSON(w io.Writer, diff *yamlpkg.SchemaDiff) error {
	import "encoding/json" // This import lives at the top of the file; just a reminder

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(diff)
}

func init() {
	rootCmd.AddCommand(dbDiffCmd)

	// Connection flags — bound to the same package-level variables used by
	// db2schema so the user has a consistent interface across commands.
	dbDiffCmd.Flags().StringVar(&host, "host", "", "Database host (default: localhost)")
	dbDiffCmd.Flags().IntVar(&port, "port", 0, "Database port (default: 5432 for PostgreSQL)")
	dbDiffCmd.Flags().StringVar(&database, "database", "", "Database name")
	dbDiffCmd.Flags().StringVar(&username, "username", "", "Database username")
	dbDiffCmd.Flags().StringVar(&password, "password", "", "Database password")
	dbDiffCmd.Flags().StringVar(&sslmode, "sslmode", "", "SSL mode (default: disable)")

	// Database type — bound to the same package-level variable used by dump_sql.
	dbDiffCmd.Flags().StringVar(&databaseType, "db-type", "postgresql",
		"Database type (postgresql, mysql, sqlserver, sqlite)")

	// Output format
	dbDiffCmd.Flags().StringVar(&dbDiffFormat, "format", "text",
		"Output format: text or json")

	// Verbose output — bound to the root-level verbose flag.
	dbDiffCmd.Flags().BoolVar(&verbose, "verbose", false,
		"Show detailed processing information")
}
```

> **Note:** The `import "encoding/json"` comment inside `formatDBDiffJSON` is a reminder — move the real import to the file's top-level import block. The function body is:
> ```go
> func formatDBDiffJSON(w io.Writer, diff *yamlpkg.SchemaDiff) error {
>     enc := json.NewEncoder(w)
>     enc.SetIndent("", "  ")
>     return enc.Encode(diff)
> }
> ```
> And add `"encoding/json"` to the import list at the top.

**Step 2: Run goimports to tidy imports**

```bash
goimports -w cmd/db_diff.go
```

**Step 3: Verify it compiles**

```bash
go build ./cmd/...
```

Expected: no errors

**Step 4: Run tests — registration tests should now pass**

```bash
go test ./cmd/... -run "TestDBDiffCommandRegistered|TestDBDiffCommandHasRequiredFlags" -v
```

Expected: PASS for both

**Step 5: Run lint**

```bash
golangci-lint run --no-config ./cmd/...
```

Fix any reported issues before continuing.

**Step 6: Commit**

```bash
git add cmd/db_diff.go cmd/db_diff_test.go
git commit -m "feat: add db-diff command skeleton with registration and flag setup"
```

---

## Task 4: Make the normalizeDBSchema tests pass

**Files:**
- Modify: `cmd/db_diff.go` (the `normalizeDBSchema` function already written in Task 3)

**Step 1: Run the normalization test**

```bash
go test ./cmd/... -run "TestNormalizeDBSchema" -v
```

Expected: PASS (the function is already implemented in Task 3).

If it fails, check the mapping table in `normalizeDBSchema()` against the expected values in the test and fix any mismatches.

**Step 2: Commit if any changes were needed**

```bash
git add cmd/db_diff.go
git commit -m "fix: correct DB type normalization mappings"
```

---

## Task 5: Make the formatDBDiff tests pass

**Files:**
- Modify: `cmd/db_diff.go` (the `formatDBDiff` function already written in Task 3)

**Step 1: Run the formatter tests**

```bash
go test ./cmd/... -run "TestFormatDBDiff" -v
```

Expected: All PASS.

If tests fail, read the failure message carefully. Common issues:
- `color` package adds ANSI escape codes that break `strings.Contains` checks. If this happens, add `color.NoColor = true` in the test setup or use a `color.New(...).Sprint(...)` pattern that checks `color.NoColor`.

**Fix for color in tests (if needed):**

Add this to `TestMain` in `cmd/db_diff_test.go` or at the start of each test:

```go
func init() {
    // Disable color output during tests so ANSI codes don't break string assertions.
    color.NoColor = true
}
```

**Step 2: Re-run all cmd tests**

```bash
go test ./cmd/... -v 2>&1 | tail -20
```

Expected: all PASS

**Step 3: Commit**

```bash
git add cmd/db_diff.go cmd/db_diff_test.go
git commit -m "fix: disable color in tests for reliable string matching"
```

---

## Task 6: Add integration test with a fake provider and mocked DAG output

This task adds an end-to-end test that exercises `runDBDiff` with a fake in-memory setup, ensuring all three code paths (no migrations, DAG found, DB connected) work together without needing a real database or building a binary.

**Why this is hard:** `runDBDiff` calls `queryDAG()` (which builds a real binary) and `providers.NewProvider()` (which connects to a real database). These cannot be unit-tested easily without refactoring.

**Approach:** Extract the core logic into a testable helper `runDBDiffWithSchemas` that accepts already-loaded schemas. The `runDBDiff` function calls it after loading. This keeps the integration seam clean.

**Files:**
- Modify: `cmd/db_diff.go` — extract helper `runDBDiffWithSchemas`
- Modify: `cmd/db_diff_test.go` — add integration test

**Step 1: Refactor `runDBDiff` to extract the core logic**

Add this function to `cmd/db_diff.go`:

```go
// runDBDiffWithSchemas performs the diff and output given already-loaded schemas.
// This is separated from runDBDiff to make unit testing possible without
// a real database connection or migration binary.
func runDBDiffWithSchemas(w io.Writer, dagSchema, dbSchema *yamlpkg.Schema, format string, verbose bool) error {
	// Normalize DB schema types
	normalizeDBSchema(dbSchema)

	// Compute diff (direction: DAG is "old", DB is "new")
	diffEngine := yamlpkg.NewDiffEngine(verbose)
	diff, err := diffEngine.CompareSchemas(dagSchema, dbSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	if format == "json" {
		return formatDBDiffJSON(w, diff)
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, verbose)
	fmt.Fprint(w, buf.String())

	if diff.HasChanges {
		return fmt.Errorf("schema drift detected: %d difference(s) found", len(diff.Changes))
	}
	return nil
}
```

Update `runDBDiff` to call `runDBDiffWithSchemas` after loading both schemas:

```go
// Replace the "Step 5" block at the bottom of runDBDiff with:
return runDBDiffWithSchemas(cmd.OutOrStdout(), dagSchema, dbSchema, dbDiffFormat, verbose)
```

**Step 2: Write integration tests using `runDBDiffWithSchemas`**

Add to `cmd/db_diff_test.go`:

```go
// TestRunDBDiffWithSchemas_NoDiff verifies that when DAG and DB are identical,
// no error is returned and the output says "No differences".
func TestRunDBDiffWithSchemas_NoDiff(t *testing.T) {
	color.NoColor = true

	schema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}

	// DAG and DB schemas are identical — deep copy by value
	dagSchema := *schema
	dbSchema := *schema

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)
	if err != nil {
		t.Fatalf("expected no error for identical schemas, got: %v", err)
	}
	if !strings.Contains(buf.String(), "No differences") {
		t.Errorf("expected 'No differences' in output, got:\n%s", buf.String())
	}
}

// TestRunDBDiffWithSchemas_MissingTable verifies drift detection when a table
// tracked by the DAG is absent from the live DB.
func TestRunDBDiffWithSchemas_MissingTable(t *testing.T) {
	color.NoColor = true

	dagSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{Name: "users", Fields: []yamlpkg.Field{{Name: "id", Type: "uuid"}}},
			{Name: "audit_log", Fields: []yamlpkg.Field{{Name: "id", Type: "uuid"}}},
		},
	}
	dbSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{Name: "users", Fields: []yamlpkg.Field{{Name: "id", Type: "uuid"}}},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, dagSchema, dbSchema, "text", false)
	if err == nil {
		t.Fatal("expected error indicating schema drift")
	}
	output := buf.String()
	if !strings.Contains(output, "audit_log") {
		t.Errorf("expected 'audit_log' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Missing") {
		t.Errorf("expected 'Missing' section, got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_ExtraTable verifies drift detection when the live DB
// has a table that is not tracked in the DAG.
func TestRunDBDiffWithSchemas_ExtraTable(t *testing.T) {
	color.NoColor = true

	dagSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{Name: "users", Fields: []yamlpkg.Field{{Name: "id", Type: "uuid"}}},
		},
	}
	dbSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{Name: "users", Fields: []yamlpkg.Field{{Name: "id", Type: "uuid"}}},
			{Name: "legacy_cache", Fields: []yamlpkg.Field{{Name: "key", Type: "varchar", Length: 255}}},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, dagSchema, dbSchema, "text", false)
	if err == nil {
		t.Fatal("expected error indicating schema drift")
	}
	output := buf.String()
	if !strings.Contains(output, "legacy_cache") {
		t.Errorf("expected 'legacy_cache' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Extra") {
		t.Errorf("expected 'Extra' section, got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_TypeNormalization verifies that DB-native type names
// are normalized before comparison so they don't produce false-positive diffs.
func TestRunDBDiffWithSchemas_TypeNormalization(t *testing.T) {
	color.NoColor = true

	// DAG uses YAML types; DB uses SQL-native PostgreSQL types
	dagSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "products",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
					{Name: "name", Type: "varchar", Length: 255},
					{Name: "price", Type: "decimal"},
					{Name: "active", Type: "boolean"},
				},
			},
		},
	}
	dbSchema := &yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "products",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
					{Name: "name", Type: "character varying"},
					{Name: "price", Type: "numeric"},
					{Name: "active", Type: "bool"},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, dagSchema, dbSchema, "text", false)
	// After normalization character varying→varchar, numeric→decimal, bool→boolean
	// the two schemas should be equivalent — no diff expected
	if err != nil {
		t.Fatalf("expected no drift after type normalization, got error: %v\nOutput:\n%s", err, buf.String())
	}
}

// TestRunDBDiffWithSchemas_JSONFormat verifies that --format=json produces
// valid JSON output.
func TestRunDBDiffWithSchemas_JSONFormat(t *testing.T) {
	import "encoding/json" // reminder: real import goes at top of file

	dagSchema := &yamlpkg.Schema{}
	dbSchema := &yamlpkg.Schema{}

	var buf bytes.Buffer
	if err := runDBDiffWithSchemas(&buf, dagSchema, dbSchema, "json", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded yamlpkg.SchemaDiff
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput:\n%s", err, buf.String())
	}
}
```

> **Note:** Move `"encoding/json"` to the real import block at the top of `db_diff_test.go`.

**Step 3: Run the integration tests**

```bash
go test ./cmd/... -run "TestRunDBDiffWithSchemas" -v
```

Expected: all PASS

**Step 4: Run the full test suite**

```bash
go test ./...
```

Expected: all PASS

**Step 5: Run lint**

```bash
golangci-lint run --no-config ./...
```

Fix any issues.

**Step 6: Commit**

```bash
git add cmd/db_diff.go cmd/db_diff_test.go
git commit -m "test: add integration tests for runDBDiffWithSchemas"
```

---

## Task 7: Manual smoke test (optional, requires real database)

This task is optional and manual. It verifies the command works end-to-end with a real PostgreSQL or SQLite database.

**Step 1: Build the tool**

```bash
go build -o /tmp/makemigrations .
```

**Step 2: Run against a local database**

```bash
/tmp/makemigrations db-diff \
  --host=localhost \
  --port=5432 \
  --database=myapp \
  --username=postgres \
  --db-type=postgresql \
  --verbose
```

**Step 3: Check JSON output**

```bash
/tmp/makemigrations db-diff \
  --host=localhost --database=myapp --username=postgres \
  --db-type=postgresql --format=json | jq .
```

Expected: valid JSON with `has_changes`, `changes`, and `is_destructive` keys.

---

## Task 8: Write documentation

**Files:**
- Create: `docs/db-diff.md`

**Step 1: Write the documentation file**

````markdown
# `db-diff` Command

Compares the live database schema against the schema expected by the migration
DAG and reports any differences.

## Usage

```
makemigrations db-diff [flags]
```

## What It Does

1. Reads the migration DAG from compiled migration files in the `migrations/`
   directory to determine the "expected" schema state.
2. Connects to the live database and reads the actual schema.
3. Normalizes SQL-native type names (e.g. `character varying` → `varchar`) so
   comparisons are meaningful.
4. Diffs the two schemas and reports:
   - **Missing from DB** — Tables/columns the migrations expect but that don't
     exist yet in the live database (pending migrations not applied).
   - **Extra in DB** — Tables/columns in the live database not tracked by any
     migration (manual schema changes or legacy objects).
   - **Field differences** — Columns that exist in both but have different type,
     nullability, default, or other attribute values.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--host` | `localhost` | Database host |
| `--port` | `5432` | Database port |
| `--database` | — | Database name |
| `--username` | — | Database username |
| `--password` | — | Database password |
| `--sslmode` | `disable` | SSL mode |
| `--db-type` | `postgresql` | Database type: `postgresql`, `mysql`, `sqlserver`, `sqlite` |
| `--format` | `text` | Output format: `text` or `json` |
| `--verbose` | `false` | Show detailed progress information |
| `--config` | — | Path to `makemigrations.config.yaml` |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No differences found — live DB matches the migration DAG. |
| `1` | Differences found, or an error occurred. |

Use exit code `1` in CI pipelines to fail a job when schema drift is detected.

## Examples

```bash
# Check for drift against a local PostgreSQL database
makemigrations db-diff \
  --host=localhost \
  --port=5432 \
  --database=myapp \
  --username=postgres

# Use config file for connection settings
makemigrations db-diff --config=migrations/makemigrations.config.yaml

# JSON output for scripting
makemigrations db-diff --format=json | jq .changes

# CI usage — fail on any schema drift
makemigrations db-diff --host=$DB_HOST --database=$DB_NAME \
  --username=$DB_USER --password=$DB_PASS || exit 1
```

## Notes on Type Comparison

The DAG schema uses YAML-normalized type names (e.g. `varchar`, `integer`,
`timestamp`). The live database may report SQL-native names (e.g.
`character varying`, `int4`, `timestamp without time zone`). The `db-diff`
command normalizes these before comparing, so minor naming differences do not
appear as false-positive drift.

If you see unexpected field-type differences, check whether the DB type has a
known mapping in `cmd/db_diff.go:normalizeDBSchema()` and add it if needed.
````

**Step 2: Commit**

```bash
git add docs/db-diff.md
git commit -m "docs: add db-diff command documentation"
```

---

## Task 9: Final checks and branch ready for review

**Step 1: Run the full test suite one last time**

```bash
go test ./...
```

Expected: all PASS

**Step 2: Run lint**

```bash
golangci-lint run --no-config ./...
```

Expected: no new issues (pre-existing issues in `db2schema.go`, `goose.go`, `schema2diagram.go` are acceptable — do not fix them).

**Step 3: Build the final binary**

```bash
go build ./...
```

Expected: success

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: db-diff command — compare live DB schema against migration DAG"
```

---

## Implementation Checklist

- [ ] Worktree created at `.worktrees/db-diff` on branch `feature/db-diff`
- [ ] `cmd/db_diff_test.go` — failing tests written first
- [ ] `cmd/db_diff.go` — command registered and flags declared
- [ ] `normalizeDBSchema()` — SQL-native types mapped to YAML types
- [ ] `formatDBDiff()` — human-readable color-coded output
- [ ] `formatDBDiffJSON()` — JSON output for scripting
- [ ] `runDBDiffWithSchemas()` — extracted for testability
- [ ] All `cmd/...` tests pass
- [ ] `golangci-lint run --no-config ./...` — no new issues
- [ ] `docs/db-diff.md` — command documentation written
- [ ] Final build succeeds
