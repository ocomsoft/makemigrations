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

	"github.com/spf13/cobra"

	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// dumpSQLCmd represents the dump_sql command
var dumpSQLCmd = &cobra.Command{
	Use:   "dump_sql",
	Short: "Dump the complete merged schema as SQL to console",
	Long: `Dump the complete merged schema as SQL to console.

This command scans all YAML schema files in Go module dependencies, merges them
into a unified schema, and outputs the complete SQL CREATE statements to the console.
This is useful for:

- Viewing the complete database schema
- Debugging schema merging issues
- Generating SQL for external tools
- Understanding the final merged schema structure

The output includes:
- All tables with their fields and constraints
- Foreign key relationships
- Indexes
- Database-specific SQL based on the --database flag

Database Support:
- PostgreSQL (default)
- MySQL
- SQL Server
- SQLite

The SQL output is equivalent to what would be generated in an initial migration
but is sent to stdout instead of being written to a file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDumpSQL(cmd, args)
	},
}

// runDumpSQL executes the dump_sql command
func runDumpSQL(cmd *cobra.Command, args []string) error {
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Dumping merged schema as SQL\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "============================\n")
	}

	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(databaseType)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Initialize YAML components
	components := InitializeYAMLComponents(dbType, verbose, false)
	sqlConverter := yamlpkg.NewSQLConverter(dbType, verbose)

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Database type: %s\n", dbType)
		fmt.Fprintf(cmd.ErrOrStderr(), "\n1. Scanning Go modules for YAML schema files...\n")
	}

	// Scan and parse schemas using shared function but adapt verbose output for dump_sql
	allSchemas, err := ScanAndParseSchemas(components, false) // Don't use verbose mode here since we customize output
	if err != nil {
		if err.Error() == "no YAML schema files found" {
			fmt.Fprintf(cmd.ErrOrStderr(), "No YAML schema files found. Nothing to dump.\n")
			return nil
		}
		return err
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n2. Parsing and merging YAML schemas...\n")
	}

	// Merge and validate schemas using shared function
	mergedSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, false) // Don't use verbose here since we customize output
	if err != nil {
		return fmt.Errorf("merged schema validation failed: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Merged schema: %d tables\n", len(mergedSchema.Tables))
		fmt.Fprintf(cmd.ErrOrStderr(), "\n3. Validating merged schema...\n")
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n4. Generating SQL...\n")
	}

	// Convert to SQL
	sql, err := sqlConverter.ConvertSchema(mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Generated %d lines of SQL\n", len(sql))
		fmt.Fprintf(cmd.ErrOrStderr(), "\n5. SQL Output:\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "================\n")
	}

	// Output SQL to stdout
	fmt.Print(sql)

	return nil
}

func init() {
	rootCmd.AddCommand(dumpSQLCmd)

	// Add flags
	dumpSQLCmd.Flags().StringVar(&databaseType, "database", "postgresql",
		"Target database type (postgresql, mysql, sqlserver, sqlite)")
	dumpSQLCmd.Flags().BoolVar(&verbose, "verbose", false,
		"Show detailed processing information")
}
