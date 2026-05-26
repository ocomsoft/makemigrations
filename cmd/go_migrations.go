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

// Package cmd contains all CLI commands for makemigrations.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/interp"
	"github.com/ocomsoft/makemigrations/internal/types"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
	"github.com/ocomsoft/makemigrations/migrate"
)

// Flag variables for the go_migrations command. These are prefixed with goMig
// to avoid conflicts with the root command flag variables.
var (
	goMigDryRun  bool
	goMigCheck   bool
	goMigMerge   bool
	goMigName    string
	goMigVerbose bool
)

// goMigrationsCmd is the Cobra command for generating Go migration files from
// YAML schema changes. It compares the current YAML schema against the
// reconstructed state from existing Go migration files and generates a new
// migration .go file for any changes detected.
var goMigrationsCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Generate Go migration files from YAML schema changes",
	Long: `Compares the current YAML schema against the reconstructed state from existing
Go migration files and generates a new migration .go file for any changes detected.

This command implements the Django-style migration workflow:
  1. Builds the existing migrations binary (if any .go files exist)
  2. Queries the DAG to reconstruct the current schema state
  3. Parses the current YAML schema files
  4. Diffs the two schemas
  5. Generates a new .go migration file with typed operations

Use --merge to generate a merge migration when concurrent branches are detected.
Use --check in CI/CD to fail if unapplied schema changes exist.`,
	RunE: runGoMakeMigrations,
}

func init() {
	rootCmd.AddCommand(goMigrationsCmd)
	goMigrationsCmd.Flags().BoolVar(&goMigDryRun, "dry-run", false,
		"Print generated migration without writing")
	goMigrationsCmd.Flags().BoolVar(&goMigCheck, "check", false,
		"Exit with error if migrations are needed (for CI/CD)")
	goMigrationsCmd.Flags().BoolVar(&goMigMerge, "merge", false,
		"Generate merge migration for detected branches")
	goMigrationsCmd.Flags().StringVar(&goMigName, "name", "",
		"Custom migration name suffix")
	goMigrationsCmd.Flags().BoolVar(&goMigVerbose, "verbose", false,
		"Show detailed output")
}

// runGoMakeMigrations is the main entry point for Go migration generation.
// It orchestrates the build-query-diff-generate pipeline:
//  1. Scan for existing .go migration files in the migrations directory
//  2. If migrations exist, compile them and query the DAG for the current schema state
//  3. Parse and merge the current YAML schema files
//  4. Diff the reconstructed state against the current schema
//  5. Generate a new .go migration file (or merge migration if --merge is set)
func runGoMakeMigrations(_ *cobra.Command, _ []string) error {
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory
	gen := codegen.NewGoGenerator()

	// 1. Query existing DAG (if any .go files exist)
	var dagOut *migrate.DAGOutput
	var prevSchema *yamlpkg.Schema

	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	// Filter to migration files only (exclude main.go)
	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	if len(migFiles) > 0 {
		dagOut, err = queryDAG(migrationsDir, goMigVerbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		prevSchema = schemaStateToYAMLSchema(dagOut.SchemaState, cfg.Database.Type)
	}

	// 2. Parse current YAML schema
	dbType, err := yamlpkg.ParseDatabaseType(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}
	components := InitializeYAMLComponents(dbType, goMigVerbose)
	allSchemas, err := ScanAndParseSchemas(components, goMigVerbose)
	if err != nil {
		return fmt.Errorf("parsing YAML schema: %w", err)
	}

	// Merge parsed schemas into a single schema for diffing
	currentSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, goMigVerbose)
	if err != nil {
		return fmt.Errorf("merging YAML schemas: %w", err)
	}

	// 3. Diff
	diffEngine := yamlpkg.NewDiffEngine(goMigVerbose)
	diff, err := diffEngine.CompareSchemas(prevSchema, currentSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	// Prepend a SetDefaults operation when the active DB defaults have changed.
	prevDefaults := getDefaultsForDB(prevSchema, cfg.Database.Type)
	currDefaults := getDefaultsForDB(currentSchema, cfg.Database.Type)
	if !mapsEqual(prevDefaults, currDefaults) && len(currDefaults) > 0 {
		diff.Changes = append([]yamlpkg.Change{{
			Type:        yamlpkg.ChangeTypeDefaultsModified,
			Description: "Update schema defaults",
			OldValue:    prevDefaults,
			NewValue:    currDefaults,
		}}, diff.Changes...)
		diff.HasChanges = true
	}

	// Prepend a SetTypeMappings operation when the active DB type mappings have changed.
	prevMappings := getTypeMappingsForDB(prevSchema, cfg.Database.Type)
	currMappings := getTypeMappingsForDB(currentSchema, cfg.Database.Type)
	if !mapsEqual(prevMappings, currMappings) && len(currMappings) > 0 {
		diff.Changes = append([]yamlpkg.Change{{
			Type:        yamlpkg.ChangeTypeTypeMappingsModified,
			Description: "Update schema type mappings",
			OldValue:    prevMappings,
			NewValue:    currMappings,
		}}, diff.Changes...)
		diff.HasChanges = true
	}

	// 4. Handle merge if requested
	if goMigMerge && dagOut != nil && dagOut.HasUnresolvedBranches {
		return goGenerateMerge(migrationsDir, dagOut, goMigDryRun, goMigVerbose)
	}

	// 5. Check for unresolved branches (warn if present and not doing merge).
	// HasUnresolvedBranches is true only when there are multiple leaf migrations —
	// resolved diamond topologies (already merged) do not trigger this warning.
	if dagOut != nil && dagOut.HasUnresolvedBranches && !goMigMerge {
		fmt.Println("WARNING: Multiple migration branches detected — merge required.")
		for i, leaf := range dagOut.Leaves {
			fmt.Printf("  Branch %d: %s\n", i+1, leaf)
		}
		fmt.Println("Run 'makemigrations --merge' to generate a merge migration.")
	}

	if !diff.HasChanges {
		fmt.Println("No changes detected.")
		return nil
	}

	if goMigCheck {
		printChangeList(diff.Changes)
		return fmt.Errorf("migrations needed: %d changes detected", len(diff.Changes))
	}

	// 6. Determine next migration name
	deps := []string{}
	if dagOut != nil {
		deps = dagOut.Leaves
	}
	count := len(migFiles)
	name := BuildMigrationName(count, goMigName, diffEngine.GenerateMigrationName(diff))

	// 7. Prompt for destructive operations and build per-change decisions.
	decisions, err := promptGoMigDecisions(diff)
	if err != nil {
		return err // includes user-requested exit
	}

	// 8. Generate Go source
	src, err := gen.GenerateMigration(name, deps, diff, currentSchema, prevSchema, decisions)
	if err != nil {
		return fmt.Errorf("generating migration source: %w", err)
	}

	if goMigDryRun {
		fmt.Println(src)
		return nil
	}

	// 9. Write file
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}
	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0o644); err != nil {
		return fmt.Errorf("writing migration file: %w", err)
	}
	fmt.Printf("Created %s\n", outPath)
	return nil
}

// printChangeList prints a human-readable summary of schema changes grouped by type.
func printChangeList(changes []yamlpkg.Change) {
	type entry struct {
		table string
		field string
		desc  string
	}
	groups := make(map[yamlpkg.ChangeType][]entry)
	for _, c := range changes {
		groups[c.Type] = append(groups[c.Type], entry{c.TableName, c.FieldName, c.Description})
	}

	labels := []struct {
		ct    yamlpkg.ChangeType
		label string
	}{
		{yamlpkg.ChangeTypeTableAdded, "Tables added"},
		{yamlpkg.ChangeTypeTableRemoved, "Tables removed"},
		{yamlpkg.ChangeTypeTableRenamed, "Tables renamed"},
		{yamlpkg.ChangeTypeFieldAdded, "Fields added"},
		{yamlpkg.ChangeTypeFieldRemoved, "Fields removed"},
		{yamlpkg.ChangeTypeFieldRenamed, "Fields renamed"},
		{yamlpkg.ChangeTypeFieldModified, "Fields modified"},
		{yamlpkg.ChangeTypeIndexAdded, "Indexes added"},
		{yamlpkg.ChangeTypeIndexRemoved, "Indexes removed"},
		{yamlpkg.ChangeTypeForeignKeyAdded, "Foreign keys added"},
		{yamlpkg.ChangeTypeForeignKeyRemoved, "Foreign keys removed"},
		{yamlpkg.ChangeTypeDefaultsModified, "Defaults modified"},
		{yamlpkg.ChangeTypeTypeMappingsModified, "Type mappings modified"},
	}

	fmt.Println()
	for _, l := range labels {
		entries, ok := groups[l.ct]
		if !ok {
			continue
		}
		fmt.Printf("  %s (%d):\n", l.label, len(entries))
		for _, e := range entries {
			if e.field != "" {
				fmt.Printf("    - %s.%s\n", e.table, e.field)
			} else {
				fmt.Printf("    - %s\n", e.table)
			}
		}
	}
	fmt.Println()
}

// promptGoMigDecisions iterates through diff.Changes and, for each destructive
// operation, interactively asks the user what to do. The returned map is keyed
// by change index and holds the user's PromptResponse for that change.
//
// If the user chooses PromptOmit the generated operation will have SchemaOnly: true
// (schema state advances but no SQL is executed). If the user chooses PromptExit
// an error is returned and migration generation is cancelled.
func promptGoMigDecisions(diff *yamlpkg.SchemaDiff) (map[int]yamlpkg.PromptResponse, error) {
	decisions := make(map[int]yamlpkg.PromptResponse)
	generateAll := false

	for i, change := range diff.Changes {
		if !yamlpkg.IsDestructiveOperation(change.Type) {
			continue
		}
		if generateAll {
			decisions[i] = yamlpkg.PromptGenerate
			continue
		}

		if change.FieldName != "" {
			fmt.Printf("\n⚠  Destructive operation detected: %s on %q (field: %q)\n", change.Type, change.TableName, change.FieldName)
		} else {
			fmt.Printf("\n⚠  Destructive operation detected: %s on %q\n", change.Type, change.TableName)
		}
		fmt.Println("  1) Generate      — include operation in migration")
		fmt.Println("  2) Review        — include with // REVIEW comment")
		fmt.Println("  3) Omit          — skip operation; schema state still advances (SchemaOnly)")
		fmt.Println("  4) Exit          — cancel migration generation")
		fmt.Println("  5) All           — generate all remaining destructive ops without prompting")
		fmt.Println("  6) IgnoreErrors  — include with IgnoreErrors: true (continue on failure)")
		fmt.Print("Choice [1-6]: ")

		var input string
		if _, err := fmt.Scanln(&input); err != nil {
			return nil, fmt.Errorf("reading input: %w", err)
		}

		switch strings.TrimSpace(input) {
		case "1":
			decisions[i] = yamlpkg.PromptGenerate
		case "2":
			decisions[i] = yamlpkg.PromptReview
		case "3":
			decisions[i] = yamlpkg.PromptOmit
		case "4":
			return nil, fmt.Errorf("migration generation cancelled by user")
		case "5":
			decisions[i] = yamlpkg.PromptGenerate
			generateAll = true
		case "6":
			decisions[i] = yamlpkg.PromptIgnoreErrors
		default:
			decisions[i] = yamlpkg.PromptGenerate
		}
	}
	return decisions, nil
}

// queryDAG loads the migrations directory with the yaegi interpreter and
// returns the current migration graph state. No Go toolchain is invoked; the
// migration .go files are interpreted in-process and registered with a fresh
// *migrate.Registry, then BuildGraph + ToDAGOutput run directly.
//
// The verbose parameter is preserved for API compatibility with callers but
// has no effect now that no build step takes place.
func queryDAG(migrationsDir string, _ bool) (*migrate.DAGOutput, error) {
	reg, err := interp.LoadRegistry(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("loading migrations: %w", err)
	}
	g, err := migrate.BuildGraph(reg)
	if err != nil {
		return nil, fmt.Errorf("building migration graph: %w", err)
	}
	return g.ToDAGOutput()
}

// schemaStateToYAMLSchema converts a migrate.SchemaState (reconstructed from
// the migration DAG) into a yaml.Schema suitable for diffing against the
// current YAML schema. Tables are sorted by name for deterministic output.
// dbType is used to populate the Defaults section of the schema so that
// defaults changes are detected on subsequent diff runs.
func schemaStateToYAMLSchema(state *migrate.SchemaState, dbType string) *yamlpkg.Schema {
	if state == nil {
		return nil
	}
	schema := &yamlpkg.Schema{}
	for _, ts := range state.Tables {
		t := yamlpkg.Table{Name: ts.Name}
		for _, f := range ts.Fields {
			nullable := f.Nullable
			yf := yamlpkg.Field{
				Name:       f.Name,
				Type:       f.Type,
				PrimaryKey: f.PrimaryKey,
				Nullable:   &nullable,
				Default:    f.Default,
				Length:     f.Length,
				Precision:  f.Precision,
				Scale:      f.Scale,
				AutoCreate: f.AutoCreate,
				AutoUpdate: f.AutoUpdate,
			}
			if f.ForeignKey != nil {
				// Only include the FK annotation when the constraint actually exists in
				// the migration state. If it is absent (e.g. tables created before
				// AddForeignKey support was introduced), leave ForeignKey nil so the
				// diff engine detects the missing constraint and emits
				// ChangeTypeForeignKeyAdded.
				constraintName := fmt.Sprintf("fk_%s_%s", ts.Name, f.Name)
				for _, fkc := range ts.ForeignKeys {
					if fkc.Name == constraintName {
						yf.ForeignKey = &yamlpkg.ForeignKey{
							Table:    fkc.ReferencedTable,
							OnDelete: fkc.OnDelete,
						}
						break
					}
				}
			}
			t.Fields = append(t.Fields, yf)
		}
		for _, idx := range ts.Indexes {
			if idx.FromFK {
				continue
			}
			t.Indexes = append(t.Indexes, yamlpkg.Index{
				Name:   idx.Name,
				Fields: idx.Fields,
				Unique: idx.Unique,
				Method: idx.Method,
				Where:  idx.Where,
			})
		}
		schema.Tables = append(schema.Tables, t)
	}
	// Sort tables for determinism
	sort.Slice(schema.Tables, func(i, j int) bool {
		return schema.Tables[i].Name < schema.Tables[j].Name
	})
	// Populate the Defaults section so that defaults changes are detected on
	// subsequent diff runs (the diff engine compares schema.Defaults).
	if len(state.Defaults) > 0 {
		switch dbType {
		case "postgresql":
			schema.Defaults.PostgreSQL = state.Defaults
		case "mysql":
			schema.Defaults.MySQL = state.Defaults
		case "sqlserver":
			schema.Defaults.SQLServer = state.Defaults
		case "sqlite":
			schema.Defaults.SQLite = state.Defaults
		}
	}
	// Populate TypeMappings so that type mapping changes are detected on subsequent diff runs.
	if len(state.TypeMappings) > 0 {
		switch dbType {
		case "postgresql":
			schema.TypeMappings.PostgreSQL = state.TypeMappings
		case "mysql":
			schema.TypeMappings.MySQL = state.TypeMappings
		case "sqlserver":
			schema.TypeMappings.SQLServer = state.TypeMappings
		case "sqlite":
			schema.TypeMappings.SQLite = state.TypeMappings
		case "redshift":
			schema.TypeMappings.Redshift = state.TypeMappings
		case "clickhouse":
			schema.TypeMappings.ClickHouse = state.TypeMappings
		case "tidb":
			schema.TypeMappings.TiDB = state.TypeMappings
		case "vertica":
			schema.TypeMappings.Vertica = state.TypeMappings
		case "ydb":
			schema.TypeMappings.YDB = state.TypeMappings
		case "turso":
			schema.TypeMappings.Turso = state.TypeMappings
		case "starrocks":
			schema.TypeMappings.StarRocks = state.TypeMappings
		case "auroradsql":
			schema.TypeMappings.AuroraDSQL = state.TypeMappings
		}
	}
	return schema
}

// getDefaultsForDB returns the defaults map for the given database type from a schema.
// Returns nil when the schema or the relevant DB defaults are empty.
func getDefaultsForDB(schema *yamlpkg.Schema, dbType string) map[string]string {
	if schema == nil {
		return nil
	}
	switch dbType {
	case "postgresql":
		return schema.Defaults.PostgreSQL
	case "mysql":
		return schema.Defaults.MySQL
	case "sqlserver":
		return schema.Defaults.SQLServer
	case "sqlite":
		return schema.Defaults.SQLite
	default:
		return nil
	}
}

// getTypeMappingsForDB returns the type mappings for the given database type from a schema.
// Returns nil when the schema or the relevant DB type mappings are empty.
func getTypeMappingsForDB(schema *yamlpkg.Schema, dbType string) map[string]string {
	if schema == nil {
		return nil
	}
	return schema.TypeMappings.ForProvider(types.DatabaseType(dbType))
}

// mapsEqual reports whether two map[string]string values are identical.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// BuildMigrationName builds the migration name from a sequence number and an
// optional custom name. If customName is provided, it is normalized (lowered,
// spaces replaced with underscores). Otherwise autoName (from the diff engine)
// is used. As a final fallback a timestamp is appended.
// Exported for testing.
func BuildMigrationName(currentCount int, customName, autoName string) string {
	num := codegen.NextMigrationNumber(currentCount)
	if customName != "" {
		return fmt.Sprintf("%s_%s", num, strings.ToLower(strings.ReplaceAll(customName, " ", "_")))
	}
	if autoName != "" {
		return fmt.Sprintf("%s_%s", num, autoName)
	}
	return fmt.Sprintf("%s_%s", num, time.Now().Format("20060102150405"))
}

// goGenerateMerge generates a merge migration for detected branches. It uses
// codegen.MergeGenerator to produce a .go file that depends on all branch
// leaves but contains no operations, thus unifying the DAG.
func goGenerateMerge(migrationsDir string, dagOut *migrate.DAGOutput, dryRun, verbose bool) error {
	// Count existing migration files
	count := 0
	goFiles, _ := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			count++
		}
	}

	name := fmt.Sprintf("%s_merge_%s", codegen.NextMigrationNumber(count),
		strings.Join(dagOut.Leaves, "_and_"))
	// Truncate if too long
	if len(name) > 80 {
		name = fmt.Sprintf("%s_merge", codegen.NextMigrationNumber(count))
	}

	mergeGen := codegen.NewMergeGenerator()
	src, err := mergeGen.GenerateMerge(name, dagOut.Leaves)
	if err != nil {
		return fmt.Errorf("generating merge migration: %w", err)
	}

	if dryRun {
		fmt.Println(src)
		return nil
	}

	if verbose {
		fmt.Printf("Generating merge migration: %s\n", name)
	}

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}
	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0o644); err != nil {
		return fmt.Errorf("writing merge migration: %w", err)
	}
	fmt.Printf("Created merge migration: %s\n", outPath)
	fmt.Printf("Dependencies: %s\n", strings.Join(dagOut.Leaves, ", "))
	return nil
}

// findParentGoVersion walks up from startDir looking for a go.work or go.mod.
// It returns the most specific Go version found, preferring the toolchain
// directive (e.g. "1.25.7") over the go directive (e.g. "1.25"), because the
// toolchain directive has the full patch version that is already cached locally.
// Returns "" if nothing is found.
func findParentGoVersion(startDir string) string {
	dir := startDir
	for {
		workPath := filepath.Join(dir, "go.work")
		if data, err := os.ReadFile(workPath); err == nil {
			if f, err := modfile.ParseWork(workPath, data, nil); err == nil {
				if f.Toolchain != nil && f.Toolchain.Name != "" {
					return strings.TrimPrefix(f.Toolchain.Name, "go")
				}
				if f.Go != nil {
					return f.Go.Version
				}
			}
		}
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			if f, err := modfile.Parse(modPath, data, nil); err == nil {
				if f.Toolchain != nil && f.Toolchain.Name != "" {
					return strings.TrimPrefix(f.Toolchain.Name, "go")
				}
				if f.Go != nil {
					return f.Go.Version
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
