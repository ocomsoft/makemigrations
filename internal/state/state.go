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
package state

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultMigrationsDir = "migrations"
	SnapshotFilename     = ".schema_snapshot.sql"
)

type Manager struct {
	migrationsDir string
	verbose       bool
}

func New(migrationsDir string, verbose bool) *Manager {
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}
	return &Manager{
		migrationsDir: migrationsDir,
		verbose:       verbose,
	}
}

func (m *Manager) EnsureMigrationsDir() error {
	if _, err := os.Stat(m.migrationsDir); os.IsNotExist(err) {
		if m.verbose {
			fmt.Printf("Creating migrations directory: %s\n", m.migrationsDir)
		}
		if err := os.MkdirAll(m.migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}
	return nil
}

func (m *Manager) GetSnapshotPath() string {
	return filepath.Join(m.migrationsDir, SnapshotFilename)
}

func (m *Manager) LoadSnapshot() (string, error) {
	path := m.GetSnapshotPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if m.verbose {
				fmt.Println("No previous schema snapshot found - this is the first migration")
			}
			return "", nil
		}
		return "", fmt.Errorf("failed to read schema snapshot: %w", err)
	}

	if m.verbose {
		fmt.Printf("Loaded schema snapshot from: %s\n", path)
	}

	return string(data), nil
}

func (m *Manager) SaveSnapshot(schema string) error {
	if err := m.EnsureMigrationsDir(); err != nil {
		return err
	}

	path := m.GetSnapshotPath()

	if err := os.WriteFile(path, []byte(schema), 0644); err != nil {
		return fmt.Errorf("failed to write schema snapshot: %w", err)
	}

	if m.verbose {
		fmt.Printf("Saved schema snapshot to: %s\n", path)
	}

	return nil
}

func (m *Manager) GetMigrationsDir() string {
	return m.migrationsDir
}

func (m *Manager) GetNextMigrationNumber() (int, error) {
	if err := m.EnsureMigrationsDir(); err != nil {
		return 0, err
	}

	files, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	maxNumber := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		// Skip snapshot file
		if name == SnapshotFilename {
			continue
		}

		// Try to extract number from filename (e.g., "00001_initial.sql")
		var num int
		if _, err := fmt.Sscanf(name, "%d", &num); err == nil {
			if num > maxNumber {
				maxNumber = num
			}
		}
	}

	return maxNumber + 1, nil
}
