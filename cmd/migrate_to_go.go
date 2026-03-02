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
package cmd

import (
	"database/sql"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/gooseparser"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
	"github.com/ocomsoft/makemigrations/migrate"
	"github.com/spf13/cobra"
)

// migrateEntry holds the mapping between a legacy SQL file and the new Go migration file.
type migrateEntry struct {
	sqlFile   string
	goName    string
	versionID int64 // used by history migration to match against goose_db_version.version_id
}

// Package-level flag variables for the migrate-to-go command.
var (
	migrateToGoDryRun    bool
	migrateToGoNoHistory bool
	migrateToGoNoDelete  bool
	migrateToGoForce     bool
)

// migrateToGoCmd converts legacy Goose SQL migration files to Go RunSQL migration files.
var migrateToGoCmd = &cobra.Command{
	Use:   "migrate-to-go",
	Short: "Convert Goose SQL migrations to Go RunSQL migration files",
	Long: `Converts all .sql Goose migration files in the migrations directory
into Go source files that use the RunSQL operation. Optionally migrates
the goose_db_version history to the new migration_record table.

By default, the original .sql files are deleted after successful conversion.
Use --no-delete to keep them. Use --dry-run to preview changes without writing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault(configFile)
		return ExecuteMigrateToGo(
			cfg.Migration.Directory,
			migrateToGoDryRun,
			migrateToGoNoHistory,
			migrateToGoNoDelete,
			cmd.OutOrStdout(),
		)
	},
}

func init() {
	rootCmd.AddCommand(migrateToGoCmd)

	migrateToGoCmd.Flags().BoolVar(&migrateToGoDryRun, "dry-run", false, "Preview changes without writing files")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoHistory, "no-history", false, "Skip migrating goose_db_version history")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoDelete, "no-delete", false, "Keep original .sql files after conversion")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoForce, "force", false, "Overwrite existing .go migration files")
}

// ExecuteMigrateToGo is the main entry point for the migrate-to-go conversion.
// It discovers SQL files in migrationsDir, parses them with gooseparser, generates
// Go RunSQL migration files, and optionally cleans up the SQL originals.
func ExecuteMigrateToGo(migrationsDir string, dryRun, noHistory, noDelete bool, out io.Writer) error {
	// Step 1: Discover SQL files.
	sqlFiles, err := discoverSQLFiles(migrationsDir)
	if err != nil {
		return err
	}
	if len(sqlFiles) == 0 {
		return fmt.Errorf("no .sql migration files found in %s", migrationsDir)
	}

	// Step 2: Check for existing Go migration files (unless --force).
	if !migrateToGoForce {
		if err := checkNoExistingGoMigrations(migrationsDir); err != nil {
			return err
		}
	}

	// Step 3: Build migrateEntry slice from the discovered SQL files.
	entries := make([]migrateEntry, 0, len(sqlFiles))
	for i, sqlFile := range sqlFiles {
		desc := gooseparser.ExtractDescription(sqlFile)
		goName := fmt.Sprintf("%04d_%s", i+1, desc)

		versionID, verErr := gooseparser.ExtractVersionID(sqlFile)
		if verErr != nil {
			return fmt.Errorf("extracting version from %s: %w", sqlFile, verErr)
		}

		entries = append(entries, migrateEntry{
			sqlFile:   sqlFile,
			goName:    goName,
			versionID: versionID,
		})
	}

	// Step 4: Parse each SQL file and generate Go migration source.
	for i, entry := range entries {
		mig, parseErr := gooseparser.ParseFile(filepath.Join(migrationsDir, entry.sqlFile))
		if parseErr != nil {
			return fmt.Errorf("parsing %s: %w", entry.sqlFile, parseErr)
		}

		// Build dependency list: first migration has none, subsequent depend on the previous.
		var deps []string
		if i > 0 {
			deps = []string{entries[i-1].goName}
		}

		src, genErr := generateRunSQLMigration(entry.goName, deps, mig.ForwardSQL, mig.BackwardSQL)
		if genErr != nil {
			return fmt.Errorf("generating Go source for %s: %w", entry.sqlFile, genErr)
		}

		if dryRun {
			fmt.Fprintf(out, "Would create %s.go:\n%s\n", entry.goName, src)
			continue
		}

		goPath := filepath.Join(migrationsDir, entry.goName+".go")
		if writeErr := os.WriteFile(goPath, []byte(src), 0o644); writeErr != nil {
			return fmt.Errorf("writing %s: %w", goPath, writeErr)
		}
		fmt.Fprintf(out, "%s Created %s\n", green("✓"), goPath)
	}

	// Step 5: Generate main.go and go.mod if not present (skip in dry-run).
	if !dryRun {
		gen := codegen.NewGoGenerator()

		mainPath := filepath.Join(migrationsDir, "main.go")
		if _, statErr := os.Stat(mainPath); os.IsNotExist(statErr) {
			if writeErr := os.WriteFile(mainPath, []byte(gen.GenerateMainGo()), 0o644); writeErr != nil {
				return fmt.Errorf("writing main.go: %w", writeErr)
			}
			fmt.Fprintf(out, "%s Created %s\n", green("✓"), mainPath)
		}

		goModPath := filepath.Join(migrationsDir, "go.mod")
		if _, statErr := os.Stat(goModPath); os.IsNotExist(statErr) {
			moduleName := readModuleName() + "/migrations"
			version := "v0.3.0"
			if writeErr := os.WriteFile(goModPath, []byte(gen.GenerateGoMod(moduleName, version, findParentGoVersion("."), findLocalMakemigrations("."))), 0o644); writeErr != nil {
				return fmt.Errorf("writing go.mod: %w", writeErr)
			}
			fmt.Fprintf(out, "%s Created %s\n", green("✓"), goModPath)
		}
	}

	// Step 6: Generate a schema-state bootstrap migration.
	//
	// Why this exists:
	//   RunSQL.Mutate() is a no-op — raw SQL cannot be reliably parsed back into a
	//   typed schema representation. This means that after migrate-to-go, when
	//   `makemigrations makemigrations` queries the DAG to reconstruct the previous
	//   schema state, it gets an empty SchemaState and believes no tables exist yet.
	//   The next diff would then generate a "create everything" migration, corrupting
	//   the migration chain.
	//
	//   The bootstrap migration is a typed Go migration (CreateTable + AddField ops)
	//   placed at the end of the chain with SchemaOnly: true on every operation.
	//   When the runner executes it:
	//     • Up()/Down() return "" — no SQL is sent to the database (tables already exist)
	//     • Mutate() runs normally — the SchemaState is populated correctly
	//   Subsequent calls to `makemigrations makemigrations` then see the correct
	//   previous schema and produce accurate diffs.
	if !dryRun {
		if bootstrapErr := generateSchemaStateBootstrap(migrationsDir, entries, out); bootstrapErr != nil {
			return bootstrapErr
		}
	}


	// Step 7 (was 6): Migrate goose history.
	if !noHistory && !dryRun {
		cfg := config.LoadOrDefault(configFile)
		db, dbErr := setupGooseDB(cfg)
		if dbErr != nil {
			return fmt.Errorf("connecting to database for history migration: %w (use --no-history to skip)", dbErr)
		}
		defer func() { _ = db.Close() }()
		if histErr := migrateHistoryWithEntries(db, entries, dryRun, out); histErr != nil {
			return fmt.Errorf("migrating history: %w", histErr)
		}
	}

	// Step 8: Delete original SQL files.
	if !noDelete && !dryRun {
		for _, entry := range entries {
			sqlPath := filepath.Join(migrationsDir, entry.sqlFile)
			if rmErr := os.Remove(sqlPath); rmErr != nil {
				return fmt.Errorf("deleting %s: %w", sqlPath, rmErr)
			}
			fmt.Fprintf(out, "%s Deleted %s\n", cyan("✗"), sqlPath)
		}
	}

	return nil
}

// generateSchemaStateBootstrap generates a schema-state migration at the end of
// the converted RunSQL chain. It loads the current YAML schema (or snapshot) and
// produces a typed Go migration where every CreateTable/AddField operation has
// SchemaOnly: true. See the inline comment in ExecuteMigrateToGo for a full
// explanation of why this migration is required.
func generateSchemaStateBootstrap(migrationsDir string, entries []migrateEntry, out io.Writer) error {
	schema, schemaErr := loadSchemaForBootstrap(migrationsDir)
	if schemaErr != nil {
		return fmt.Errorf(
			"cannot generate schema-state bootstrap migration: %w\n\n"+
				"Run 'makemigrations db2schema' to extract your database schema into YAML files,\n"+
				"then re-run migrate-to-go.",
			schemaErr,
		)
	}

	diff := schemaToInitialDiff(schema)
	if !diff.HasChanges {
		fmt.Fprintf(out, "⚠  No tables found in schema — skipping schema-state bootstrap migration\n")
		return nil
	}

	// All changes are SchemaOnly: don't execute SQL; only populate schema state.
	decisions := make(map[int]yamlpkg.PromptResponse, len(diff.Changes))
	for i := range diff.Changes {
		decisions[i] = yamlpkg.PromptOmit
	}

	// Sequential number immediately after the last RunSQL migration.
	bootstrapNum := len(entries) + 1
	bootstrapName := fmt.Sprintf("%04d_schema_state", bootstrapNum)

	var deps []string
	if len(entries) > 0 {
		deps = []string{entries[len(entries)-1].goName}
	}

	gen := codegen.NewGoGenerator()
	src, genErr := gen.GenerateMigration(bootstrapName, deps, diff, schema, nil, decisions)
	if genErr != nil {
		return fmt.Errorf("generating schema-state bootstrap migration: %w", genErr)
	}

	goPath := filepath.Join(migrationsDir, bootstrapName+".go")
	if writeErr := os.WriteFile(goPath, []byte(src), 0o644); writeErr != nil {
		return fmt.Errorf("writing schema-state bootstrap migration: %w", writeErr)
	}
	fmt.Fprintf(out, "%s Created %s (schema-state bootstrap, SchemaOnly)\n", green("✓"), goPath)
	return nil
}

// loadSchemaForBootstrap tries to locate the current schema for the bootstrap migration.
// It first attempts to scan and merge YAML schema files from the project; if none are
// found it falls back to loading the .schema_snapshot.yaml from migrationsDir.
// Returns an error if neither source is available.
func loadSchemaForBootstrap(migrationsDir string) (*yamlpkg.Schema, error) {
	cfg := config.LoadOrDefault(configFile)

	dbType, err := yamlpkg.ParseDatabaseType(cfg.Database.Type)
	if err != nil {
		// Non-fatal: fall through to snapshot
		dbType = yamlpkg.DatabasePostgreSQL
	}

	components := InitializeYAMLComponents(dbType, false, true)
	allSchemas, scanErr := ScanAndParseSchemas(components, false)
	if scanErr == nil && len(allSchemas) > 0 {
		merged, mergeErr := MergeAndValidateSchemas(components, allSchemas, dbType, false)
		if mergeErr == nil && merged != nil && len(merged.Tables) > 0 {
			return merged, nil
		}
	}

	// Fall back to .schema_snapshot.yaml in migrationsDir.
	sm := yamlpkg.NewStateManager(false)
	snapshot, snapErr := sm.LoadSchemaSnapshot(migrationsDir)
	if snapErr == nil && snapshot != nil && len(snapshot.Tables) > 0 {
		return snapshot, nil
	}

	return nil, fmt.Errorf("no YAML schema files or .schema_snapshot.yaml found")
}

// discoverSQLFiles returns the names of all .sql files in dir, sorted alphabetically.
func discoverSQLFiles(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*.sql")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", pattern, err)
	}

	sort.Strings(matches)

	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	return names, nil
}

// checkNoExistingGoMigrations returns an error if any .go files (other than main.go)
// exist in dir. This prevents accidental overwrites of previously converted migrations.
func checkNoExistingGoMigrations(dir string) error {
	pattern := filepath.Join(dir, "*.go")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}

	for _, m := range matches {
		base := filepath.Base(m)
		if base == "main.go" {
			continue
		}
		return fmt.Errorf("existing Go migration file found: %s (use --force to overwrite)", base)
	}
	return nil
}

// generateRunSQLMigration produces a gofmt'd Go source file that registers a RunSQL
// migration with the given name, dependencies, and SQL content.
func generateRunSQLMigration(name string, deps []string, forwardSQL, backwardSQL string) (string, error) {
	var b strings.Builder

	b.WriteString("package main\n\n")
	b.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	b.WriteString("func init() {\n")
	b.WriteString("\tm.Register(&m.Migration{\n")
	b.WriteString(fmt.Sprintf("\t\tName: %q,\n", name))

	// Dependencies
	b.WriteString("\t\tDependencies: []string{")
	if len(deps) > 0 {
		depStrs := make([]string, len(deps))
		for i, d := range deps {
			depStrs[i] = fmt.Sprintf("%q", d)
		}
		b.WriteString(strings.Join(depStrs, ", "))
	}
	b.WriteString("},\n")

	// Operations
	b.WriteString("\t\tOperations: []m.Operation{\n")
	b.WriteString("\t\t\t&m.RunSQL{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\tForwardSQL:  %s,\n", goRawString(forwardSQL)))
	b.WriteString(fmt.Sprintf("\t\t\t\tBackwardSQL: %s,\n", goRawString(backwardSQL)))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t},\n")

	b.WriteString("\t})\n")
	b.WriteString("}\n")

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String(), fmt.Errorf("formatting generated code: %w", err)
	}
	return string(formatted), nil
}

// goRawString wraps s in a Go backtick raw string literal. If s is empty, it
// returns `""`. If s contains backticks, it splits on them and joins with
// concatenated backtick-containing string literals.
func goRawString(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	// Split on backtick and join with ` + "`" + `
	parts := strings.Split(s, "`")
	var b strings.Builder
	for i, part := range parts {
		if i > 0 {
			b.WriteString(" + \"`\" + ")
		}
		b.WriteString("`")
		b.WriteString(part)
		b.WriteString("`")
	}
	return b.String()
}

// ExecuteMigrateGooseHistory migrates goose_db_version history to makemigrations_history.
// It discovers SQL files in migrationsDir to build the version-ID-to-go-name mapping,
// then reads goose_db_version and records each applied migration in makemigrations_history.
// Exported for use in tests with an injected DB connection.
func ExecuteMigrateGooseHistory(db *sql.DB, migrationsDir string, dryRun bool, out io.Writer) error {
	sqlFiles, err := discoverSQLFiles(migrationsDir)
	if err != nil {
		return err
	}
	entries := make([]migrateEntry, 0, len(sqlFiles))
	for i, f := range sqlFiles {
		desc := gooseparser.ExtractDescription(f)
		goName := fmt.Sprintf("%04d_%s", i+1, desc)
		vid, vErr := gooseparser.ExtractVersionID(f)
		if vErr != nil {
			return vErr
		}
		entries = append(entries, migrateEntry{sqlFile: f, goName: goName, versionID: vid})
	}
	return migrateHistoryWithEntries(db, entries, dryRun, out)
}

// migrateHistoryWithEntries reads goose_db_version and inserts applied migrations
// into makemigrations_history using the new Go migration names.
func migrateHistoryWithEntries(db *sql.DB, entries []migrateEntry, dryRun bool, out io.Writer) error {
	// Read last state per version_id from goose_db_version.
	// Iterate all rows in insertion order; last write per version_id wins.
	rows, err := db.Query("SELECT version_id, is_applied FROM goose_db_version ORDER BY id ASC")
	if err != nil {
		// If the table doesn't exist, warn and skip rather than failing.
		if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "does not exist") {
			fmt.Fprintf(out, "%s goose_db_version table not found — skipping history migration\n", yellow("⚠"))
			return nil
		}
		return fmt.Errorf("querying goose_db_version: %w", err)
	}
	defer func() { _ = rows.Close() }()

	appliedVersions := map[int64]bool{}
	for rows.Next() {
		var vid int64
		var isApplied bool
		if scanErr := rows.Scan(&vid, &isApplied); scanErr != nil {
			return fmt.Errorf("scanning goose_db_version: %w", scanErr)
		}
		appliedVersions[vid] = isApplied
	}
	if rowErr := rows.Err(); rowErr != nil {
		return fmt.Errorf("iterating goose_db_version: %w", rowErr)
	}

	recorder := migrate.NewMigrationRecorder(db)
	if !dryRun {
		if ensureErr := recorder.EnsureTable(); ensureErr != nil {
			return ensureErr
		}
	}

	fmt.Fprintf(out, "\nMigrating history (goose_db_version → makemigrations_history):\n")
	for _, e := range entries {
		isApplied := appliedVersions[e.versionID]
		status := "pending"
		if isApplied {
			status = "applied"
		}
		fmt.Fprintf(out, "  %-30s %s\n", e.goName, status)
		if isApplied && !dryRun {
			if recErr := recorder.RecordApplied(e.goName); recErr != nil {
				return fmt.Errorf("recording %s as applied: %w", e.goName, recErr)
			}
		}
	}
	return nil
}
