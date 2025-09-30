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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ocomsoft/makemigrations/internal/version"
)

var (
	cfgFile    string
	configFile string // Config file path
	dryRun     bool
	check      bool
	customName string
	verbose    bool
	silent     bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Django-style migration generator for Go",
	Long: `Generate database migrations from schema.sql files in Go modules.

This tool scans Go module dependencies for schema.sql files, merges them into 
a unified schema, and generates Goose-compatible migration files by comparing 
against the last known schema state.

When run without a subcommand, defaults to 'makemigrations_sql'.

Available commands:
- init: Initialize migrations directory and create initial migration from YAML schemas
- init_sql: Initialize migrations directory and create initial migration from SQL schemas
- makemigrations: Generate migrations from YAML schemas
- makemigrations_sql: Generate migrations from schema.sql files

Features:
- Scans direct Go module dependencies for sql/schema.sql files
- Merges duplicate tables with intelligent conflict resolution
- Handles foreign key dependencies and circular references
- Generates both UP and DOWN migrations
- Adds REVIEW comments for destructive operations
- Compatible with Goose migration runner`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to makemigrations_sql when no subcommand is provided
		// Import the logic directly since this is the default behavior
		return runDefaultMakeMigrations(cmd, args)
	},
}

// GetRootCmd returns the root command for embedding in other applications
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Display version at startup for all commands
	fmt.Printf("%s\n", version.GetDisplayVersion())
	cobra.CheckErr(rootCmd.Execute())
}

// runDefaultMakeMigrations runs the makemigrations_sql functionality as the default command
func runDefaultMakeMigrations(cmd *cobra.Command, args []string) error {
	return ExecuteSQLMakeMigrations(verbose, dryRun, check, customName)
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flag for config file
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path (default: migrations/makemigrations.config.yaml)")

	// Add the main command flags
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be generated without creating files")
	rootCmd.Flags().BoolVar(&check, "check", false, "Exit with error code if migrations are needed (for CI/CD)")
	rootCmd.Flags().StringVar(&customName, "name", "", "Override auto-generated migration name")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
	rootCmd.Flags().BoolVar(&silent, "silent", false, "Skip prompts for destructive operations (use review comments instead)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".makemigrations" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".makemigrations")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
