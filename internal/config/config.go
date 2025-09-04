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
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v3"
)

// Config represents the makemigrations configuration
type Config struct {
	// Database configuration
	Database DatabaseConfig `yaml:"database" mapstructure:"database"`

	// Migration settings
	Migration MigrationConfig `yaml:"migration" mapstructure:"migration"`

	// Schema settings
	Schema SchemaConfig `yaml:"schema" mapstructure:"schema"`

	// Output settings
	Output OutputConfig `yaml:"output" mapstructure:"output"`
}

// DatabaseConfig contains database-related settings
type DatabaseConfig struct {
	Type             string `yaml:"type" mapstructure:"type"`                           // postgresql, mysql, sqlserver, sqlite
	DefaultSchema    string `yaml:"default_schema" mapstructure:"default_schema"`       // Default schema name for databases that support schemas
	QuoteIdentifiers bool   `yaml:"quote_identifiers" mapstructure:"quote_identifiers"` // Whether to quote table/column names
}

// MigrationConfig contains migration-related settings
type MigrationConfig struct {
	Directory              string   `yaml:"directory" mapstructure:"directory"`                               // Directory for migration files
	FilePrefix             string   `yaml:"file_prefix" mapstructure:"file_prefix"`                           // Prefix for migration files (e.g., timestamp format)
	SnapshotFile           string   `yaml:"snapshot_file" mapstructure:"snapshot_file"`                       // Name of the schema snapshot file
	AutoApply              bool     `yaml:"auto_apply" mapstructure:"auto_apply"`                             // Whether to auto-apply migrations (dangerous!)
	IncludeDownSQL         bool     `yaml:"include_down_sql" mapstructure:"include_down_sql"`                 // Whether to generate DOWN migrations
	ReviewCommentPrefix    string   `yaml:"review_comment_prefix" mapstructure:"review_comment_prefix"`       // Prefix for review comments on destructive operations
	DestructiveOperations  []string `yaml:"destructive_operations" mapstructure:"destructive_operations"`     // List of operation types to mark with review comments
	Silent                 bool     `yaml:"silent" mapstructure:"silent"`                                     // Whether to skip prompts for destructive operations
	RejectionCommentPrefix string   `yaml:"rejection_comment_prefix" mapstructure:"rejection_comment_prefix"` // Prefix for rejected destructive operations
}

// SchemaConfig contains schema scanning and processing settings
type SchemaConfig struct {
	SearchPaths    []string `yaml:"search_paths" mapstructure:"search_paths"`         // Additional paths to search for schema files
	IgnoreModules  []string `yaml:"ignore_modules" mapstructure:"ignore_modules"`     // Module patterns to ignore
	SchemaFileName string   `yaml:"schema_file_name" mapstructure:"schema_file_name"` // Name of schema files to look for
	ValidateStrict bool     `yaml:"validate_strict" mapstructure:"validate_strict"`   // Whether to use strict validation
}

// OutputConfig contains output formatting settings
type OutputConfig struct {
	Verbose         bool   `yaml:"verbose" mapstructure:"verbose"`                   // Enable verbose output
	ColorEnabled    bool   `yaml:"color_enabled" mapstructure:"color_enabled"`       // Enable colored output
	ProgressBar     bool   `yaml:"progress_bar" mapstructure:"progress_bar"`         // Show progress bars
	TimestampFormat string `yaml:"timestamp_format" mapstructure:"timestamp_format"` // Format for timestamps in output
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type:             "postgresql",
			DefaultSchema:    "public",
			QuoteIdentifiers: true,
		},
		Migration: MigrationConfig{
			Directory:           "migrations",
			FilePrefix:          "20060102150405", // Go timestamp format for YYYYMMDDHHMMSS
			SnapshotFile:        ".schema_snapshot.yaml",
			AutoApply:           false,
			IncludeDownSQL:      true,
			ReviewCommentPrefix: "-- REVIEW: ",
			DestructiveOperations: []string{
				"table_removed",
				"field_removed",
				"index_removed",
				"table_renamed",
				"field_renamed",
				"field_modified",
			},
			Silent:                 false,
			RejectionCommentPrefix: "-- REJECTED: ",
		},
		Schema: SchemaConfig{
			SearchPaths:    []string{},
			IgnoreModules:  []string{},
			SchemaFileName: "schema.yaml",
			ValidateStrict: false,
		},
		Output: OutputConfig{
			Verbose:         false,
			ColorEnabled:    true,
			ProgressBar:     false,
			TimestampFormat: "2006-01-02 15:04:05",
		},
	}
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set up environment variable binding
	v.SetEnvPrefix("MAKEMIGRATIONS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	cfg := DefaultConfig()
	setDefaults(v, cfg)

	// Try to read config file if it exists
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in migrations directory
		v.SetConfigName("makemigrations.config")
		v.SetConfigType("yaml")
		v.AddConfigPath("migrations")
		v.AddConfigPath(".")
	}

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we'll use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into our config struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// LoadOrDefault loads configuration or returns default if not found
func LoadOrDefault(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// Save saves the configuration to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := `# Makemigrations Configuration File
# 
# This file contains configuration for the makemigrations tool.
# All settings can be overridden using environment variables with the prefix MAKEMIGRATIONS_
# For example: MAKEMIGRATIONS_DATABASE_TYPE=mysql
#
# For nested values, use underscores: MAKEMIGRATIONS_OUTPUT_COLOR_ENABLED=false
#
# Available destructive operation types for review comments:
#   - table_removed: Dropping an entire table
#   - field_removed: Removing a column from a table
#   - index_removed: Removing an index
#   - table_renamed: Renaming a table (data preserved but references may break)
#   - field_renamed: Renaming a column (data preserved but references may break)
#   - field_modified: Changing a column's data type (may cause data loss)
#

`

	// Write to file
	fullContent := []byte(header + string(data))
	if err := os.WriteFile(path, fullContent, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// setDefaults sets default values in viper
func setDefaults(v *viper.Viper, cfg *Config) {
	// Database defaults
	v.SetDefault("database.type", cfg.Database.Type)
	v.SetDefault("database.default_schema", cfg.Database.DefaultSchema)
	v.SetDefault("database.quote_identifiers", cfg.Database.QuoteIdentifiers)

	// Migration defaults
	v.SetDefault("migration.directory", cfg.Migration.Directory)
	v.SetDefault("migration.file_prefix", cfg.Migration.FilePrefix)
	v.SetDefault("migration.snapshot_file", cfg.Migration.SnapshotFile)
	v.SetDefault("migration.auto_apply", cfg.Migration.AutoApply)
	v.SetDefault("migration.include_down_sql", cfg.Migration.IncludeDownSQL)
	v.SetDefault("migration.review_comment_prefix", cfg.Migration.ReviewCommentPrefix)
	v.SetDefault("migration.destructive_operations", cfg.Migration.DestructiveOperations)
	v.SetDefault("migration.silent", cfg.Migration.Silent)
	v.SetDefault("migration.rejection_comment_prefix", cfg.Migration.RejectionCommentPrefix)

	// Schema defaults
	v.SetDefault("schema.search_paths", cfg.Schema.SearchPaths)
	v.SetDefault("schema.ignore_modules", cfg.Schema.IgnoreModules)
	v.SetDefault("schema.schema_file_name", cfg.Schema.SchemaFileName)
	v.SetDefault("schema.validate_strict", cfg.Schema.ValidateStrict)

	// Output defaults
	v.SetDefault("output.verbose", cfg.Output.Verbose)
	v.SetDefault("output.color_enabled", cfg.Output.ColorEnabled)
	v.SetDefault("output.progress_bar", cfg.Output.ProgressBar)
	v.SetDefault("output.timestamp_format", cfg.Output.TimestampFormat)
}

// GetConfigPath returns the default config file path
func GetConfigPath() string {
	return filepath.Join("migrations", "makemigrations.config.yaml")
}

// ConfigExists checks if a config file exists
func ConfigExists() bool {
	_, err := os.Stat(GetConfigPath())
	return err == nil
}
