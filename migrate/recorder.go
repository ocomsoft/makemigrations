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
)

// createHistoryTableSQL creates the migration history table.
// Uses portable SQL that works across SQLite, PostgreSQL, MySQL.
const createHistoryTableSQL = `CREATE TABLE IF NOT EXISTS makemigrations_history (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    applied_at TEXT DEFAULT CURRENT_TIMESTAMP
)`

// MigrationRecorder manages the makemigrations_history table.
// It records which migrations have been applied to the database.
type MigrationRecorder struct {
	db *sql.DB
}

// NewMigrationRecorder creates a new MigrationRecorder using the given db connection.
func NewMigrationRecorder(db *sql.DB) *MigrationRecorder {
	return &MigrationRecorder{db: db}
}

// EnsureTable creates the makemigrations_history table if it does not exist.
func (r *MigrationRecorder) EnsureTable() error {
	_, err := r.db.Exec(createHistoryTableSQL)
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
	_, err := r.db.Exec("INSERT INTO makemigrations_history (name) VALUES (?)", name)
	if err != nil {
		return fmt.Errorf("recording migration %q as applied: %w", name, err)
	}
	return nil
}

// RecordRolledBack removes a migration name from the history table.
func (r *MigrationRecorder) RecordRolledBack(name string) error {
	_, err := r.db.Exec("DELETE FROM makemigrations_history WHERE name = ?", name)
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
