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

import "strings"

// shouldAddReviewComment determines if a review comment should be added for a change
func (sc *SQLConverter) shouldAddReviewComment(change Change) bool {
	// First check if this is actually a destructive operation
	if !isDestructiveOperation(change.Type) {
		return false
	}

	// If no prefix is set (empty string), never add review comments
	if sc.reviewCommentPrefix == "" {
		return false
	}

	// Check if this change type should get a review comment
	return sc.destructiveOperations[string(change.Type)]
}

// addReviewComment adds a review comment to comment out the SQL statement
func (sc *SQLConverter) addReviewComment(sqlStmt string) string {
	// If no prefix is set, return unchanged
	if sc.reviewCommentPrefix == "" {
		return sqlStmt
	}

	// Split into lines and add comment prefix to each line to comment them out
	lines := strings.Split(sqlStmt, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Comment out this line by prefixing it with the review comment
			lines[i] = sc.reviewCommentPrefix + line
		}
	}

	return strings.Join(lines, "\n")
}
