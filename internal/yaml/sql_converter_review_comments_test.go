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
	"strings"
	"testing"
)

func TestSQLConverter_ReviewComments(t *testing.T) {
	tests := []struct {
		name                 string
		reviewPrefix         string
		destructiveOps       []string
		changeType           ChangeType
		sqlStatement         string
		expectedContains     string
		expectedNotContains  string
		shouldBeCommentedOut bool
	}{
		{
			name:                 "field_removed with default prefix",
			reviewPrefix:         "-- REVIEW: ",
			destructiveOps:       []string{"field_removed"},
			changeType:           ChangeTypeFieldRemoved,
			sqlStatement:         `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedContains:     "-- REVIEW: ALTER TABLE",
			shouldBeCommentedOut: true,
		},
		{
			name:                 "field_removed with custom prefix",
			reviewPrefix:         "-- WARNING: ",
			destructiveOps:       []string{"field_removed"},
			changeType:           ChangeTypeFieldRemoved,
			sqlStatement:         `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedContains:     "-- WARNING: ALTER TABLE",
			shouldBeCommentedOut: true,
		},
		{
			name:                 "field_removed with empty prefix - no comment",
			reviewPrefix:         "",
			destructiveOps:       []string{"field_removed"},
			changeType:           ChangeTypeFieldRemoved,
			sqlStatement:         `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedContains:     `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedNotContains:  "-- ",
			shouldBeCommentedOut: false,
		},
		{
			name:                 "field_removed not in destructive list - no comment",
			reviewPrefix:         "-- REVIEW: ",
			destructiveOps:       []string{"table_removed"},
			changeType:           ChangeTypeFieldRemoved,
			sqlStatement:         `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedContains:     `ALTER TABLE "users" DROP COLUMN "email";`,
			expectedNotContains:  "-- REVIEW:",
			shouldBeCommentedOut: false,
		},
		{
			name:                 "table_removed with prefix",
			reviewPrefix:         "-- DANGER: ",
			destructiveOps:       []string{"table_removed"},
			changeType:           ChangeTypeTableRemoved,
			sqlStatement:         `DROP TABLE "users";`,
			expectedContains:     "-- DANGER: DROP TABLE",
			shouldBeCommentedOut: true,
		},
		{
			name:                 "field_added not destructive - no comment",
			reviewPrefix:         "-- REVIEW: ",
			destructiveOps:       []string{"field_removed", "table_removed"},
			changeType:           ChangeTypeFieldAdded,
			sqlStatement:         `ALTER TABLE "users" ADD COLUMN "email" VARCHAR(255);`,
			expectedContains:     `ALTER TABLE "users" ADD COLUMN "email"`,
			expectedNotContains:  "-- REVIEW:",
			shouldBeCommentedOut: false,
		},
		{
			name:                 "multiline SQL statement",
			reviewPrefix:         "-- REVIEW: ",
			destructiveOps:       []string{"field_removed"},
			changeType:           ChangeTypeFieldRemoved,
			sqlStatement:         "ALTER TABLE \"users\"\nDROP COLUMN \"email\";",
			expectedContains:     "-- REVIEW: ALTER TABLE",
			shouldBeCommentedOut: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create SQLConverter with test configuration
			converter := NewSQLConverterWithConfig(
				DatabasePostgreSQL,
				false, // verbose
				false, // safeTypeChanges
				tt.reviewPrefix,
				tt.destructiveOps,
				true,            // silent (no prompting in tests)
				"-- REJECTED: ", // rejectionPrefix
			)

			// Create a test change
			change := Change{
				Type:        tt.changeType,
				Description: "Test change",
				Destructive: true, // All our test cases are destructive changes
			}

			// Test shouldAddReviewComment
			shouldAdd := converter.shouldAddReviewComment(change)
			expectedShouldAdd := tt.reviewPrefix != "" && contains(tt.destructiveOps, string(tt.changeType))

			if shouldAdd != expectedShouldAdd {
				t.Errorf("shouldAddReviewComment() = %v, expected %v (prefix=%q, ops=%v, changeType=%s)",
					shouldAdd, expectedShouldAdd, tt.reviewPrefix, tt.destructiveOps, string(tt.changeType))
			}

			// Test addReviewComment - only apply if shouldAdd is true
			var result string
			if shouldAdd {
				result = converter.addReviewComment(tt.sqlStatement)
			} else {
				result = tt.sqlStatement
			}

			// Check expected content
			if tt.expectedContains != "" && !strings.Contains(result, tt.expectedContains) {
				t.Errorf("Expected result to contain %q, but got:\n%s", tt.expectedContains, result)
			}

			// Check unexpected content
			if tt.expectedNotContains != "" && strings.Contains(result, tt.expectedNotContains) {
				t.Errorf("Expected result NOT to contain %q, but got:\n%s", tt.expectedNotContains, result)
			}

			// Check if SQL is properly commented out
			if tt.shouldBeCommentedOut {
				lines := strings.Split(result, "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
						t.Errorf("Expected all non-empty lines to be commented out, but found uncommented line: %q", line)
					}
				}
			} else {
				// Should have at least one uncommented SQL line
				lines := strings.Split(result, "\n")
				hasUncommentedSQL := false
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
						hasUncommentedSQL = true
						break
					}
				}
				if !hasUncommentedSQL {
					t.Errorf("Expected to have uncommented SQL, but all lines are commented:\n%s", result)
				}
			}
		})
	}
}

func TestSQLConverter_ReviewCommentsIntegration(t *testing.T) {
	// Test the full integration with ConvertDiffToSQL
	converter := NewSQLConverterWithConfig(
		DatabasePostgreSQL,
		false, // verbose
		false, // safeTypeChanges
		"-- REVIEW: ",
		[]string{"field_removed"},
		true,            // silent (no prompting in tests)
		"-- REJECTED: ", // rejectionPrefix
	)

	// Create a SchemaDiff with a field removal
	diff := &SchemaDiff{
		HasChanges:    true,
		IsDestructive: true,
		Changes: []Change{
			{
				Type:        ChangeTypeFieldRemoved,
				Description: "Remove email field",
				Destructive: true,
				TableName:   "users",
				FieldName:   "email",
			},
		},
	}

	// Create test schemas (not actually used by ConvertDiffToSQL but needed for the method signature)
	oldSchema := &Schema{}
	newSchema := &Schema{}

	upSQL, downSQL, err := converter.ConvertDiffToSQL(diff, oldSchema, newSchema)
	if err != nil {
		t.Fatalf("ConvertDiffToSQL failed: %v", err)
	}

	// Check that the UP SQL contains review comment
	if !strings.Contains(upSQL, "-- REVIEW: ") {
		t.Errorf("Expected UP SQL to contain review comment, got:\n%s", upSQL)
	}

	// Check that the DOWN SQL is also commented out (if it exists)
	if downSQL != "" && !strings.Contains(downSQL, "-- REVIEW: ") {
		t.Errorf("Expected DOWN SQL to contain review comment, got:\n%s", downSQL)
	}

	// Verify the actual SQL is commented out
	if !strings.Contains(upSQL, "-- REVIEW: ALTER TABLE") {
		t.Errorf("Expected UP SQL to have commented ALTER TABLE, got:\n%s", upSQL)
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
