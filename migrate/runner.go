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

package migrate

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers"
)

// RunOptions controls optional behaviour for Up and Down.
type RunOptions struct {
	// WarnOnMissingDrop causes drop operations that fail because the target
	// object does not exist to print a warning and continue rather than stop.
	WarnOnMissingDrop bool
}

// Runner executes migrations against a database in topological order.
type Runner struct {
	graph    *Graph
	provider providers.Provider
	db       *sql.DB
	recorder *MigrationRecorder
}

// NewRunner creates a Runner using the given graph, provider, db, and recorder.
func NewRunner(graph *Graph, provider providers.Provider, db *sql.DB, recorder *MigrationRecorder) *Runner {
	return &Runner{
		graph:    graph,
		provider: provider,
		db:       db,
		recorder: recorder,
	}
}

// Up applies all pending migrations in topological order.
// If to is non-empty, stops after applying the named migration.
func (r *Runner) Up(to string, opts RunOptions) error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return fmt.Errorf("linearizing graph: %w", err)
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}
	state := NewSchemaState()

	// Replay already-applied migrations to rebuild state
	for _, mig := range plan {
		if !applied[mig.Name] {
			continue
		}
		for _, op := range mig.Operations {
			if err := op.Mutate(state); err != nil {
				return fmt.Errorf("replaying state for %q: %w", mig.Name, err)
			}
		}
	}

	for _, mig := range plan {
		if applied[mig.Name] {
			continue
		}
		fmt.Printf("Applying %s...", mig.Name)
		if err := r.applyMigration(mig, state, opts); err != nil {
			fmt.Println(" FAILED")
			return fmt.Errorf("applying migration %q: %w", mig.Name, err)
		}
		fmt.Println(" done")
		if to != "" && mig.Name == to {
			break
		}
	}
	return nil
}

// Down rolls back migrations. If steps > 0, rolls back that many.
// If to is set, rolls back until that migration name is reached (exclusive).
func (r *Runner) Down(steps int, to string, opts RunOptions) error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return fmt.Errorf("linearizing graph: %w", err)
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	// Collect applied migrations in reverse topological order
	var toRollback []*Migration
	for i := len(plan) - 1; i >= 0; i-- {
		if applied[plan[i].Name] {
			toRollback = append(toRollback, plan[i])
		}
	}

	for i, mig := range toRollback {
		if steps > 0 && i >= steps {
			break
		}
		if to != "" && mig.Name == to {
			break
		}
		// Reconstruct state just before this migration by replaying
		// all applied migrations up to (but not including) this one
		state := NewSchemaState()
		for _, m := range plan {
			if m.Name == mig.Name {
				break
			}
			if applied[m.Name] {
				for _, op := range m.Operations {
					_ = op.Mutate(state)
				}
			}
		}
		fmt.Printf("Rolling back %s...", mig.Name)
		if err := r.rollbackMigration(mig, state, opts); err != nil {
			fmt.Println(" FAILED")
			return fmt.Errorf("rolling back migration %q: %w", mig.Name, err)
		}
		fmt.Println(" done")
	}
	return nil
}

// Status prints migration status: applied vs pending.
func (r *Runner) Status() error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return err
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return err
	}
	fmt.Printf("%-50s %s\n", "Migration", "Status")
	fmt.Println(strings.Repeat("-", 60))
	for _, mig := range plan {
		status := "Pending"
		if applied[mig.Name] {
			status = "Applied"
		}
		fmt.Printf("%-50s %s\n", mig.Name, status)
	}
	return nil
}

// ShowSQL prints all pending migration SQL without executing it.
func (r *Runner) ShowSQL() error {
	plan, err := r.graph.Linearize()
	if err != nil {
		return err
	}
	applied, err := r.recorder.GetApplied()
	if err != nil {
		return err
	}
	state := NewSchemaState()
	for _, mig := range plan {
		if applied[mig.Name] {
			for _, op := range mig.Operations {
				_ = op.Mutate(state)
			}
			continue
		}
		fmt.Printf("-- %s\n", mig.Name)
		for _, op := range mig.Operations {
			r.provider.SetTypeMappings(state.TypeMappings)
			sqlStr, err := op.Up(r.provider, state, state.Defaults)
			if err != nil {
				return fmt.Errorf("generating SQL for %q: %w", mig.Name, err)
			}
			if sqlStr != "" {
				fmt.Println(sqlStr)
				fmt.Println()
			}
			_ = op.Mutate(state)
		}
	}
	return nil
}

// applyMigration executes all operations in a migration within a transaction
// and records it as applied atomically. If any operation fails the transaction
// is rolled back and the database is left unchanged.
// When opts.WarnOnMissingDrop is true, drop operations that fail because the object
// does not exist are skipped with a warning instead of stopping the migration.
//
// Note: DDL statements in MySQL are auto-committed and cannot be rolled back
// regardless of the transaction. PostgreSQL supports transactional DDL fully.
func (r *Runner) applyMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op if already committed

	for _, op := range mig.Operations {
		r.provider.SetTypeMappings(state.TypeMappings)
		sqlStr, err := op.Up(r.provider, state, state.Defaults)
		if err != nil {
			return fmt.Errorf("generating SQL for operation %q: %w", op.Describe(), err)
		}
		skipped := false
		if sqlStr != "" {
			if _, execErr := tx.Exec(sqlStr); execErr != nil {
				if opts.WarnOnMissingDrop && isDropOp(op) && r.provider.IsNotFoundError(execErr) {
					fmt.Printf("[WARNING] %s — object not found in database, skipping\n", op.Describe())
					skipped = true
				} else {
					return fmt.Errorf("executing SQL %q: %w", sqlStr, execErr)
				}
			}
		}
		// Skip state mutation when the drop operation was skipped — the object
		// was never in the schema state either, so Mutate would fail.
		if !skipped {
			if err := op.Mutate(state); err != nil {
				return fmt.Errorf("mutating state: %w", err)
			}
		}
	}

	if err := r.recorder.RecordAppliedTx(tx, mig.Name); err != nil {
		return err
	}

	return tx.Commit()
}

// rollbackMigration reverses all operations in a migration within a transaction
// and removes it from history atomically. If any operation fails the transaction
// is rolled back and the database is left unchanged.
// When opts.WarnOnMissingDrop is true, drop operations that fail because the object
// does not exist are skipped with a warning instead of stopping the rollback.
//
// Note: DDL statements in MySQL are auto-committed and cannot be rolled back
// regardless of the transaction. PostgreSQL supports transactional DDL fully.
func (r *Runner) rollbackMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op if already committed

	// Pre-apply state-only ops (SetDefaults, SetTypeMappings) from this migration so that
	// Defaults and TypeMappings are populated when generating Down SQL for other ops.
	for _, op := range mig.Operations {
		switch op.TypeName() {
		case "set_defaults", "set_type_mappings":
			_ = op.Mutate(state)
		}
	}

	for i := len(mig.Operations) - 1; i >= 0; i-- {
		op := mig.Operations[i]
		r.provider.SetTypeMappings(state.TypeMappings)
		sqlStr, err := op.Down(r.provider, state, state.Defaults)
		if err != nil {
			return fmt.Errorf("generating down SQL for %q: %w", op.Describe(), err)
		}
		if sqlStr != "" {
			if _, execErr := tx.Exec(sqlStr); execErr != nil {
				if opts.WarnOnMissingDrop && isDropOp(op) && r.provider.IsNotFoundError(execErr) {
					fmt.Printf("[WARNING] %s — object not found in database, skipping\n", op.Describe())
				} else if isCreateOp(op) && r.provider.IsAlreadyExistsError(execErr) {
					fmt.Printf("[WARNING] %s — object already exists in database, skipping\n", op.Describe())
				} else {
					return fmt.Errorf("executing down SQL %q: %w", sqlStr, execErr)
				}
			}
		}
	}

	if err := r.recorder.RecordRolledBackTx(tx, mig.Name); err != nil {
		return err
	}

	return tx.Commit()
}

// isDropOp returns true for operations that remove a database object.
// Only drop operations trigger WarnOnMissingDrop — all other failures stop immediately.
func isDropOp(op Operation) bool {
	switch op.TypeName() {
	case "drop_table", "drop_field", "drop_index":
		return true
	default:
		return false
	}
}

// isCreateOp returns true for operations whose Down SQL creates a database object.
// Used during rollback to skip re-creating objects that already exist.
func isCreateOp(op Operation) bool {
	switch op.TypeName() {
	case "drop_table", "drop_field", "drop_index":
		return true
	default:
		return false
	}
}
