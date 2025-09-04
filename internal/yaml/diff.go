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
package yaml

import (
	"fmt"
	"strings"
)

// DiffEngine handles YAML schema comparison and diff generation
type DiffEngine struct {
	verbose bool
}

// NewDiffEngine creates a new YAML diff engine
func NewDiffEngine(verbose bool) *DiffEngine {
	return &DiffEngine{
		verbose: verbose,
	}
}

// Change represents a structural change between schemas
type Change struct {
	Type        ChangeType  `json:"type"`
	TableName   string      `json:"table_name"`
	FieldName   string      `json:"field_name,omitempty"`
	Description string      `json:"description"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	Destructive bool        `json:"destructive"`
}

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeTypeTableAdded    ChangeType = "table_added"
	ChangeTypeTableRemoved  ChangeType = "table_removed"
	ChangeTypeTableRenamed  ChangeType = "table_renamed"
	ChangeTypeFieldAdded    ChangeType = "field_added"
	ChangeTypeFieldRemoved  ChangeType = "field_removed"
	ChangeTypeFieldRenamed  ChangeType = "field_renamed"
	ChangeTypeFieldModified ChangeType = "field_modified"
	ChangeTypeIndexAdded    ChangeType = "index_added"
	ChangeTypeIndexRemoved  ChangeType = "index_removed"
)

// SchemaDiff represents the complete difference between two schemas
type SchemaDiff struct {
	Changes       []Change `json:"changes"`
	HasChanges    bool     `json:"has_changes"`
	IsDestructive bool     `json:"is_destructive"`
}

// CompareSchemas compares two YAML schemas and returns the differences
func (de *DiffEngine) CompareSchemas(oldSchema, newSchema *Schema) (*SchemaDiff, error) {
	if de.verbose {
		oldTableCount := 0
		newTableCount := 0
		if oldSchema != nil {
			oldTableCount = len(oldSchema.Tables)
		}
		if newSchema != nil {
			newTableCount = len(newSchema.Tables)
		}
		fmt.Printf("Comparing schemas: %d -> %d tables\n", oldTableCount, newTableCount)
	}

	diff := &SchemaDiff{
		Changes:    make([]Change, 0),
		HasChanges: false,
	}

	// Handle case where old schema is nil (initial migration)
	if oldSchema == nil {
		if newSchema != nil {
			for _, table := range newSchema.Tables {
				diff.Changes = append(diff.Changes, Change{
					Type:        ChangeTypeTableAdded,
					TableName:   table.Name,
					Description: fmt.Sprintf("Add table '%s'", table.Name),
					NewValue:    table,
				})
			}
		}
		diff.HasChanges = len(diff.Changes) > 0
		return diff, nil
	}

	// Handle case where new schema is nil (shouldn't happen normally)
	if newSchema == nil {
		for _, table := range oldSchema.Tables {
			diff.Changes = append(diff.Changes, Change{
				Type:        ChangeTypeTableRemoved,
				TableName:   table.Name,
				Description: fmt.Sprintf("Remove table '%s'", table.Name),
				OldValue:    table,
				Destructive: true,
			})
		}
		diff.HasChanges = len(diff.Changes) > 0
		diff.IsDestructive = true
		return diff, nil
	}

	// Compare tables
	oldTables := make(map[string]*Table)
	newTables := make(map[string]*Table)

	for i := range oldSchema.Tables {
		oldTables[oldSchema.Tables[i].Name] = &oldSchema.Tables[i]
	}
	for i := range newSchema.Tables {
		newTables[newSchema.Tables[i].Name] = &newSchema.Tables[i]
	}

	// Find added tables
	for tableName, newTable := range newTables {
		if _, exists := oldTables[tableName]; !exists {
			diff.Changes = append(diff.Changes, Change{
				Type:        ChangeTypeTableAdded,
				TableName:   tableName,
				Description: fmt.Sprintf("Add table '%s'", tableName),
				NewValue:    *newTable,
			})
			if de.verbose {
				fmt.Printf("Table added: %s\n", tableName)
			}
		}
	}

	// Find removed tables
	for tableName, oldTable := range oldTables {
		if _, exists := newTables[tableName]; !exists {
			diff.Changes = append(diff.Changes, Change{
				Type:        ChangeTypeTableRemoved,
				TableName:   tableName,
				Description: fmt.Sprintf("Remove table '%s'", tableName),
				OldValue:    *oldTable,
				Destructive: true,
			})
			diff.IsDestructive = true
			if de.verbose {
				fmt.Printf("Table removed: %s\n", tableName)
			}
		}
	}

	// Find modified tables
	for tableName, newTable := range newTables {
		if oldTable, exists := oldTables[tableName]; exists {
			tableChanges, err := de.compareTablesForChanges(oldTable, newTable)
			if err != nil {
				return nil, fmt.Errorf("failed to compare table %s: %w", tableName, err)
			}

			for _, change := range tableChanges {
				if change.Destructive {
					diff.IsDestructive = true
				}
			}
			diff.Changes = append(diff.Changes, tableChanges...)
		}
	}

	diff.HasChanges = len(diff.Changes) > 0

	if de.verbose {
		fmt.Printf("Schema comparison completed: %d changes found (destructive: %v)\n",
			len(diff.Changes), diff.IsDestructive)
	}

	return diff, nil
}

// compareTablesForChanges compares two tables and returns the field-level changes
func (de *DiffEngine) compareTablesForChanges(oldTable, newTable *Table) ([]Change, error) {
	var changes []Change

	// Compare fields
	oldFields := make(map[string]*Field)
	newFields := make(map[string]*Field)

	for i := range oldTable.Fields {
		oldFields[oldTable.Fields[i].Name] = &oldTable.Fields[i]
	}
	for i := range newTable.Fields {
		newFields[newTable.Fields[i].Name] = &newTable.Fields[i]
	}

	// Find added fields
	for fieldName, newField := range newFields {
		if _, exists := oldFields[fieldName]; !exists {
			changes = append(changes, Change{
				Type:        ChangeTypeFieldAdded,
				TableName:   newTable.Name,
				FieldName:   fieldName,
				Description: fmt.Sprintf("Add field '%s.%s'", newTable.Name, fieldName),
				NewValue:    *newField,
			})
			if de.verbose {
				fmt.Printf("  Field added: %s.%s\n", newTable.Name, fieldName)
			}
		}
	}

	// Find removed fields
	for fieldName, oldField := range oldFields {
		if _, exists := newFields[fieldName]; !exists {
			changes = append(changes, Change{
				Type:        ChangeTypeFieldRemoved,
				TableName:   newTable.Name,
				FieldName:   fieldName,
				Description: fmt.Sprintf("Remove field '%s.%s'", newTable.Name, fieldName),
				OldValue:    *oldField,
				Destructive: true,
			})
			if de.verbose {
				fmt.Printf("  Field removed: %s.%s\n", newTable.Name, fieldName)
			}
		}
	}

	// Find modified fields
	for fieldName, newField := range newFields {
		if oldField, exists := oldFields[fieldName]; exists {
			fieldChanges := de.compareFieldsForChanges(oldTable.Name, oldField, newField)
			changes = append(changes, fieldChanges...)
		}
	}

	// Compare indexes
	indexChanges := de.compareIndexes(oldTable, newTable)
	changes = append(changes, indexChanges...)

	return changes, nil
}

// compareFieldsForChanges compares two fields and returns the property-level changes
func (de *DiffEngine) compareFieldsForChanges(tableName string, oldField, newField *Field) []Change {
	var changes []Change

	// Type changes
	if oldField.Type != newField.Type {
		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' type from %s to %s", tableName, oldField.Name, oldField.Type, newField.Type),
			OldValue:    oldField.Type,
			NewValue:    newField.Type,
			Destructive: de.isTypeChangeDestructive(oldField.Type, newField.Type),
		})
	}

	// Length changes
	if oldField.Length != newField.Length && (oldField.Type == "varchar" || oldField.Type == "text") {
		destructive := newField.Length < oldField.Length
		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' length from %d to %d", tableName, oldField.Name, oldField.Length, newField.Length),
			OldValue:    oldField.Length,
			NewValue:    newField.Length,
			Destructive: destructive,
		})
	}

	// Precision/Scale changes for decimal fields
	if oldField.Type == "decimal" && (oldField.Precision != newField.Precision || oldField.Scale != newField.Scale) {
		destructive := newField.Precision < oldField.Precision || newField.Scale < oldField.Scale
		changes = append(changes, Change{
			Type:      ChangeTypeFieldModified,
			TableName: tableName,
			FieldName: oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' precision/scale from %d,%d to %d,%d",
				tableName, oldField.Name, oldField.Precision, oldField.Scale, newField.Precision, newField.Scale),
			OldValue:    fmt.Sprintf("%d,%d", oldField.Precision, oldField.Scale),
			NewValue:    fmt.Sprintf("%d,%d", newField.Precision, newField.Scale),
			Destructive: destructive,
		})
	}

	// Nullable changes
	if oldField.IsNullable() != newField.IsNullable() {
		destructive := !newField.IsNullable() // Making a field NOT NULL is potentially destructive
		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' nullable from %v to %v", tableName, oldField.Name, oldField.IsNullable(), newField.IsNullable()),
			OldValue:    oldField.IsNullable(),
			NewValue:    newField.IsNullable(),
			Destructive: destructive,
		})
	}

	// Primary key changes
	if oldField.PrimaryKey != newField.PrimaryKey {
		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' primary key from %v to %v", tableName, oldField.Name, oldField.PrimaryKey, newField.PrimaryKey),
			OldValue:    oldField.PrimaryKey,
			NewValue:    newField.PrimaryKey,
			Destructive: !newField.PrimaryKey, // Removing primary key is destructive
		})
	}

	// Default value changes
	if oldField.Default != newField.Default {
		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' default from '%s' to '%s'", tableName, oldField.Name, oldField.Default, newField.Default),
			OldValue:    oldField.Default,
			NewValue:    newField.Default,
		})
	}

	// Foreign key changes
	if !de.compareForeignKeys(oldField.ForeignKey, newField.ForeignKey) {
		oldFK := "none"
		newFK := "none"
		if oldField.ForeignKey != nil {
			oldFK = oldField.ForeignKey.Table
		}
		if newField.ForeignKey != nil {
			newFK = newField.ForeignKey.Table
		}

		changes = append(changes, Change{
			Type:        ChangeTypeFieldModified,
			TableName:   tableName,
			FieldName:   oldField.Name,
			Description: fmt.Sprintf("Change field '%s.%s' foreign key from %s to %s", tableName, oldField.Name, oldFK, newFK),
			OldValue:    oldField.ForeignKey,
			NewValue:    newField.ForeignKey,
			Destructive: newField.ForeignKey == nil && oldField.ForeignKey != nil, // Removing FK is destructive
		})
	}

	if de.verbose && len(changes) > 0 {
		fmt.Printf("  Field modified: %s.%s (%d property changes)\n", tableName, oldField.Name, len(changes))
	}

	return changes
}

// isTypeChangeDestructive determines if a type change is destructive
func (de *DiffEngine) isTypeChangeDestructive(oldType, newType string) bool {
	// Safe promotions (non-destructive)
	safePromotions := map[string][]string{
		"integer": {"bigint"},
		"varchar": {"text"},
		"float":   {"decimal"},
	}

	if promotions, exists := safePromotions[oldType]; exists {
		for _, promotion := range promotions {
			if promotion == newType {
				return false
			}
		}
	}

	// Any other type change is considered destructive
	return oldType != newType
}

// compareForeignKeys compares two foreign key definitions
func (de *DiffEngine) compareForeignKeys(fk1, fk2 *ForeignKey) bool {
	if fk1 == nil && fk2 == nil {
		return true
	}
	if fk1 == nil || fk2 == nil {
		return false
	}
	return fk1.Table == fk2.Table &&
		fk1.OnDelete == fk2.OnDelete
}

// GenerateMigrationName generates a meaningful migration name from changes
func (de *DiffEngine) GenerateMigrationName(diff *SchemaDiff) string {
	if !diff.HasChanges {
		return "no_changes"
	}

	if len(diff.Changes) == 1 {
		change := diff.Changes[0]
		switch change.Type {
		case ChangeTypeTableAdded:
			return fmt.Sprintf("add_%s_table", change.TableName)
		case ChangeTypeTableRemoved:
			return fmt.Sprintf("remove_%s_table", change.TableName)
		case ChangeTypeFieldAdded:
			return fmt.Sprintf("add_%s_to_%s", change.FieldName, change.TableName)
		case ChangeTypeFieldRemoved:
			return fmt.Sprintf("remove_%s_from_%s", change.FieldName, change.TableName)
		case ChangeTypeFieldModified:
			return fmt.Sprintf("modify_%s_in_%s", change.FieldName, change.TableName)
		}
	}

	// Multiple changes - categorize
	tableChanges := 0
	fieldChanges := 0

	for _, change := range diff.Changes {
		if strings.Contains(string(change.Type), "table") {
			tableChanges++
		} else {
			fieldChanges++
		}
	}

	if tableChanges > 0 && fieldChanges > 0 {
		return "update_schema"
	} else if tableChanges > 0 {
		if tableChanges == 1 {
			return "modify_table"
		}
		return "modify_tables"
	} else {
		if fieldChanges == 1 {
			return "modify_field"
		}
		return "modify_fields"
	}
}

// HasDestructiveChanges checks if the diff contains destructive changes
func (de *DiffEngine) HasDestructiveChanges(diff *SchemaDiff) bool {
	return diff.IsDestructive
}

// GetChangesByType returns changes filtered by type
func (de *DiffEngine) GetChangesByType(diff *SchemaDiff, changeType ChangeType) []Change {
	var result []Change
	for _, change := range diff.Changes {
		if change.Type == changeType {
			result = append(result, change)
		}
	}
	return result
}

// GetDestructiveChanges returns only destructive changes
func (de *DiffEngine) GetDestructiveChanges(diff *SchemaDiff) []Change {
	var result []Change
	for _, change := range diff.Changes {
		if change.Destructive {
			result = append(result, change)
		}
	}
	return result
}

// compareIndexes compares indexes between two tables
func (de *DiffEngine) compareIndexes(oldTable, newTable *Table) []Change {
	var changes []Change

	// Create maps for easier comparison
	oldIndexes := make(map[string]*Index)
	newIndexes := make(map[string]*Index)

	for i := range oldTable.Indexes {
		oldIndexes[oldTable.Indexes[i].Name] = &oldTable.Indexes[i]
	}
	for i := range newTable.Indexes {
		newIndexes[newTable.Indexes[i].Name] = &newTable.Indexes[i]
	}

	// Find added indexes
	for indexName, newIndex := range newIndexes {
		if _, exists := oldIndexes[indexName]; !exists {
			indexType := "index"
			if newIndex.Unique {
				indexType = "unique index"
			}
			changes = append(changes, Change{
				Type:        ChangeTypeIndexAdded,
				TableName:   newTable.Name,
				FieldName:   indexName,
				Description: fmt.Sprintf("Add %s '%s' on table '%s'", indexType, indexName, newTable.Name),
				NewValue:    *newIndex,
			})
			if de.verbose {
				fmt.Printf("  Index added: %s on %s\n", indexName, newTable.Name)
			}
		}
	}

	// Find removed indexes
	for indexName, oldIndex := range oldIndexes {
		if _, exists := newIndexes[indexName]; !exists {
			indexType := "index"
			if oldIndex.Unique {
				indexType = "unique index"
			}
			changes = append(changes, Change{
				Type:        ChangeTypeIndexRemoved,
				TableName:   newTable.Name,
				FieldName:   indexName,
				Description: fmt.Sprintf("Remove %s '%s' from table '%s'", indexType, indexName, newTable.Name),
				OldValue:    *oldIndex,
				Destructive: true,
			})
			if de.verbose {
				fmt.Printf("  Index removed: %s from %s\n", indexName, newTable.Name)
			}
		}
	}

	// Find modified indexes (compare fields and unique property)
	for indexName, newIndex := range newIndexes {
		if oldIndex, exists := oldIndexes[indexName]; exists {
			if !isIndexEqual(oldIndex, newIndex) {
				changes = append(changes, Change{
					Type:        ChangeTypeIndexRemoved,
					TableName:   newTable.Name,
					FieldName:   indexName,
					Description: fmt.Sprintf("Remove index '%s' from table '%s' (will be recreated)", indexName, newTable.Name),
					OldValue:    *oldIndex,
					Destructive: true,
				})
				changes = append(changes, Change{
					Type:        ChangeTypeIndexAdded,
					TableName:   newTable.Name,
					FieldName:   indexName,
					Description: fmt.Sprintf("Recreate index '%s' on table '%s' with new definition", indexName, newTable.Name),
					NewValue:    *newIndex,
				})
				if de.verbose {
					fmt.Printf("  Index modified: %s on %s\n", indexName, newTable.Name)
				}
			}
		}
	}

	return changes
}

// isIndexEqual compares two index definitions
func isIndexEqual(idx1, idx2 *Index) bool {
	if idx1.Unique != idx2.Unique {
		return false
	}
	if len(idx1.Fields) != len(idx2.Fields) {
		return false
	}
	for i, field := range idx1.Fields {
		if field != idx2.Fields[i] {
			return false
		}
	}
	return true
}
