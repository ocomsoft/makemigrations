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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
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
		prevSchema = schemaStateToYAMLSchema(dagOut.SchemaState)
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

	// 4. Handle merge if requested
	if goMigMerge && dagOut != nil && dagOut.HasBranches {
		return goGenerateMerge(migrationsDir, dagOut, goMigDryRun, goMigVerbose)
	}

	// 5. Check for branches (warn if present and not doing merge)
	if dagOut != nil && dagOut.HasBranches && !goMigMerge {
		fmt.Printf("WARNING: Branches detected: %s\n", strings.Join(dagOut.Leaves, ", "))
		fmt.Println("Run 'makemigrations makemigrations --merge' to generate a merge migration.")
	}

	if !diff.HasChanges {
		fmt.Println("No changes detected.")
		return nil
	}

	if goMigCheck {
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
		fmt.Println("  1) Generate  — include SQL in migration")
		fmt.Println("  2) Review    — include with // REVIEW comment")
		fmt.Println("  3) Omit      — skip SQL; schema state still advances (SchemaOnly)")
		fmt.Println("  4) Exit      — cancel migration generation")
		fmt.Println("  5) All       — generate all remaining destructive ops without prompting")
		fmt.Print("Choice [1-5]: ")

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
		default:
			decisions[i] = yamlpkg.PromptGenerate
		}
	}
	return decisions, nil
}

// buildMigrationsBinary compiles the migrations module in migrationsDir into a
// temporary binary. It returns the binary path and a cleanup function that
// removes the temporary directory. The caller must invoke cleanup() when done.
func buildMigrationsBinary(migrationsDir string, verbose bool) (binPath string, cleanup func(), err error) {
	absMigrationsDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolving migrations dir: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "makemigrations-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	tmpBin := filepath.Join(tmpDir, "migrations-bin")

	if verbose {
		fmt.Printf("Building migration binary from %s...\n", absMigrationsDir)
	}

	// Determine the Go version for the temp workspace.
	// Prefer the toolchain directive from the nearest parent go.work/go.mod
	// (e.g. "go1.25.7" → "1.25.7"), which has an exact patch version and is
	// already cached locally. Fall back to the version of the currently running
	// binary (runtime.Version() strips the leading "go"). A plain major.minor
	// like "1.25" from a go directive is not enough — Go would try to download
	// a toolchain to satisfy it.
	parentDir := filepath.Dir(absMigrationsDir)
	goVersion := findParentGoVersion(parentDir)
	if !isFullGoVersion(goVersion) {
		// runtime.Version() returns e.g. "go1.25.7"; strip the "go" prefix.
		goVersion = strings.TrimPrefix(runtime.Version(), "go")
	}

	// Check parent go.mod files for a local replace of makemigrations.
	// If found, include it in the workspace so the build resolves locally
	// without any network access or go.sum concerns.
	localMakemig := findLocalMakemigrations(parentDir)

	// Build a temporary go.work that includes the migrations module (and
	// optionally a local makemigrations). This solves two problems:
	//  1. Parent workspace conflict: a go.work in a parent directory may not
	//     list the migrations sub-module, causing "main module does not contain
	//     package" errors when GOWORK is inherited.
	//  2. Toolchain downloads: by declaring the same go version as the parent
	//     project, Go reuses the already-cached toolchain rather than fetching
	//     a different one.
	var work strings.Builder
	fmt.Fprintf(&work, "go %s\n\nuse %s\n", goVersion, absMigrationsDir)
	if localMakemig != "" {
		fmt.Fprintf(&work, "use %s\n", localMakemig)
		if verbose {
			fmt.Printf("Using local makemigrations: %s\n", localMakemig)
		}
	}

	var goEnv []string
	tmpWork := filepath.Join(tmpDir, "go.work")
	if werr := os.WriteFile(tmpWork, []byte(work.String()), 0o644); werr == nil {
		goEnv = append(os.Environ(), "GOWORK="+tmpWork)
	} else {
		// Fallback: disable workspace entirely.
		goEnv = append(os.Environ(), "GOWORK=off")
	}

	// When no local makemigrations is available the migrations go.sum may be
	// stale (e.g. a new subpackage was added to the published module). Run
	// go mod download to sync go.sum without modifying go.mod.
	if localMakemig == "" {
		downloadCmd := exec.Command("go", "mod", "download")
		downloadCmd.Dir = absMigrationsDir
		downloadCmd.Env = goEnv
		if out, dlErr := downloadCmd.CombinedOutput(); dlErr != nil {
			if verbose {
				fmt.Printf("go mod download warning: %s\n", string(out))
			}
		}
	}

	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	buildCmd.Dir = absMigrationsDir
	buildCmd.Env = goEnv
	if out, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("building migration binary: %w\nOutput: %s", buildErr, string(out))
	}

	return tmpBin, cleanup, nil
}

// queryDAG builds the migrations binary in a temporary directory and runs
// `dag --format json` to retrieve the current migration graph state. The
// binary is cleaned up automatically when the function returns.
func queryDAG(migrationsDir string, verbose bool) (*migrate.DAGOutput, error) {
	binPath, cleanup, err := buildMigrationsBinary(migrationsDir, verbose)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	dagCmd := exec.Command(binPath, "dag", "--format", "json")
	dagOutput, err := dagCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running dag command: %w", err)
	}

	var result migrate.DAGOutput
	if err := json.Unmarshal(dagOutput, &result); err != nil {
		return nil, fmt.Errorf("parsing DAG output: %w", err)
	}
	return &result, nil
}

// schemaStateToYAMLSchema converts a migrate.SchemaState (reconstructed from
// the migration DAG) into a yaml.Schema suitable for diffing against the
// current YAML schema. Tables are sorted by name for deterministic output.
func schemaStateToYAMLSchema(state *migrate.SchemaState) *yamlpkg.Schema {
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
				yf.ForeignKey = &yamlpkg.ForeignKey{
					Table:    f.ForeignKey.Table,
					OnDelete: f.ForeignKey.OnDelete,
				}
			}
			t.Fields = append(t.Fields, yf)
		}
		for _, idx := range ts.Indexes {
			t.Indexes = append(t.Indexes, yamlpkg.Index{
				Name:   idx.Name,
				Fields: idx.Fields,
				Unique: idx.Unique,
			})
		}
		schema.Tables = append(schema.Tables, t)
	}
	// Sort tables for determinism
	sort.Slice(schema.Tables, func(i, j int) bool {
		return schema.Tables[i].Name < schema.Tables[j].Name
	})
	return schema
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

// isFullGoVersion reports whether v is a complete three-part Go version
// (e.g. "1.25.7") rather than a partial major.minor like "1.25". A partial
// version used in a go.work 'go' directive causes Go to attempt a toolchain
// download rather than reusing an already-installed binary.
func isFullGoVersion(v string) bool {
	parts := strings.Split(v, ".")
	return len(parts) >= 3
}

// findLocalMakemigrations walks up from startDir looking for a go.mod that
// has a replace directive for github.com/ocomsoft/makemigrations pointing to
// a local path. Returns the absolute path of the local module, or "" if none
// is found.
func findLocalMakemigrations(startDir string) string {
	const modPkg = "github.com/ocomsoft/makemigrations"
	dir := startDir
	for {
		modPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modPath); err == nil {
			if f, err := modfile.Parse(modPath, data, nil); err == nil {
				for _, r := range f.Replace {
					if r.Old.Path == modPkg && r.New.Path != "" {
						p := r.New.Path
						if !filepath.IsAbs(p) {
							p = filepath.Join(dir, filepath.FromSlash(p))
						}
						if abs, err := filepath.Abs(p); err == nil {
							return abs
						}
						return p
					}
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
