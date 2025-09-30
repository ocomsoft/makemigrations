package main

import (
	"fmt"

	"github.com/ocomsoft/makemigrations/internal/yaml"
)

func main() {
	// Create a schema with defaults mapping
	schema := &yaml.Schema{
		Defaults: yaml.Defaults{
			PostgreSQL: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "CURRENT_DATE",
				"new_uuid": "gen_random_uuid()",
				"true":     "true",
				"false":    "false",
				"zero":     "0",
				"blank":    "''",
				"null":     "NULL",
			},
		},
	}

	converter := yaml.NewSQLConverter(yaml.DatabasePostgreSQL, false)

	// Test cases that should use defaults mapping
	testCases := []string{
		"now",    // Should return CURRENT_TIMESTAMP
		"true",   // Should return true (not 'true')
		"false",  // Should return false (not 'false')
		"zero",   // Should return 0 (not '0')
		"42",     // Should return 42 (not '42') - numeric literal
		"custom", // Should return 'custom' - string literal
	}

	for _, testCase := range testCases {
		result, err := converter.ConvertDefaultValue(schema, testCase)
		if err != nil {
			fmt.Printf("Error for '%s': %v\n", testCase, err)
		} else {
			fmt.Printf("Default for '%s': %s\n", testCase, result)
		}
	}
}
