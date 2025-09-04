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

// Variables are now shared from root.go

// makemigrationsSqlCmd represents the makemigrations_sql command
var makemigrationsSqlCmd = &cobra.Command{
	Use:   "makemigrations_sql",
	Short: "Generate database migrations from schema.sql files",
	Long: `Generate database migrations from schema.sql files in Go modules.

This tool scans Go module dependencies for schema.sql files, merges them into 
a unified schema, and generates Goose-compatible migration files by comparing 
against the last known schema state.

Features:
- Scans direct Go module dependencies for sql/schema.sql files
- Merges duplicate tables with intelligent conflict resolution
- Handles foreign key dependencies and circular references
- Generates both UP and DOWN migrations
- Adds REVIEW comments for destructive operations
- Compatible with Goose migration runner`,
	RunE: runDefaultMakeMigrations,
}

func init() {
	rootCmd.AddCommand(makemigrationsSqlCmd)

	makemigrationsSqlCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be generated without creating files")
	makemigrationsSqlCmd.Flags().BoolVar(&check, "check", false, "Exit with error code if migrations are needed (for CI/CD)")
	makemigrationsSqlCmd.Flags().StringVar(&customName, "name", "", "Override auto-generated migration name")
	makemigrationsSqlCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
}

// runMakeMigrations function removed - now using shared runDefaultMakeMigrations from root.go
