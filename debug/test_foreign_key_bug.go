package main

import (
	"fmt"
	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/yaml"
)

func boolPtr(b bool) *bool {
	return &b
}

func main() {
	schema := &types.Schema{
		Tables: []types.Table{
			{
				Name: "fs_meta_data",
				Fields: []types.Field{
					{
						Name:       "id",
						Type:       "uuid",
						PrimaryKey: true,
						Nullable:   boolPtr(false),
					},
					{
						Name:     "original_filename",
						Type:     "varchar",
						Nullable: boolPtr(false),
						Length:   255,
					},
				},
			},
			{
				Name: "category",
				Fields: []types.Field{
					{
						Name:       "id",
						Type:       "serial",
						PrimaryKey: true,
					},
					{
						Name:     "name",
						Type:     "varchar",
						Nullable: boolPtr(false),
						Length:   255,
					},
				},
			},
			{
				Name: "misc_model",
				Fields: []types.Field{
					{
						Name:       "id",
						Type:       "uuid",
						PrimaryKey: true,
						Default:    "new_uuid",
					},
					{
						Name:     "description",
						Type:     "varchar",
						Nullable: boolPtr(false),
						Length:   255,
					},
					{
						Name: "a_file_id",
						Type: "foreign_key",
						ForeignKey: &types.ForeignKey{
							Table:    "fs_meta_data",
							OnDelete: "PROTECT",
						},
					},
					{
						Name: "category_id",
						Type: "foreign_key",
						ForeignKey: &types.ForeignKey{
							Table:    "category",
							OnDelete: "CASCADE",
						},
					},
				},
			},
		},
	}

	converter := yaml.NewSQLConverter(yaml.DatabasePostgreSQL, true)
	sql, err := converter.ConvertSchema(schema)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated SQL:\n%s\n", sql)
}
