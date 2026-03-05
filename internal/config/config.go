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

	// Output settings
	Output OutputConfig `yaml:"output" mapstructure:"output"`
}

// DatabaseConfig contains database-related settings
type DatabaseConfig struct {
	Type string `yaml:"type" mapstructure:"type"` // postgresql, mysql, sqlserver, sqlite
}

// MigrationConfig contains migration-related settings
type MigrationConfig struct {
	Directory string `yaml:"directory" mapstructure:"directory"` // Directory for migration files
}

// OutputConfig contains output formatting settings
type OutputConfig struct {
	Verbose      bool `yaml:"verbose" mapstructure:"verbose"`           // Enable verbose output
	ColorEnabled bool `yaml:"color_enabled" mapstructure:"color_enabled"` // Enable colored output
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type: "postgresql",
		},
		Migration: MigrationConfig{
			Directory: "migrations",
		},
		Output: OutputConfig{
			Verbose:      false,
			ColorEnabled: true,
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
	v.SetDefault("database.type", cfg.Database.Type)
	v.SetDefault("migration.directory", cfg.Migration.Directory)
	v.SetDefault("output.verbose", cfg.Output.Verbose)
	v.SetDefault("output.color_enabled", cfg.Output.ColorEnabled)
}

// GetConfigPath returns the default config file path
func GetConfigPath() string {
	return filepath.Join("migrations", "makemigrations.config.yaml")
}

