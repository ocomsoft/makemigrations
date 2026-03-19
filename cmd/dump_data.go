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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/dumpdata"
	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/migrate"
)

var (
	dumpDataName        string
	dumpDataDryRun      bool
	dumpDataVerbose     bool
	dumpDataConflictKey []string
	dumpDataDSN         string
)

// dumpDataCmd is the "makemigrations dump-data" subcommand. It connects to a
// live database, fetches all rows from the specified tables, and generates a
// Go migration file containing UpsertData operations for each table.
var dumpDataCmd = &cobra.Command{
	Use:   "dump-data [table1 table2 ...]",
	Short: "Generate a migration that upserts data from live database tables",
	Long: `Connects to a live database, fetches all rows from the specified tables,
and generates a Go migration file containing UpsertData operations.

Primary keys are determined from the migration SchemaState (reconstructed from
existing migrations). Use --conflict-key to override PK detection or when the
table has no migrations yet.

The generated migration depends on the current DAG leaves so it slots into
the migration chain correctly.

Examples:
  # Dump a single table
  makemigrations dump-data countries

  # Dump multiple tables with a custom name
  makemigrations dump-data countries currencies --name seed_reference_data

  # Override conflict keys for tables without migrations
  makemigrations dump-data legacy_config --conflict-key code

  # Preview without writing
  makemigrations dump-data countries --dry-run

  # Use a full DSN instead of individual flags
  makemigrations dump-data countries --dsn "host=localhost port=5432 dbname=myapp user=dev sslmode=disable"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDumpData,
}

func init() {
	rootCmd.AddCommand(dumpDataCmd)

	dumpDataCmd.Flags().StringVar(&dumpDataName, "name", "",
		"Custom migration name suffix")
	dumpDataCmd.Flags().BoolVar(&dumpDataDryRun, "dry-run", false,
		"Print generated migration without writing")
	dumpDataCmd.Flags().BoolVar(&dumpDataVerbose, "verbose", false,
		"Show connection and row-count details")
	dumpDataCmd.Flags().StringSliceVar(&dumpDataConflictKey, "conflict-key", nil,
		"Override PK detection; applied to all tables")
	dumpDataCmd.Flags().StringVar(&dumpDataDSN, "dsn", "",
		"Full database DSN (overrides host/port/etc)")

	// Register DB connection flags bound to existing package-level vars
	// declared in cmd/db2schema.go.
	dumpDataCmd.Flags().StringVar(&host, "host", "", "Database host (default: localhost)")
	dumpDataCmd.Flags().IntVar(&port, "port", 0, "Database port (default: 5432 for PostgreSQL)")
	dumpDataCmd.Flags().StringVar(&database, "database", "", "Database name")
	dumpDataCmd.Flags().StringVar(&username, "username", "", "Database username")
	dumpDataCmd.Flags().StringVar(&password, "password", "", "Database password")
	dumpDataCmd.Flags().StringVar(&sslmode, "sslmode", "", "SSL mode (default: disable)")
}

// runDumpData generates a dump-data migration by fetching rows from a live
// database and wrapping them in UpsertData operations.
func runDumpData(_ *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory

	// Determine DB type from config, default to postgresql.
	dbType := cfg.Database.Type
	if dbType == "" {
		dbType = "postgresql"
	}

	// Scan for existing migration files.
	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	// Query DAG for current leaves and schema state.
	var deps []string
	var schemaState *migrate.SchemaState

	if len(migFiles) > 0 {
		dagOut, dagErr := queryDAG(migrationsDir, dumpDataVerbose)
		if dagErr != nil {
			return fmt.Errorf("querying migration DAG: %w", dagErr)
		}

		deps = dagOut.Leaves
		schemaState = dagOut.SchemaState
	}

	// Open DB connection.
	dsn := buildDumpDataDSN(dbType)
	driverName := driverForDBType(dbType)

	if dumpDataVerbose {
		fmt.Printf("Connecting to %s database...\n", dbType)
	}

	db, err := dumpdata.OpenDB(driverName, dsn)
	if err != nil {
		return fmt.Errorf("opening database connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Fetch data for each table.
	var tables []codegen.TableDump

	for _, tableName := range args {
		// Determine conflict keys.
		conflictKeys := resolveConflictKeys(schemaState, tableName)

		if len(conflictKeys) == 0 {
			return fmt.Errorf(
				"no primary keys found for table %q in migration state; use --conflict-key to specify them",
				tableName,
			)
		}

		if dumpDataVerbose {
			fmt.Printf("Fetching rows from %q (conflict keys: %v)...\n", tableName, conflictKeys)
		}

		rows, _, fetchErr := dumpdata.FetchRows(db, tableName)
		if fetchErr != nil {
			return fmt.Errorf("fetching rows from %q: %w", tableName, fetchErr)
		}

		if dumpDataVerbose {
			fmt.Printf("  %d rows fetched\n", len(rows))
		}

		tables = append(tables, codegen.TableDump{
			Table:        tableName,
			ConflictKeys: conflictKeys,
			Rows:         rows,
		})
	}

	// Build migration name.
	name := BuildMigrationName(len(migFiles), dumpDataName, buildDumpAutoName(args))

	if dumpDataVerbose {
		fmt.Printf("Generating migration: %s\n", name)
		if len(deps) > 0 {
			fmt.Printf("Dependencies: %v\n", deps)
		}
	}

	// Generate source.
	gen := codegen.NewDumpDataGenerator()

	src, err := gen.Generate(name, deps, tables)
	if err != nil {
		return fmt.Errorf("generating dump-data migration: %w", err)
	}

	if dumpDataDryRun {
		fmt.Print(src)
		return nil
	}

	// Write the migration file.
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}

	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0o644); err != nil {
		return fmt.Errorf("writing dump-data migration: %w", err)
	}

	fmt.Printf("Created %s\n", outPath)
	return nil
}

// resolveConflictKeys determines the conflict keys for a table. If --conflict-key
// was provided, those are used directly. Otherwise primary keys are extracted
// from the migration SchemaState.
func resolveConflictKeys(state *migrate.SchemaState, tableName string) []string {
	if len(dumpDataConflictKey) > 0 {
		return dumpDataConflictKey
	}

	return pksFromState(state, tableName)
}

// pksFromState extracts primary-key field names for the given table from
// the reconstructed migration SchemaState. Returns nil if the table is not
// found or has no primary key fields.
func pksFromState(state *migrate.SchemaState, table string) []string {
	if state == nil {
		return nil
	}

	ts, ok := state.Tables[table]
	if !ok {
		return nil
	}

	var pks []string
	for _, f := range ts.Fields {
		if f.PrimaryKey {
			pks = append(pks, f.Name)
		}
	}

	return pks
}

// buildDumpDataDSN builds the DSN from --dsn flag, the DATABASE_URL environment
// variable, or from individual connection flags. Each DB type uses the
// appropriate format.
func buildDumpDataDSN(dbType string) string {
	if dumpDataDSN != "" {
		return dumpDataDSN
	}

	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	switch strings.ToLower(dbType) {
	case "postgresql", "postgres":
		return buildConnectionString(types.DatabasePostgreSQL)
	case "mysql", "tidb":
		h := host
		if h == "" {
			h = "localhost"
		}

		p := port
		if p == 0 {
			p = 3306
		}

		db := database
		if db == "" {
			db = "mysql"
		}

		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, h, p, db)
	case "sqlite":
		if database != "" {
			return database
		}

		return "makemigrations.db"
	default:
		return buildConnectionString(types.DatabasePostgreSQL)
	}
}

// driverForDBType returns the database/sql driver name for the given DB type.
func driverForDBType(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgresql", "postgres":
		return "postgres"
	case "mysql", "tidb":
		return "mysql"
	case "sqlite":
		return "sqlite3"
	default:
		return "postgres"
	}
}

// buildDumpAutoName generates a default migration name suffix based on the
// table names being dumped.
func buildDumpAutoName(tables []string) string {
	if len(tables) == 1 {
		return "dump_" + tables[0]
	}

	return "dump_data"
}
