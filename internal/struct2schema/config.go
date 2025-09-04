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
package struct2schema

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration for struct2schema conversion
type Config struct {
	TypeMappings   map[string]string            `yaml:"type_mappings"`
	CustomDefaults map[string]map[string]string `yaml:"custom_defaults"`
	TableNaming    TableNamingConfig            `yaml:"table_naming"`
}

// TableNamingConfig configures how table names are generated
type TableNamingConfig struct {
	ConvertCase string `yaml:"convert_case"` // snake_case, camelCase, PascalCase
	Prefix      string `yaml:"prefix"`
	Suffix      string `yaml:"suffix"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Set defaults
	if config.TableNaming.ConvertCase == "" {
		config.TableNaming.ConvertCase = "snake_case"
	}

	if config.TypeMappings == nil {
		config.TypeMappings = make(map[string]string)
	}

	return &config, nil
}

// GetDefaultConfig returns a default configuration
func GetDefaultConfig() *Config {
	return &Config{
		TypeMappings: map[string]string{
			// Custom type mappings can be added here
		},
		CustomDefaults: map[string]map[string]string{
			"postgresql": {
				"blank":        "''",
				"array":        "'[]'::jsonb",
				"object":       "'{}'::jsonb",
				"zero":         "0",
				"current_time": "CURRENT_TIME",
				"new_uuid":     "gen_random_uuid()",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "CURRENT_DATE",
				"false":        "false",
				"null":         "null",
				"true":         "true",
			},
			"mysql": {
				"blank":        "''",
				"array":        "('[]')",
				"object":       "('{}')",
				"zero":         "0",
				"current_time": "(CURTIME())",
				"new_uuid":     "(UUID())",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "(CURDATE())",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
			"sqlite": {
				"blank":        "''",
				"array":        "'[]'",
				"object":       "'{}'",
				"zero":         "0",
				"current_time": "CURRENT_TIME",
				"new_uuid":     "",
				"now":          "CURRENT_TIMESTAMP",
				"today":        "CURRENT_DATE",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
			"sqlserver": {
				"blank":        "''",
				"array":        "'[]'",
				"object":       "'{}'",
				"zero":         "0",
				"current_time": "CAST(GETDATE() AS TIME)",
				"new_uuid":     "NEWID()",
				"now":          "GETDATE()",
				"today":        "CAST(GETDATE() AS DATE)",
				"false":        "0",
				"null":         "null",
				"true":         "1",
			},
		},
		TableNaming: TableNamingConfig{
			ConvertCase: "snake_case",
			Prefix:      "",
			Suffix:      "",
		},
	}
}

// SaveConfig saves configuration to a YAML file
func (c *Config) SaveConfig(filename string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddTypeMapping adds a custom type mapping
func (c *Config) AddTypeMapping(goType, sqlType string) {
	if c.TypeMappings == nil {
		c.TypeMappings = make(map[string]string)
	}
	c.TypeMappings[goType] = sqlType
}

// GetTypeMapping gets a custom type mapping
func (c *Config) GetTypeMapping(goType string) (string, bool) {
	if c.TypeMappings == nil {
		return "", false
	}
	sqlType, exists := c.TypeMappings[goType]
	return sqlType, exists
}

// AddCustomDefault adds a custom default value for a database
func (c *Config) AddCustomDefault(database, key, value string) {
	if c.CustomDefaults == nil {
		c.CustomDefaults = make(map[string]map[string]string)
	}
	if c.CustomDefaults[database] == nil {
		c.CustomDefaults[database] = make(map[string]string)
	}
	c.CustomDefaults[database][key] = value
}

// GetCustomDefault gets a custom default value
func (c *Config) GetCustomDefault(database, key string) (string, bool) {
	if c.CustomDefaults == nil {
		return "", false
	}
	if c.CustomDefaults[database] == nil {
		return "", false
	}
	value, exists := c.CustomDefaults[database][key]
	return value, exists
}
