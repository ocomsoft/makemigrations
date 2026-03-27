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
	"sort"
	"strings"
)

// topologicallySortTables returns tables sorted so that a referenced table always
// appears before the table that references it via a foreign_key field.  Only
// intra-set dependencies are considered; references to tables outside the provided
// set (e.g. already-existing tables) are ignored.  If a cycle is detected the
// function falls back to alphabetical order so callers always receive a stable,
// deterministic result.
func topologicallySortTables(tables []Table) []Table {
	// Build a name→index lookup for fast membership tests.
	idx := make(map[string]int, len(tables))
	for i, t := range tables {
		idx[t.Name] = i
	}

	// in-degree count and adjacency list (dependency → dependents).
	inDegree := make([]int, len(tables))
	deps := make([][]int, len(tables)) // deps[i] = tables that i depends on (within the set)

	for i, t := range tables {
		for _, f := range t.Fields {
			if f.Type == "foreign_key" && f.ForeignKey != nil {
				if j, ok := idx[f.ForeignKey.Table]; ok && j != i {
					// table i depends on table j
					deps[i] = append(deps[i], j)
					inDegree[i]++
				}
			}
		}
	}

	// Kahn's algorithm: start with nodes that have no in-set dependencies.
	queue := make([]int, 0, len(tables))
	for i := range tables {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	// Sort initial queue alphabetically for determinism.
	sort.Slice(queue, func(a, b int) bool { return tables[queue[a]].Name < tables[queue[b]].Name })

	// Build reverse adjacency: which tables depend on table j?
	rdeps := make([][]int, len(tables))
	for i, ds := range deps {
		for _, j := range ds {
			rdeps[j] = append(rdeps[j], i)
		}
	}

	sorted := make([]Table, 0, len(tables))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		sorted = append(sorted, tables[cur])

		// Reduce in-degree for all tables that depend on cur.
		next := make([]int, 0)
		for _, dependent := range rdeps[cur] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				next = append(next, dependent)
			}
		}
		sort.Slice(next, func(a, b int) bool { return tables[next[a]].Name < tables[next[b]].Name })
		queue = append(queue, next...)
	}

	// Cycle fallback: append any remaining tables alphabetically.
	if len(sorted) < len(tables) {
		remaining := make([]Table, 0, len(tables)-len(sorted))
		inSorted := make(map[string]bool, len(sorted))
		for _, t := range sorted {
			inSorted[t.Name] = true
		}
		for _, t := range tables {
			if !inSorted[t.Name] {
				remaining = append(remaining, t)
			}
		}
		sort.Slice(remaining, func(a, b int) bool { return remaining[a].Name < remaining[b].Name })
		sorted = append(sorted, remaining...)
	}

	return sorted
}

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
	ChangeTypeTableAdded           ChangeType = "table_added"
	ChangeTypeTableRemoved         ChangeType = "table_removed"
	ChangeTypeTableRenamed         ChangeType = "table_renamed"
	ChangeTypeFieldAdded           ChangeType = "field_added"
	ChangeTypeFieldRemoved         ChangeType = "field_removed"
	ChangeTypeFieldRenamed         ChangeType = "field_renamed"
	ChangeTypeFieldModified        ChangeType = "field_modified"
	ChangeTypeIndexAdded           ChangeType = "index_added"
	ChangeTypeIndexRemoved         ChangeType = "index_removed"
	ChangeTypeForeignKeyAdded      ChangeType = "foreign_key_added"
	ChangeTypeForeignKeyRemoved    ChangeType = "foreign_key_removed"
	ChangeTypeDefaultsModified     ChangeType = "defaults_modified"      // non-destructive: updates active schema defaults
	ChangeTypeTypeMappingsModified ChangeType = "type_mappings_modified" // non-destructive: updates active provider type mappings
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
			// Topologically sort so that referenced tables come before referencing tables,
			// then emit all CreateTable changes first, followed by all FK changes.
			sorted := topologicallySortTables(newSchema.Tables)
			var allFKChanges []Change
			for _, table := range sorted {
				diff.Changes = append(diff.Changes, Change{
					Type:        ChangeTypeTableAdded,
					TableName:   table.Name,
					Description: fmt.Sprintf("Add table '%s'", table.Name),
					NewValue:    table,
				})
				// Collect FK changes to emit after all CreateTable operations so that
				// every referenced table exists before any FK constraint is added.
				allFKChanges = append(allFKChanges, fkChangesForFields(table.Name, table.Fields, nil)...)
			}
			diff.Changes = append(diff.Changes, allFKChanges...)
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

	// Find added tables: collect first, then topologically sort so that
	// referenced tables (even across the new-table set) are created before
	// referencing tables.  All CreateTable changes are emitted before any
	// AddForeignKey change so every referenced table exists when the FK is added.
	var addedTables []Table
	for tableName, newTable := range newTables {
		if _, exists := oldTables[tableName]; !exists {
			addedTables = append(addedTables, *newTable)
		}
	}
	sortedAdded := topologicallySortTables(addedTables)
	var addedFKChanges []Change
	for _, table := range sortedAdded {
		diff.Changes = append(diff.Changes, Change{
			Type:        ChangeTypeTableAdded,
			TableName:   table.Name,
			Description: fmt.Sprintf("Add table '%s'", table.Name),
			NewValue:    table,
		})
		addedFKChanges = append(addedFKChanges, fkChangesForFields(table.Name, table.Fields, nil)...)
		if de.verbose {
			fmt.Printf("Table added: %s\n", table.Name)
		}
	}
	diff.Changes = append(diff.Changes, addedFKChanges...)

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

	// Emit companion FK changes for any foreign_key fields added or removed.
	// We collect the added/removed field slices by re-walking the field maps so
	// that the logic stays self-contained and independent of the loops above.
	var addedFKFields, removedFKFields []Field
	for _, newField := range newTable.Fields {
		found := false
		for _, oldField := range oldTable.Fields {
			if oldField.Name == newField.Name {
				found = true
				break
			}
		}
		if !found {
			addedFKFields = append(addedFKFields, newField)
		}
	}
	for _, oldField := range oldTable.Fields {
		found := false
		for _, newField := range newTable.Fields {
			if newField.Name == oldField.Name {
				found = true
				break
			}
		}
		if !found {
			removedFKFields = append(removedFKFields, oldField)
		}
	}
	changes = append(changes, fkChangesForFields(newTable.Name, addedFKFields, removedFKFields)...)

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

	// Foreign key constraint changes — for foreign_key typed fields the
	// constraint is tracked independently from the column. Emit FK
	// operations (not FieldModified) so the code generator produces
	// AddForeignKey / DropForeignKey rather than an AlterField.
	if !de.compareForeignKeys(oldField.ForeignKey, newField.ForeignKey) {
		if oldField.Type == "foreign_key" || newField.Type == "foreign_key" {
			if oldField.ForeignKey != nil {
				constraintName := fmt.Sprintf("fk_%s_%s", tableName, oldField.Name)
				changes = append(changes, Change{
					Type:        ChangeTypeForeignKeyRemoved,
					TableName:   tableName,
					FieldName:   oldField.Name,
					Description: fmt.Sprintf("Remove foreign key %s from %s.%s", constraintName, tableName, oldField.Name),
					OldValue:    *oldField,
					Destructive: true,
				})
			}
			if newField.ForeignKey != nil {
				constraintName := fmt.Sprintf("fk_%s_%s", tableName, newField.Name)
				changes = append(changes, Change{
					Type:        ChangeTypeForeignKeyAdded,
					TableName:   tableName,
					FieldName:   newField.Name,
					Description: fmt.Sprintf("Add foreign key %s on %s.%s → %s", constraintName, tableName, newField.Name, newField.ForeignKey.Table),
					NewValue:    *newField,
				})
			}
		} else {
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
				Destructive: newField.ForeignKey == nil && oldField.ForeignKey != nil,
			})
		}
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
	if idx1.Method != idx2.Method {
		return false
	}
	if idx1.Where != idx2.Where {
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

// fkChangesForFields emits FK change records for foreign_key fields that are
// present in addedFields (ChangeTypeForeignKeyAdded) or removedFields (ChangeTypeForeignKeyRemoved).
func fkChangesForFields(tableName string, addedFields, removedFields []Field) []Change {
	var changes []Change
	for _, f := range addedFields {
		if f.Type != "foreign_key" || f.ForeignKey == nil {
			continue
		}
		constraintName := fmt.Sprintf("fk_%s_%s", tableName, f.Name)
		changes = append(changes, Change{
			Type:        ChangeTypeForeignKeyAdded,
			TableName:   tableName,
			FieldName:   f.Name,
			Description: fmt.Sprintf("Add foreign key %s on %s.%s → %s", constraintName, tableName, f.Name, f.ForeignKey.Table),
			NewValue:    f,
		})
	}
	for _, f := range removedFields {
		if f.Type != "foreign_key" || f.ForeignKey == nil {
			continue
		}
		constraintName := fmt.Sprintf("fk_%s_%s", tableName, f.Name)
		changes = append(changes, Change{
			Type:        ChangeTypeForeignKeyRemoved,
			TableName:   tableName,
			FieldName:   f.Name,
			Description: fmt.Sprintf("Remove foreign key %s from %s.%s", constraintName, tableName, f.Name),
			OldValue:    f,
			Destructive: true,
		})
	}
	return changes
}
