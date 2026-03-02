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

var initDatabaseType string

// initCmd represents the init command
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
