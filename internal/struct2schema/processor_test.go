package struct2schema

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewProcessorValid verifies creation with a valid config.
func TestNewProcessorValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a Go file so the directory has content
	goFile := filepath.Join(dir, "model.go")
	if err := os.WriteFile(goFile, []byte("package models\ntype User struct { Name string }\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: filepath.Join(dir, "schema.yaml"),
		TargetDB:   "postgresql",
	}

	p, err := NewProcessor(config)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}
	if p == nil {
		t.Fatal("processor is nil")
	}
}

// TestNewProcessorInvalidInputDir verifies error for nonexistent input dir.
func TestNewProcessorInvalidInputDir(t *testing.T) {
	t.Parallel()

	config := ProcessorConfig{
		InputDir:   "/nonexistent/input/dir",
		OutputFile: "/tmp/output.yaml",
		TargetDB:   "postgresql",
	}

	_, err := NewProcessor(config)
	if err == nil {
		t.Error("expected error for nonexistent input dir")
	}
}

// TestNewProcessorInvalidDB verifies error for invalid database type.
func TestNewProcessorInvalidDB(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: filepath.Join(dir, "schema.yaml"),
		TargetDB:   "invalid_db",
	}

	_, err := NewProcessor(config)
	if err == nil {
		t.Error("expected error for invalid database type")
	}
}

// TestNewProcessorInvalidConfigFile verifies error for nonexistent config file.
func TestNewProcessorInvalidConfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: filepath.Join(dir, "schema.yaml"),
		TargetDB:   "postgresql",
		ConfigFile: "/nonexistent/config.yaml",
	}

	_, err := NewProcessor(config)
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

// TestProcessDryRun verifies the full pipeline in dry-run mode.
func TestProcessDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "model.go")
	src := `package models

type Product struct {
	ID    int    ` + "`db:\"id\" gorm:\"primaryKey\"`" + `
	Name  string ` + "`db:\"name\"`" + `
	Price float64
}
`
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: filepath.Join(dir, "schema.yaml"),
		TargetDB:   "postgresql",
		DryRun:     true,
	}

	p, err := NewProcessor(config)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	// Dry run should not write a file
	if err := p.Process(); err != nil {
		t.Fatalf("Process (dry-run): %v", err)
	}

	// Verify no file was written
	if _, err := os.Stat(config.OutputFile); !os.IsNotExist(err) {
		t.Error("dry-run should not write output file")
	}
}

// TestProcessWritesFile verifies that Process writes the schema file.
func TestProcessWritesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "model.go")
	src := `package models

type Account struct {
	ID   int    ` + "`gorm:\"primaryKey\"`" + `
	Name string ` + "`db:\"name\"`" + `
}
`
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	outputPath := filepath.Join(dir, "output", "schema.yaml")
	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: outputPath,
		TargetDB:   "postgresql",
	}

	p, err := NewProcessor(config)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	if err := p.Process(); err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output file should exist after Process")
	}
}

// TestProcessEmptyDirectory verifies that processing a directory
// with no structs completes without error.
func TestProcessEmptyDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := ProcessorConfig{
		InputDir:   dir,
		OutputFile: filepath.Join(dir, "schema.yaml"),
		TargetDB:   "postgresql",
	}

	p, err := NewProcessor(config)
	if err != nil {
		t.Fatalf("NewProcessor: %v", err)
	}

	// Should complete without error (prints message but no error)
	if err := p.Process(); err != nil {
		t.Fatalf("Process on empty dir: %v", err)
	}
}

// TestValidateConfig verifies the config validation logic.
func TestValidateConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tests := []struct {
		name    string
		config  ProcessorConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ProcessorConfig{
				InputDir: dir,
				TargetDB: "postgresql",
			},
			wantErr: false,
		},
		{
			name: "nonexistent input dir",
			config: ProcessorConfig{
				InputDir: "/nonexistent",
				TargetDB: "postgresql",
			},
			wantErr: true,
		},
		{
			name: "invalid database",
			config: ProcessorConfig{
				InputDir: dir,
				TargetDB: "oracle",
			},
			wantErr: true,
		},
		{
			name: "nonexistent config file",
			config: ProcessorConfig{
				InputDir:   dir,
				TargetDB:   "postgresql",
				ConfigFile: "/nonexistent/config.yaml",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
