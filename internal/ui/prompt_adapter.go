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

// Package ui provides bubbletea TUI prompts for destructive migration operations.
package ui

import (
	"fmt"

	yamlpkg "github.com/ocomsoft/morphic/internal/yaml"
)

// NewBubbleteaPromptFunc returns a yamlpkg.PromptFunc that wraps the bubbletea
// TUI prompt. It tracks scope state (all remaining / all of type) across calls
// so that a single "All remaining" or "All of this type" selection applies to
// subsequent destructive operations automatically.
func NewBubbleteaPromptFunc() yamlpkg.PromptFunc {
	// Scope tracking across calls — matches the logic in promptGoMigDecisions.
	var applyAll yamlpkg.PromptResponse
	applyByType := make(map[yamlpkg.ChangeType]yamlpkg.PromptResponse)

	return func(sqlStmt string, change yamlpkg.Change) (yamlpkg.PromptResponse, error) {
		// If a previous "All remaining" selection was made, reuse it
		if applyAll != 0 {
			return applyAll, nil
		}

		// If a previous "All of this type" selection was made, reuse it
		if resp, ok := applyByType[change.Type]; ok {
			return resp, nil
		}

		// Build the prompt title
		var title string
		if change.FieldName != "" {
			title = fmt.Sprintf("Destructive: %s on %q (field: %q)\nSQL: %s", change.Type, change.TableName, change.FieldName, sqlStmt)
		} else {
			title = fmt.Sprintf("Destructive: %s on %q\nSQL: %s", change.Type, change.TableName, sqlStmt)
		}

		resp, scope, err := RunDestructivePrompt(title, change.Type)
		if err != nil {
			return yamlpkg.PromptExit, err
		}

		// Record scope for future calls
		switch scope {
		case ScopeAll:
			applyAll = resp
		case ScopeType:
			applyByType[change.Type] = resp
		}

		// Map PromptGenerateAll to PromptGenerate since scope is now tracked
		// by the adapter itself (not by the SQLConverter's generateAllTypes map).
		// This keeps the PromptGenerateAll semantics consistent.
		if resp == yamlpkg.PromptGenerateAll {
			return yamlpkg.PromptGenerate, nil
		}

		return resp, nil
	}
}
