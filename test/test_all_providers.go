package main

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers/auroradsql"
	"github.com/ocomsoft/makemigrations/internal/providers/clickhouse"
	"github.com/ocomsoft/makemigrations/internal/providers/mysql"
	"github.com/ocomsoft/makemigrations/internal/providers/postgresql"
	"github.com/ocomsoft/makemigrations/internal/providers/redshift"
	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/internal/providers/sqlserver"
	"github.com/ocomsoft/makemigrations/internal/providers/starrocks"
	"github.com/ocomsoft/makemigrations/internal/providers/tidb"
	"github.com/ocomsoft/makemigrations/internal/providers/turso"
	"github.com/ocomsoft/makemigrations/internal/providers/vertica"
	"github.com/ocomsoft/makemigrations/internal/providers/ydb"
	"github.com/ocomsoft/makemigrations/internal/types"
)

func main() {
	// Test schema with defaults
	schema := &types.Schema{
		Database: types.Database{
			Name:    "test_defaults",
			Version: "1.0.0",
		},
		Defaults: types.Defaults{
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
			MySQL: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "(CURDATE())",
				"new_uuid": "(UUID())",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
			},
			SQLServer: map[string]string{
				"now":      "GETDATE()",
				"today":    "CAST(GETDATE() AS DATE)",
				"new_uuid": "NEWID()",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
			},
			SQLite: map[string]string{
				"now":      "CURRENT_TIMESTAMP",
				"today":    "CURRENT_DATE",
				"new_uuid": "''",
				"true":     "1",
				"false":    "0",
				"zero":     "0",
				"blank":    "''",
				"null":     "null",
			},
		},
		Tables: []types.Table{
			{
				Name: "test_table",
				Fields: []types.Field{
					{
						Name:       "id",
						Type:       "serial",
						PrimaryKey: true,
					},
					{
						Name:     "int_field",
						Type:     "integer",
						Nullable: &[]bool{false}[0],
						Default:  "zero",
					},
					{
						Name:     "bool_field",
						Type:     "boolean",
						Nullable: &[]bool{false}[0],
						Default:  "true",
					},
					{
						Name:     "timestamp_field",
						Type:     "timestamp",
						Nullable: &[]bool{false}[0],
						Default:  "now",
					},
					{
						Name:     "uuid_field",
						Type:     "uuid",
						Nullable: &[]bool{false}[0],
						Default:  "new_uuid",
					},
					{
						Name:     "literal_string",
						Type:     "varchar",
						Length:   100,
						Nullable: &[]bool{false}[0],
						Default:  "hello",
					},
				},
			},
		},
	}

	// List of providers to test
	providers := map[string]interface{}{
		"PostgreSQL": postgresql.New(),
		"MySQL":      mysql.New(),
		"SQLite":     sqlite.New(),
		"SQL Server": sqlserver.New(),
		"StarRocks":  starrocks.New(),
		"Redshift":   redshift.New(),
		"AuroraDS":   auroradsql.New(),
		"TiDB":       tidb.New(),
		"Vertica":    vertica.New(),
		"Turso":      turso.New(),
		"ClickHouse": clickhouse.New(),
		"YDB":        ydb.New(),
	}

	fmt.Println("Testing default value conversion across all providers:")
	fmt.Println(strings.Repeat("=", 60))

	for name, provider := range providers {
		fmt.Printf("\n🔧 Testing %s provider:\n", name)

		// Type assertion to get the provider interface
		if p, ok := provider.(interface {
			GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error)
		}); ok {
			sql, err := p.GenerateCreateTable(schema, &schema.Tables[0])
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}

			fmt.Printf("✅ Generated SQL:\n")
			fmt.Println(sql)

			// Check for specific default patterns
			checks := []struct {
				pattern     string
				shouldHave  bool
				description string
			}{
				{"DEFAULT zero", false, "Should not have literal 'zero'"},
				{"DEFAULT 0", true, "Should have converted zero to 0"},
				{"DEFAULT now", false, "Should not have literal 'now'"},
				{"DEFAULT 'hello'", true, "Should have quoted string literals"},
				{"DEFAULT hello", false, "Should not have unquoted string literals"},
			}

			for _, check := range checks {
				contains := containsPattern(sql, check.pattern)
				if contains == check.shouldHave {
					if check.shouldHave {
						fmt.Printf("  ✅ %s\n", check.description)
					} else {
						fmt.Printf("  ✅ %s\n", check.description)
					}
				} else {
					if check.shouldHave {
						fmt.Printf("  ❌ %s (missing: %s)\n", check.description, check.pattern)
					} else {
						fmt.Printf("  ❌ %s (found: %s)\n", check.description, check.pattern)
					}
				}
			}
		} else {
			fmt.Printf("❌ Provider does not implement GenerateCreateTable\n")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Default value conversion test completed!")
}

func containsPattern(text, pattern string) bool {
	return len(pattern) > 0 && len(text) >= len(pattern) &&
		func() bool {
			for i := 0; i <= len(text)-len(pattern); i++ {
				if text[i:i+len(pattern)] == pattern {
					return true
				}
			}
			return false
		}()
}
