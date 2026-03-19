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

	"github.com/ocomsoft/makemigrations/internal/providers"
)

// MigrationRecorder manages the makemigrations_history table.
// It records which migrations have been applied to the database.
type MigrationRecorder struct {
	db       *sql.DB
	provider providers.Provider
}

// NewMigrationRecorder creates a new MigrationRecorder using the given db connection
// and provider. The provider supplies the database-specific DDL and placeholder style.
func NewMigrationRecorder(db *sql.DB, p providers.Provider) *MigrationRecorder {
	return &MigrationRecorder{db: db, provider: p}
}

// EnsureTable creates the makemigrations_history table if it does not exist.
// The DDL is supplied by the provider so it is correct for the target database.
func (r *MigrationRecorder) EnsureTable() error {
	_, err := r.db.Exec(r.provider.HistoryTableDDL())
	if err != nil {
		return fmt.Errorf("creating makemigrations_history table: %w", err)
	}
	return nil
}

// GetApplied returns a set of migration names that have been applied.
func (r *MigrationRecorder) GetApplied() (map[string]bool, error) {
	rows, err := r.db.Query("SELECT name FROM makemigrations_history")
	if err != nil {
		return nil, fmt.Errorf("querying applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning migration name: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// RecordApplied inserts a migration name into the history table.
func (r *MigrationRecorder) RecordApplied(name string) error {
	query := "INSERT INTO makemigrations_history (name) VALUES (" + r.provider.Placeholder(1) + ")"
	_, err := r.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("recording migration %q as applied: %w", name, err)
	}
	return nil
}

// RecordAppliedTx inserts a migration name into the history table within an
// existing transaction, so the history record is committed or rolled back
// atomically with the migration SQL.
func (r *MigrationRecorder) RecordAppliedTx(tx *sql.Tx, name string) error {
	query := "INSERT INTO makemigrations_history (name) VALUES (" + r.provider.Placeholder(1) + ")"
	_, err := tx.Exec(query, name)
	if err != nil {
		return fmt.Errorf("recording migration %q as applied: %w", name, err)
	}
	return nil
}

// RecordRolledBack removes a migration name from the history table.
func (r *MigrationRecorder) RecordRolledBack(name string) error {
	query := "DELETE FROM makemigrations_history WHERE name = " + r.provider.Placeholder(1)
	_, err := r.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("recording migration %q as rolled back: %w", name, err)
	}
	return nil
}

// RecordRolledBackTx removes a migration name from the history table within an
// existing transaction, so the history deletion is committed or rolled back
// atomically with the rollback SQL.
func (r *MigrationRecorder) RecordRolledBackTx(tx *sql.Tx, name string) error {
	query := "DELETE FROM makemigrations_history WHERE name = " + r.provider.Placeholder(1)
	_, err := tx.Exec(query, name)
	if err != nil {
		return fmt.Errorf("recording migration %q as rolled back: %w", name, err)
	}
	return nil
}

// Fake inserts a migration name without executing any SQL.
// Used to mark migrations as applied when the database already has the schema.
func (r *MigrationRecorder) Fake(name string) error {
	return r.RecordApplied(name)
}
