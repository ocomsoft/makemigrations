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
package writer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ocomsoft/makemigrations/internal/generator"
)

type Writer struct {
	verbose bool
}

func New(verbose bool) *Writer {
	return &Writer{
		verbose: verbose,
	}
}

func (w *Writer) WriteMigration(migration *generator.Migration, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Combine up and down migrations
	content := migration.UpSQL + "\n" + migration.DownSQL

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	if w.verbose {
		fmt.Printf("Written migration to: %s\n", path)
		if migration.IsDestructive {
			fmt.Println("  ⚠️  Contains destructive operations - please review carefully")
		}
	}

	return nil
}

func (w *Writer) PreviewMigration(migration *generator.Migration) string {
	return migration.UpSQL + "\n" + migration.DownSQL
}
