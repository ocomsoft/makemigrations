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
	"path/filepath"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/types"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

var diffVerbose bool

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show schema drift between YAML and migration state as YAML",
	Long: `Compare the current YAML schema files against the migration DAG state
and output the differences as YAML. The output shows:

  add:    - tables, fields, and indexes in schema.yaml but not yet in migrations
  remove: - tables, fields, and indexes in migrations but removed from schema.yaml
  modify: - fields whose properties changed between migration state and schema.yaml

This helps you understand exactly what a new migration would do, expressed
in the same YAML format as your schema files.`,
	RunE: runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().BoolVar(&diffVerbose, "verbose", false, "Show detailed output")
}

func runDiff(_ *cobra.Command, _ []string) error {
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory

	// 1. Query existing DAG for previous schema state
	var prevSchema *yamlpkg.Schema

	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	if len(migFiles) > 0 {
		dagOut, err := queryDAG(migrationsDir, diffVerbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		prevSchema = schemaStateToYAMLSchema(dagOut.SchemaState, cfg.Database.Type)
	}

	// 2. Parse current YAML schema
	dbType, err := yamlpkg.ParseDatabaseType(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}
	components := InitializeYAMLComponents(dbType, diffVerbose)
	allSchemas, err := ScanAndParseSchemas(components, diffVerbose)
	if err != nil {
		return fmt.Errorf("parsing YAML schema: %w", err)
	}

	currentSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, diffVerbose)
	if err != nil {
		return fmt.Errorf("merging YAML schemas: %w", err)
	}

	// 3. Diff
	diffEngine := yamlpkg.NewDiffEngine(diffVerbose)
	diff, err := diffEngine.CompareSchemas(prevSchema, currentSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	if !diff.HasChanges {
		fmt.Println("# No differences — schema.yaml matches migration state.")
		return nil
	}

	// 4. Build YAML output from changes
	output := buildDiffYAML(diff.Changes)

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshalling diff output: %w", err)
	}

	fmt.Printf("# Schema diff: %d change(s) between YAML schema and migration state\n", len(diff.Changes))
	fmt.Println("# 'add' = in schema.yaml but not yet migrated")
	fmt.Println("# 'remove' = in migration state but removed from schema.yaml")
	fmt.Println("# 'modify' = field properties changed")
	fmt.Print(string(data))

	return nil
}

// diffOutput is the top-level YAML structure for the diff command.
type diffOutput struct {
	Add    *diffSection   `yaml:"add,omitempty"`
	Remove *diffSection   `yaml:"remove,omitempty"`
	Modify []diffModified `yaml:"modify,omitempty"`
}

// diffSection holds tables, fields, and indexes for add or remove.
type diffSection struct {
	Tables  []types.Table `yaml:"tables,omitempty"`
	Fields  []diffField   `yaml:"fields,omitempty"`
	Indexes []diffIndex   `yaml:"indexes,omitempty"`
}

// diffField represents a field change scoped to a table.
type diffField struct {
	Table string      `yaml:"table"`
	Field types.Field `yaml:"field"`
}

// diffIndex represents an index change scoped to a table.
type diffIndex struct {
	Table string      `yaml:"table"`
	Index types.Index `yaml:"index"`
}

// diffModified represents a field whose properties changed.
type diffModified struct {
	Table string      `yaml:"table"`
	Field string      `yaml:"field"`
	From  types.Field `yaml:"from"`
	To    types.Field `yaml:"to"`
}

func buildDiffYAML(changes []yamlpkg.Change) diffOutput {
	var out diffOutput
	var addSection, removeSection diffSection

	for _, c := range changes {
		switch c.Type {
		case yamlpkg.ChangeTypeTableAdded:
			if table, ok := c.NewValue.(yamlpkg.Table); ok {
				addSection.Tables = append(addSection.Tables, table)
			}

		case yamlpkg.ChangeTypeTableRemoved:
			if table, ok := c.OldValue.(yamlpkg.Table); ok {
				removeSection.Tables = append(removeSection.Tables, table)
			}

		case yamlpkg.ChangeTypeFieldAdded:
			if field, ok := c.NewValue.(yamlpkg.Field); ok {
				addSection.Fields = append(addSection.Fields, diffField{
					Table: c.TableName,
					Field: field,
				})
			}

		case yamlpkg.ChangeTypeFieldRemoved:
			if field, ok := c.OldValue.(yamlpkg.Field); ok {
				removeSection.Fields = append(removeSection.Fields, diffField{
					Table: c.TableName,
					Field: field,
				})
			}

		case yamlpkg.ChangeTypeIndexAdded:
			if idx, ok := c.NewValue.(yamlpkg.Index); ok {
				addSection.Indexes = append(addSection.Indexes, diffIndex{
					Table: c.TableName,
					Index: idx,
				})
			}

		case yamlpkg.ChangeTypeIndexRemoved:
			if idx, ok := c.OldValue.(yamlpkg.Index); ok {
				removeSection.Indexes = append(removeSection.Indexes, diffIndex{
					Table: c.TableName,
					Index: idx,
				})
			}

		case yamlpkg.ChangeTypeFieldModified:
			oldField, oldOk := c.OldValue.(yamlpkg.Field)
			newField, newOk := c.NewValue.(yamlpkg.Field)
			if oldOk && newOk {
				out.Modify = append(out.Modify, diffModified{
					Table: c.TableName,
					Field: c.FieldName,
					From:  oldField,
					To:    newField,
				})
			}

		// FK changes are informational — already captured via field/table changes
		case yamlpkg.ChangeTypeForeignKeyAdded, yamlpkg.ChangeTypeForeignKeyRemoved:
			continue
		}
	}

	if len(addSection.Tables) > 0 || len(addSection.Fields) > 0 || len(addSection.Indexes) > 0 {
		out.Add = &addSection
	}
	if len(removeSection.Tables) > 0 || len(removeSection.Fields) > 0 || len(removeSection.Indexes) > 0 {
		out.Remove = &removeSection
	}

	return out
}
