# Remove SQL Migration Code Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all SQL migration code paths and subcommands, making this a Go-only migration generator project.

**Architecture:** Strip the legacy SQL migration pipeline (schema.sql scanning, SQL diffing, Goose integration, YAML-to-SQL generation) while preserving the Go migration framework (YAML schema → Go code generation, DAG-based runner, provider interface). The `init` command keeps `.schema_snapshot.yaml` support for bootstrapping Go migrations.

**Tech Stack:** Go, Cobra CLI, YAML schemas, Go code generation

---

### Task 1: Delete SQL-only subcommand files

**Files:**
- Delete: `cmd/init_sql.go`
- Delete: `cmd/sql_migrations.go`
- Delete: `cmd/makemigrations_sql.go`
- Delete: `cmd/migrate_to_go.go`
- Delete: `cmd/migrate_to_go_test.go`
- Delete: `cmd/goose.go`
- Delete: `cmd/makemigrations_test.go` (tests YAML-to-SQL path via `runYAMLMakeMigrations`)

**Step 1: Delete the files**

```bash
rm cmd/init_sql.go cmd/sql_migrations.go cmd/makemigrations_sql.go \
   cmd/migrate_to_go.go cmd/migrate_to_go_test.go cmd/goose.go \
   cmd/makemigrations_test.go
```

**Step 2: Verify deletion**

Run: `ls cmd/*.go`
Expected: Should show `common.go`, `db2schema.go`, `dump_sql.go`, `find_includes.go`, `go_init.go`, `go_init_test.go`, `go_migrations.go`, `go_migrations_test.go`, `init.go`, `migrate.go`, `root.go`, `schema2diagram.go`, `struct2schema.go`, `version.go`, `yaml_common.go`

**Step 3: Commit**

```bash
git add -u cmd/
git commit -m "chore: delete SQL-only subcommand files

Remove init_sql, sql_migrations, makemigrations_sql, migrate_to_go,
goose commands and their tests."
```

---

### Task 2: Delete SQL-only internal packages

**Files:**
- Delete: `internal/analyzer/` (SQL dependency analysis, only used by `common.go:ExecuteSQLMakeMigrations`)
- Delete: `internal/diff/` (SQL-based diffing, only used by `common.go:ExecuteSQLMakeMigrations`)
- Delete: `internal/generator/` (SQL migration file generation)
- Delete: `internal/gooseparser/` (Goose SQL file parser, only used by `migrate_to_go.go`)
- Delete: `internal/merger/` (SQL schema merging)
- Delete: `internal/parser/` (SQL schema parsing)
- Delete: `internal/state/` (SQL `.schema_snapshot.sql` state manager)
- Delete: `internal/writer/` (SQL migration file writer)

**Step 1: Delete the directories**

```bash
rm -rf internal/analyzer internal/diff internal/generator \
       internal/gooseparser internal/merger internal/parser \
       internal/state internal/writer
```

**Step 2: Verify deletion**

Run: `ls internal/`
Expected: `codegen config errors fkutils providers scanner struct2schema types utils version yaml`

**Step 3: Commit**

```bash
git add -u internal/
git commit -m "chore: delete SQL-only internal packages

Remove analyzer, diff, generator, gooseparser, merger, parser,
state, and writer packages that supported the SQL migration pipeline."
```

---

### Task 3: Clean up `cmd/common.go`

Remove `ExecuteSQLMakeMigrations` and all its SQL-pipeline imports. Keep `ExecuteStruct2Schema`.

**Files:**
- Modify: `cmd/common.go`

**Step 1: Rewrite common.go**

Replace the entire file with only `ExecuteStruct2Schema`:

```go
/*
MIT License

# Copyright (c) 2025 OcomSoft
...license header...
*/
package cmd

import (
	"fmt"

	"github.com/ocomsoft/makemigrations/internal/struct2schema"
)

// ExecuteStruct2Schema handles the complete struct-to-schema conversion process
func ExecuteStruct2Schema(inputDir, outputFile, configFile, targetDB string, dryRun, verbose bool) error {
	if verbose {
		fmt.Println("struct2schema - Go struct to YAML schema converter")
		fmt.Println("=============================================")
	}

	// Initialize the struct2schema processor
	processor, err := struct2schema.NewProcessor(struct2schema.ProcessorConfig{
		InputDir:   inputDir,
		OutputFile: outputFile,
		ConfigFile: configFile,
		TargetDB:   targetDB,
		DryRun:     dryRun,
		Verbose:    verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize processor: %w", err)
	}

	// Process the structs
	if err := processor.Process(); err != nil {
		return fmt.Errorf("failed to process structs: %w", err)
	}

	if dryRun {
		fmt.Println("\nDry run completed successfully - no files were modified")
	} else {
		if verbose {
			fmt.Printf("\nSchema file written to: %s\n", outputFile)
		}
		fmt.Println("struct2schema completed successfully")
	}

	return nil
}
```

**Step 2: Verify build compiles**

Run: `cd /workspace/makemigrations && go build ./cmd/...`
Expected: May fail due to references from root.go — that's OK, fixed in Task 5.

**Step 3: Commit**

```bash
git add cmd/common.go
git commit -m "refactor: remove ExecuteSQLMakeMigrations from common.go"
```

---

### Task 4: Clean up `cmd/yaml_common.go`

Remove `ExecuteYAMLMakeMigrations`, `ExecuteYAMLInit`, and `MigrationGenerator` from `YAMLComponents`. Keep `InitializeYAMLComponents`, `ScanAndParseSchemas`, `MergeAndValidateSchemas`, `ExecuteDumpSQL`, `ExecuteFindIncludes`.

**Files:**
- Modify: `cmd/yaml_common.go`

**Step 1: Remove `MigrationGenerator` from `YAMLComponents` struct**

Remove the `MigrationGenerator` field from the struct and its initialization line in `InitializeYAMLComponents`. The field and import `yamlpkg.MigrationGenerator` are no longer needed.

In the `YAMLComponents` struct, remove:
```go
MigrationGenerator *yamlpkg.MigrationGenerator
```

In `InitializeYAMLComponents`, remove the line:
```go
MigrationGenerator: yamlpkg.NewMigrationGeneratorWithFullConfig(dbType, verbose, cfg.Migration.IncludeDownSQL, cfg.Migration.ReviewCommentPrefix, cfg.Migration.DestructiveOperations, cfgSilent, cfg.Migration.RejectionCommentPrefix, cfg.Migration.FilePrefix),
```

Also simplify `InitializeYAMLComponents` — the config loading for `IncludeDownSQL`, `ReviewCommentPrefix`, `DestructiveOperations`, `cfgSilent`, `RejectionCommentPrefix`, `FilePrefix` is only needed for `MigrationGenerator`. The remaining fields (`StateManager`, `Scanner`, `Parser`, `Merger`, `DiffEngine`) don't use those settings. Simplify accordingly.

**Step 2: Delete `ExecuteYAMLMakeMigrations` function (lines 164-352)**

Remove the entire function.

**Step 3: Delete `ExecuteYAMLInit` function (lines 354-484)**

Remove the entire function.

**Step 4: Verify build compiles**

Run: `cd /workspace/makemigrations && go build ./cmd/...`
Expected: May still fail due to root.go — fixed in Task 5.

**Step 5: Commit**

```bash
git add cmd/yaml_common.go
git commit -m "refactor: remove SQL generation functions from yaml_common.go

Remove ExecuteYAMLMakeMigrations, ExecuteYAMLInit, and MigrationGenerator
from YAMLComponents. Keep shared YAML scan/parse/merge functions used by
the Go migration path."
```

---

### Task 5: Update `cmd/root.go` — change default command

The root command currently defaults to `ExecuteSQLMakeMigrations`. Change it to run the Go migration generator (`runGoMakeMigrations`).

**Files:**
- Modify: `cmd/root.go`

**Step 1: Update root command**

Changes needed:
1. Remove the `RunE` that calls `runDefaultMakeMigrations`
2. Remove `runDefaultMakeMigrations` function
3. Update help text to reflect Go-only project
4. Remove `silent` flag variable (was only used by SQL path, and `goose.go` which defined color vars is gone)
5. Keep `dryRun`, `check`, `customName`, `verbose` flags — these are used by Go migrations too. Actually, the Go migrations command defines its own `goMigDryRun` etc. flags. The root flags are only used by `runDefaultMakeMigrations` which is being removed. So remove them from root and make root just show help.

Updated root command:
```go
var rootCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Django-style Go migration generator",
	Long: `Generate database migrations from YAML schema files as typed Go code.

Available commands:
  init              Initialize migrations directory and create initial migration
  makemigrations    Generate Go migration files from YAML schema changes
  migrate           Build and run the compiled migrations binary
  db2schema         Extract database schema to YAML
  struct2schema     Convert Go structs to YAML schema
  dump_sql          Dump merged YAML schema as SQL
  schema2diagram    Generate diagram from YAML schema
  find_includes     Discover schema includes in Go modules`,
}
```

Remove `runDefaultMakeMigrations`, remove unused flag vars (`dryRun`, `check`, `customName`, `silent`), remove flags from `rootCmd.Flags()`. Keep `configFile` and `verbose` as persistent flags since they're used by multiple commands.

**Step 2: Clean up init() function**

Remove flags that were for the SQL default behavior. Keep `configFile` persistent flag.

**Step 3: Verify build compiles**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "refactor: remove SQL default from root command

Root command now shows help instead of defaulting to SQL migration
generation. Use 'makemigrations makemigrations' for Go migration
generation."
```

---

### Task 6: Update `cmd/init.go` — remove `--sql` flag

Remove the `--sql` flag and the `initSQLMode` variable. Go directly to `ExecuteGoMigrationInit`.

**Files:**
- Modify: `cmd/init.go`

**Step 1: Simplify init.go**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

var (
	initDatabaseType string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize migrations directory for the Go migration framework",
	Long: `Bootstrap the migrations/ directory for the Django-style Go migration framework.

This command:
- Creates the migrations/ directory if it doesn't exist
- Generates migrations/main.go and migrations/go.mod
- If a .schema_snapshot.yaml exists, generates an initial 0001_initial.go migration
- Prints instructions for faking the initial migration on an existing database

Use this command when setting up makemigrations for the first time in a project.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return ExecuteGoMigrationInit(initDatabaseType, verbose)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&initDatabaseType, "database", "postgresql",
		"Target database type (postgresql, mysql, sqlserver, sqlite)")
	initCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
}
```

**Step 2: Verify build compiles**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "refactor: remove --sql flag from init command

Init now only supports the Go migration framework."
```

---

### Task 7: Remove `internal/yaml/migration_generator.go` (SQL generator)

This file generates Goose-compatible SQL migration files from YAML diffs. It's the SQL generation engine for the YAML workflow. With `ExecuteYAMLMakeMigrations` and `ExecuteYAMLInit` removed, nothing references it.

**Files:**
- Delete: `internal/yaml/migration_generator.go`
- Delete: `internal/yaml/migration_generator_test.go`

**Step 1: Delete the files**

```bash
rm internal/yaml/migration_generator.go internal/yaml/migration_generator_test.go
```

**Step 2: Verify no remaining references**

Run: `grep -r "MigrationGenerator\|NewMigrationGenerator" internal/ cmd/`
Expected: No matches

**Step 3: Verify build compiles**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add -u internal/yaml/
git commit -m "chore: delete YAML-to-SQL migration generator

Remove migration_generator.go which generated Goose-compatible SQL
migration files from YAML schema diffs."
```

---

### Task 8: Clean up `go.mod` — remove Goose dependency

The `github.com/pressly/goose/v3` dependency and its SQL driver imports (`lib/pq`, `go-sql-driver/mysql`, `mattn/go-sqlite3`, `microsoft/go-mssqldb`) were used by `cmd/goose.go`. Check if any remaining code uses these.

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Check remaining usage of goose**

Run: `grep -r "pressly/goose" --include="*.go" .`
Expected: No matches (goose.go was deleted in Task 1)

**Step 2: Check SQL driver imports**

Run: `grep -r "lib/pq\|go-sql-driver/mysql\|mattn/go-sqlite3\|microsoft/go-mssqldb" --include="*.go" .`
Expected: May still be used by the migrate runtime or providers. Only remove if no references remain.

**Step 3: Run go mod tidy**

```bash
cd /workspace/makemigrations && go mod tidy
```

**Step 4: Verify build**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 5: Run all tests**

Run: `cd /workspace/makemigrations && go test ./...`
Expected: PASS (or only expected failures)

**Step 6: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: remove unused dependencies after SQL code removal"
```

---

### Task 9: Verify build and tests pass

**Step 1: Full build**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 2: Run all tests**

Run: `cd /workspace/makemigrations && go test ./... 2>&1 | head -100`
Expected: All tests pass

**Step 3: Verify CLI help**

Run: `cd /workspace/makemigrations && go run . --help`
Expected: Shows updated help text with Go-only commands. Should NOT show `init_sql`, `sql-migrations`, `migrate-to-go`, `goose`, or `makemigrations_sql`.

**Step 4: Verify init command help**

Run: `cd /workspace/makemigrations && go run . init --help`
Expected: No `--sql` flag listed.

**Step 5: Commit (if any fixes needed)**

Only if previous tasks introduced issues that needed fixing.

---

### Task 10: Rework `dump_sql` to show SQL for pending migrations

Currently `dump_sql` dumps the entire merged YAML schema as SQL. The new behavior should show only the SQL that *pending migrations would execute* — i.e., the SQL operations from unapplied Go migration files. This lets developers preview what `migrate up` would do.

**Files:**
- Modify: `cmd/dump_sql.go`
- Modify: `cmd/yaml_common.go` (remove `ExecuteDumpSQL` — replaced with new logic)

**Step 1: Understand current flow**

Current `dump_sql` calls `ExecuteDumpSQL` which:
1. Scans YAML schemas
2. Merges them
3. Converts entire schema to SQL via `SQLConverter`

New behavior should:
1. Build the migrations binary (like `go_migrations.go` does)
2. Query the DAG for the current schema state
3. Diff against current YAML schema
4. For each change in the diff, call `GenerateMigration` to get the Go source
5. Instead of writing the file, use the provider to render the SQL that each operation's `Up()` would produce
6. Print the SQL to stdout

**Step 2: Rewrite `cmd/dump_sql.go`**

The new implementation should:
1. Load config and determine database type
2. Build the migrations binary and query DAG (reuse `queryDAG` from `go_migrations.go`)
3. Parse current YAML schemas
4. Diff previous state vs current schema
5. For each change, create the corresponding operation and call `Up()` with the appropriate provider
6. Print all SQL statements to stdout

Use the `migrate` package's operations and provider interface to render SQL. This means:
- Create a provider via `migrate.BuildProviderFromType(dbType)`
- For each diff change, create the corresponding `m.Operation` (CreateTable, AddField, etc.)
- Call `op.Up(provider, state, defaults)` to get the SQL string
- Print each SQL statement

**Step 3: Remove `ExecuteDumpSQL` from `yaml_common.go`**

Delete the function since `dump_sql.go` now has its own implementation.

**Step 4: Update help text**

Update the command's `Short` and `Long` descriptions to reflect the new behavior:
```
Short: "Preview SQL that pending migrations would execute"
Long: "Shows the SQL statements that would be generated for unapplied schema changes..."
```

**Step 5: Test manually**

Run: `cd /workspace/makemigrations && go build . && ./makemigrations dump_sql --help`
Expected: Updated help text

**Step 6: Commit**

```bash
git add cmd/dump_sql.go cmd/yaml_common.go
git commit -m "feat: rework dump_sql to preview pending migration SQL

Instead of dumping the entire schema, dump_sql now shows the SQL
that pending migrations would execute, using the provider interface
to render database-specific SQL."
```

---

### Task 11: Implement `GenerateAlterColumn` for all providers

Currently only PostgreSQL has a partial `GenerateAlterColumn` implementation. All other 11 providers return `"not implemented yet"`. The `AlterField` operation calls this method at runtime, so it must work for all supported databases.

**Files:**
- Modify: `internal/providers/mysql/provider.go`
- Modify: `internal/providers/sqlite/provider.go`
- Modify: `internal/providers/sqlserver/provider.go`
- Modify: `internal/providers/tidb/provider.go`
- Modify: `internal/providers/clickhouse/provider.go`
- Modify: `internal/providers/redshift/provider.go`
- Modify: `internal/providers/vertica/provider.go`
- Modify: `internal/providers/starrocks/provider.go`
- Modify: `internal/providers/auroradsql/provider.go`
- Modify: `internal/providers/turso/provider.go`
- Modify: `internal/providers/ydb/provider.go`

**Step 1: Study PostgreSQL's implementation as reference**

Read `internal/providers/postgresql/provider.go` GenerateAlterColumn (lines ~336-375).
The pattern: compare old field vs new field, generate ALTER TABLE statements for type changes, nullability changes, and default changes.

**Step 2: Implement for MySQL**

MySQL uses `ALTER TABLE t MODIFY COLUMN col type [NOT NULL] [DEFAULT val]`.

Write a failing test first, then implement.

**Step 3: Implement for SQLite**

SQLite doesn't support ALTER COLUMN natively. The standard approach is the 4-step table recreation:
1. Create new table with desired schema
2. Copy data
3. Drop old table
4. Rename new table

Or for newer SQLite (3.35+), `ALTER TABLE RENAME COLUMN` works for renames. For type/constraint changes, use table recreation.

**Step 4: Implement for SQL Server**

SQL Server uses `ALTER TABLE t ALTER COLUMN col type [NOT NULL]` for type/nullability changes, and separate `ALTER TABLE ... ADD/DROP CONSTRAINT` for defaults.

**Step 5: Implement for remaining providers**

Follow similar patterns, adapting SQL syntax for each database:
- **TiDB**: MySQL-compatible, use `MODIFY COLUMN`
- **ClickHouse**: `ALTER TABLE t MODIFY COLUMN col type [DEFAULT val]`
- **Redshift**: PostgreSQL-compatible subset
- **Vertica**: Similar to PostgreSQL
- **StarRocks**: MySQL-compatible subset
- **Aurora SQL**: PostgreSQL-compatible
- **Turso**: SQLite-compatible
- **YDB**: `ALTER TABLE t ALTER COLUMN col SET type`

**Step 6: Write tests for each provider**

Each provider should have a test that verifies GenerateAlterColumn produces correct SQL for:
- Type change
- Nullability change (NULL → NOT NULL, NOT NULL → NULL)
- Default value change

**Step 7: Commit**

```bash
git add internal/providers/
git commit -m "feat: implement GenerateAlterColumn for all providers

Add ALTER COLUMN SQL generation for MySQL, SQLite, SQL Server, TiDB,
ClickHouse, Redshift, Vertica, StarRocks, Aurora SQL, Turso, and YDB."
```

---

### Task 12: Implement remaining unimplemented provider methods

Several provider methods are inconsistently implemented across the 12 providers. Fill in the gaps.

**Methods to implement (by priority):**

1. **GenerateJunctionTable** — All 12 providers return "not implemented". This generates many-to-many junction tables. Implement for all providers.

2. **GenerateForeignKeyConstraint** — 5 providers return empty string. Implement for MySQL, SQLite, TiDB, Turso (and any others that support FK constraints).

3. **GenerateDropForeignKeyConstraint** — 4 providers return empty string. Implement for MySQL, SQLite, TiDB, Turso.

4. **GenerateForeignKeyConstraints** — 7 providers return empty string. Implement for providers that support FKs.

5. **InferForeignKeyType** — 5 providers return empty string. Return appropriate default FK column type for each database.

6. **GenerateIndexes** — 4 providers return empty string. Implement for MySQL, SQLite, Redshift, Turso.

7. **GetDatabaseSchema** — All 12 return "not implemented". This is for reverse engineering an existing database into YAML schema. Implement at least for PostgreSQL (which has a partial implementation), MySQL, and SQLite as the most common targets. Others can be lower priority.

**Approach:**
- Work through one method at a time across all providers
- Write failing tests first
- Use PostgreSQL's implementation as the reference pattern
- For databases that don't support a feature (e.g., ClickHouse doesn't support FK constraints), return an appropriate empty string or comment

**Step 1-N: For each method, implement across all applicable providers with tests**

This is a large task that should be broken into sub-tasks per method. Each method implementation should follow TDD.

**Commit after each method is complete across all providers.**

---

### Task 13: Final verification and cleanup

**Step 1: Full build**

Run: `cd /workspace/makemigrations && go build ./...`
Expected: PASS

**Step 2: Run all tests**

Run: `cd /workspace/makemigrations && go test ./...`
Expected: All tests pass

**Step 3: Verify CLI commands**

Run: `cd /workspace/makemigrations && go run . --help`
Expected: Clean help text, no SQL-only commands

**Step 4: Verify no dead code**

Run: `grep -r "not implemented" internal/providers/ | grep -v "_test.go"`
Expected: Significantly fewer "not implemented" occurrences than before

**Step 5: Final commit**

```bash
git add .
git commit -m "chore: final cleanup after SQL migration code removal"
```

---

## Summary of Changes

### Files Deleted (17 files + 8 directories):
- `cmd/init_sql.go`
- `cmd/sql_migrations.go`
- `cmd/makemigrations_sql.go`
- `cmd/migrate_to_go.go`
- `cmd/migrate_to_go_test.go`
- `cmd/goose.go`
- `cmd/makemigrations_test.go`
- `internal/analyzer/` (entire directory)
- `internal/diff/` (entire directory)
- `internal/generator/` (entire directory)
- `internal/gooseparser/` (entire directory)
- `internal/merger/` (entire directory)
- `internal/parser/` (entire directory)
- `internal/state/` (entire directory)
- `internal/writer/` (entire directory)
- `internal/yaml/migration_generator.go`
- `internal/yaml/migration_generator_test.go`

### Files Modified (4 files):
- `cmd/common.go` — remove `ExecuteSQLMakeMigrations`
- `cmd/yaml_common.go` — remove `ExecuteYAMLMakeMigrations`, `ExecuteYAMLInit`, `MigrationGenerator`
- `cmd/root.go` — remove SQL default, update help text
- `cmd/init.go` — remove `--sql` flag

### Files Preserved (Go migration pipeline):
- `cmd/go_migrations.go` — Go migration generation
- `cmd/go_init.go` — Go init
- `cmd/migrate.go` — migration runner
- `cmd/db2schema.go` — DB → YAML extraction
- `cmd/struct2schema.go` — Go structs → YAML
- `cmd/dump_sql.go` — YAML → SQL dump utility
- `cmd/schema2diagram.go` — diagram generation
- `cmd/find_includes.go` — schema discovery
- `internal/codegen/` — Go code generation
- `internal/yaml/` — YAML schema parsing, merging, diffing, state
- `internal/providers/` — database providers
- `internal/scanner/` — module scanning
- `internal/config/` — configuration
- `migrate/` — runtime framework
