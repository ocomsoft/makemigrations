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
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// ModuleResolver handles resolving Go module paths to filesystem paths
// without shelling out to `go list`. It reads go.mod and go.work directly.
type ModuleResolver struct {
	verbose bool
	// Lazily populated: module path → absolute directory
	moduleMap map[string]string
}

// NewModuleResolver creates a new module resolver
func NewModuleResolver(verbose bool) *ModuleResolver {
	return &ModuleResolver{
		verbose: verbose,
	}
}

// ResolveModulePath resolves a Go module path to a filesystem directory.
// Resolution order: go.mod replace directives, go.work workspace members,
// then the Go module cache under GOPATH.
func (mr *ModuleResolver) ResolveModulePath(modulePath string) (string, error) {
	if mr.moduleMap == nil {
		if err := mr.buildModuleMap(); err != nil {
			return "", fmt.Errorf("building module map: %w", err)
		}
	}

	if dir, ok := mr.moduleMap[modulePath]; ok {
		if mr.verbose {
			fmt.Printf("Resolved module %s to: %s\n", modulePath, dir)
		}
		return dir, nil
	}

	return "", fmt.Errorf("module %s not found in go.mod replace directives, go.work workspace, or module cache", modulePath)
}

// ResolveIncludePath resolves a module + path combination to a full file path
func (mr *ModuleResolver) ResolveIncludePath(include Include) (string, error) {
	moduleDir, err := mr.ResolveModulePath(include.Module)
	if err != nil {
		return "", fmt.Errorf("failed to resolve module %s: %w", include.Module, err)
	}

	fullPath := filepath.Join(moduleDir, include.Path)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("included file does not exist: %s (resolved from module %s, path %s)",
			fullPath, include.Module, include.Path)
	}

	return fullPath, nil
}

// buildModuleMap populates the module→directory mapping from go.mod, go.work,
// and the module cache. Called once lazily.
func (mr *ModuleResolver) buildModuleMap() error {
	mr.moduleMap = make(map[string]string)

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	goModPath := filepath.Join(wd, "go.mod")
	goModData, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}

	goMod, err := modfile.Parse(goModPath, goModData, nil)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	// 1. go.mod replace directives with local paths
	for _, rep := range goMod.Replace {
		if rep.New.Version == "" && isLocalPath(rep.New.Path) {
			absDir := rep.New.Path
			if !filepath.IsAbs(absDir) {
				absDir = filepath.Join(wd, absDir)
			}
			absDir = filepath.Clean(absDir)
			if dirExists(absDir) {
				mr.moduleMap[rep.Old.Path] = absDir
				if mr.verbose {
					fmt.Printf("  replace: %s → %s\n", rep.Old.Path, absDir)
				}
			}
		}
	}

	// 2. go.work workspace members
	workPath := findGoWork(wd)
	if workPath != "" {
		if err := mr.loadWorkspaceModules(workPath); err != nil && mr.verbose {
			fmt.Printf("  warning: failed to load go.work modules: %v\n", err)
		}
	}

	// 3. Module cache for go.mod require directives not already resolved
	for _, req := range goMod.Require {
		if _, ok := mr.moduleMap[req.Mod.Path]; ok {
			continue
		}
		if cacheDir := moduleCachePath(req.Mod.Path, req.Mod.Version); cacheDir != "" {
			mr.moduleMap[req.Mod.Path] = cacheDir
			if mr.verbose {
				fmt.Printf("  cache: %s → %s\n", req.Mod.Path, cacheDir)
			}
		}
	}

	return nil
}

// loadWorkspaceModules reads go.work, then reads each workspace member's
// go.mod to learn its module path.
func (mr *ModuleResolver) loadWorkspaceModules(workPath string) error {
	data, err := os.ReadFile(workPath)
	if err != nil {
		return err
	}

	workFile, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return fmt.Errorf("parsing go.work: %w", err)
	}

	workDir := filepath.Dir(workPath)

	for _, use := range workFile.Use {
		memberDir := use.Path
		if !filepath.IsAbs(memberDir) {
			memberDir = filepath.Join(workDir, memberDir)
		}
		memberDir = filepath.Clean(memberDir)

		memberModPath := filepath.Join(memberDir, "go.mod")
		memberData, err := os.ReadFile(memberModPath)
		if err != nil {
			if mr.verbose {
				fmt.Printf("  warning: cannot read %s: %v\n", memberModPath, err)
			}
			continue
		}

		memberMod, err := modfile.Parse(memberModPath, memberData, nil)
		if err != nil {
			if mr.verbose {
				fmt.Printf("  warning: cannot parse %s: %v\n", memberModPath, err)
			}
			continue
		}

		if memberMod.Module != nil {
			mr.moduleMap[memberMod.Module.Mod.Path] = memberDir
			if mr.verbose {
				fmt.Printf("  workspace: %s → %s\n", memberMod.Module.Mod.Path, memberDir)
			}
		}
	}

	// Also load replace directives from go.work itself
	for _, rep := range workFile.Replace {
		if rep.New.Version == "" && isLocalPath(rep.New.Path) {
			absDir := rep.New.Path
			if !filepath.IsAbs(absDir) {
				absDir = filepath.Join(workDir, absDir)
			}
			absDir = filepath.Clean(absDir)
			if dirExists(absDir) {
				mr.moduleMap[rep.Old.Path] = absDir
				if mr.verbose {
					fmt.Printf("  work replace: %s → %s\n", rep.Old.Path, absDir)
				}
			}
		}
	}

	return nil
}

// findGoWork walks from startDir upward looking for go.work.
func findGoWork(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, "go.work")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// moduleCachePath returns the module cache directory if it exists.
func moduleCachePath(modPath, version string) string {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		goPath = filepath.Join(home, "go")
	}

	version = strings.TrimSuffix(version, "+incompatible")

	// module.EscapePath handles the case-encoding Go uses in the cache
	escaped, err := module.EscapePath(modPath)
	if err != nil {
		escaped = modPath
	}

	cachePath := filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", escaped, version))
	if dirExists(cachePath) {
		return cachePath
	}

	return ""
}

func isLocalPath(p string) bool {
	return strings.HasPrefix(p, ".") || strings.HasPrefix(p, "/")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
