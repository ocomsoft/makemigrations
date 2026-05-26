package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGoWork(t *testing.T) {
	// Create a temp directory tree: root/sub1/sub2
	root := t.TempDir()
	sub1 := filepath.Join(root, "sub1")
	sub2 := filepath.Join(sub1, "sub2")
	if err := os.MkdirAll(sub2, 0o755); err != nil {
		t.Fatal(err)
	}

	// No go.work anywhere — should return empty
	if got := findGoWork(sub2); got != "" {
		t.Fatalf("expected empty, got %s", got)
	}

	// Place go.work in root
	workPath := filepath.Join(root, "go.work")
	if err := os.WriteFile(workPath, []byte("go 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should find it from sub2
	got := findGoWork(sub2)
	if got != workPath {
		t.Fatalf("expected %s, got %s", workPath, got)
	}

	// Should find it from root itself
	got = findGoWork(root)
	if got != workPath {
		t.Fatalf("expected %s, got %s", workPath, got)
	}
}

func TestModuleCachePath_NotFound(t *testing.T) {
	// A module that definitely won't be in the cache
	got := moduleCachePath("example.com/nonexistent/module", "v0.0.0")
	if got != "" {
		t.Fatalf("expected empty for nonexistent module, got %s", got)
	}
}

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"./foo", true},
		{"../bar", true},
		{"/absolute/path", true},
		{"github.com/some/module", false},
		{"v1.2.3", false},
	}

	for _, tt := range tests {
		if got := isLocalPath(tt.path); got != tt.want {
			t.Errorf("isLocalPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBuildModuleMap_Replace(t *testing.T) {
	root := t.TempDir()
	localMod := filepath.Join(root, "localmod")
	if err := os.MkdirAll(localMod, 0o755); err != nil {
		t.Fatal(err)
	}
	// The local module needs a go.mod
	if err := os.WriteFile(filepath.Join(localMod, "go.mod"),
		[]byte("module example.com/local\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Main module's go.mod with a replace directive
	goMod := "module example.com/main\n\ngo 1.22\n\n" +
		"require example.com/local v0.0.0\n\n" +
		"replace example.com/local => ./localmod\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run from the root directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	mr := NewModuleResolver(false)
	dir, err := mr.ResolveModulePath("example.com/local")
	if err != nil {
		t.Fatalf("expected to resolve, got error: %v", err)
	}
	if dir != localMod {
		t.Fatalf("expected %s, got %s", localMod, dir)
	}
}

func TestBuildModuleMap_Workspace(t *testing.T) {
	root := t.TempDir()

	// Workspace member directory with its own go.mod
	memberDir := filepath.Join(root, "member")
	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "go.mod"),
		[]byte("module example.com/member\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Main module directory
	mainDir := filepath.Join(root, "main")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "go.mod"),
		[]byte("module example.com/main\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// go.work at root
	goWork := "go 1.22\n\nuse (\n\t./main\n\t./member\n)\n"
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(goWork), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run from the main directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(mainDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	mr := NewModuleResolver(false)
	dir, err := mr.ResolveModulePath("example.com/member")
	if err != nil {
		t.Fatalf("expected to resolve workspace member, got error: %v", err)
	}
	if dir != memberDir {
		t.Fatalf("expected %s, got %s", memberDir, dir)
	}
}
