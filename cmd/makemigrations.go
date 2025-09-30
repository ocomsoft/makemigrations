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
	"github.com/spf13/cobra"
)

var (
	databaseType string
)

// makemigrationsCmd represents the makemigrations command (YAML-based)
var makemigrationsCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Django-style migration generator from YAML schemas",
	Long: `Generate database migrations from schema.yaml files in Go modules.

This tool scans Go module dependencies for schema/schema.yaml files, merges them 
into a unified schema, and generates Goose-compatible migration files by comparing 
against the last known schema state.

The YAML schema format supports:
- Multiple database types (PostgreSQL, MySQL, SQL Server, SQLite)
- Foreign key relationships with cascade options
- Many-to-many relationships with auto-generated junction tables
- Database-specific default value mappings
- Field constraints and indexes

Features:
- Scans direct Go module dependencies for schema/schema.yaml files
- Merges duplicate tables with intelligent conflict resolution
- Handles foreign key dependencies and circular references
- Generates both UP and DOWN migrations with database-specific SQL
- Adds REVIEW comments for destructive operations
- Compatible with Goose migration runner

Database Support:
- PostgreSQL (default)
- MySQL
- SQL Server
- SQLite`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runYAMLMakeMigrations(cmd, args)
	},
}

// runYAMLMakeMigrations runs the YAML-based migration generation
func runYAMLMakeMigrations(cmd *cobra.Command, args []string) error {
	return ExecuteYAMLMakeMigrations(cmd, configFile, databaseType, verbose, silent, dryRun, check, customName)
}

func init() {
	rootCmd.AddCommand(makemigrationsCmd)

	// Add YAML-specific flags
	makemigrationsCmd.Flags().StringVar(&databaseType, "database", "postgresql",
		"Target database type (postgresql, mysql, sqlserver, sqlite)")
	makemigrationsCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be generated without creating files")
	makemigrationsCmd.Flags().BoolVar(&check, "check", false,
		"Exit with error code if migrations are needed (for CI/CD)")
	makemigrationsCmd.Flags().StringVar(&customName, "name", "",
		"Override auto-generated migration name")
	makemigrationsCmd.Flags().BoolVar(&verbose, "verbose", false,
		"Show detailed processing information")
	makemigrationsCmd.Flags().BoolVar(&silent, "silent", false,
		"Skip prompts for destructive operations (use review comments instead)")
}
