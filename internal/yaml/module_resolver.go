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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ModuleResolver handles resolving Go module paths to filesystem paths
type ModuleResolver struct {
	verbose bool
}

// NewModuleResolver creates a new module resolver
func NewModuleResolver(verbose bool) *ModuleResolver {
	return &ModuleResolver{
		verbose: verbose,
	}
}

// ResolveModulePath resolves a Go module path to a filesystem directory
// It tries workspace modules first, then falls back to Go module cache
func (mr *ModuleResolver) ResolveModulePath(modulePath string) (string, error) {
	// First try to resolve using go list (works for workspace and cached modules)
	dir, err := mr.resolveWithGoList(modulePath)
	if err == nil {
		if mr.verbose {
			fmt.Printf("Resolved module %s to: %s\n", modulePath, dir)
		}
		return dir, nil
	}

	if mr.verbose {
		fmt.Printf("Failed to resolve module %s with go list: %v\n", modulePath, err)
	}

	return "", fmt.Errorf("failed to resolve module %s: %w", modulePath, err)
}

// resolveWithGoList uses 'go list -m -f {{.Dir}}' to find module directory
func (mr *ModuleResolver) resolveWithGoList(modulePath string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modulePath)

	// Set working directory to current directory to respect go.mod/go.work
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("go list failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("go list command failed: %w", err)
	}

	dir := strings.TrimSpace(string(output))
	if dir == "" {
		return "", fmt.Errorf("go list returned empty directory for module %s", modulePath)
	}

	// Verify the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("resolved module directory does not exist: %s", dir)
	}

	return dir, nil
}

// ResolveIncludePath resolves a module + path combination to a full file path
func (mr *ModuleResolver) ResolveIncludePath(include Include) (string, error) {
	moduleDir, err := mr.ResolveModulePath(include.Module)
	if err != nil {
		return "", fmt.Errorf("failed to resolve module %s: %w", include.Module, err)
	}

	fullPath := filepath.Join(moduleDir, include.Path)

	// Verify the file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("included file does not exist: %s (resolved from module %s, path %s)",
			fullPath, include.Module, include.Path)
	}

	return fullPath, nil
}

// IsGoWorkspaceAvailable checks if go.work is available in current directory or parents
func (mr *ModuleResolver) IsGoWorkspaceAvailable() bool {
	cmd := exec.Command("go", "env", "GOWORK")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	goWork := strings.TrimSpace(string(output))
	return goWork != "" && goWork != "off"
}

// GetModuleInfo returns information about the current module context
func (mr *ModuleResolver) GetModuleInfo() (map[string]string, error) {
	info := make(map[string]string)

	// Get GOMOD
	cmd := exec.Command("go", "env", "GOMOD")
	if output, err := cmd.Output(); err == nil {
		info["GOMOD"] = strings.TrimSpace(string(output))
	}

	// Get GOWORK
	cmd = exec.Command("go", "env", "GOWORK")
	if output, err := cmd.Output(); err == nil {
		info["GOWORK"] = strings.TrimSpace(string(output))
	}

	// Get module root
	cmd = exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	if output, err := cmd.Output(); err == nil {
		info["MODULE_ROOT"] = strings.TrimSpace(string(output))
	}

	return info, nil
}
