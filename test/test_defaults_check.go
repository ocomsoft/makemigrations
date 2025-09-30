package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"os/exec"
)

func main() {
	// Create temp directory
	tempDir, err := ioutil.TempDir("", "test-defaults-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	// Change to temp dir
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create schema directory
	schemaDir := filepath.Join(tempDir, "schema")
	os.MkdirAll(schemaDir, 0755)

	// Write schema with defaults
	schemaContent := `database:
  name: test_defaults
  version: 1.0.0

tables:
  - name: test_table
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: int_field
        type: integer
        nullable: false
        default: zero
      - name: bool_field
        type: boolean
        nullable: false
        default: true
      - name: timestamp_field
        type: timestamp
        nullable: false
        default: now
      - name: literal_string
        type: varchar
        length: 100
        nullable: false
        default: "hello"
`

	schemaFile := filepath.Join(schemaDir, "schema.yaml")
	err = ioutil.WriteFile(schemaFile, []byte(schemaContent), 0644)
	if err != nil {
		fmt.Printf("Error writing schema: %v\n", err)
		return
	}

	// Run init command
	cmd := exec.Command("/home/ocom/go/go1.24.2/bin/go", "run", "main.go", "init", "--database", "postgresql")
	cmd.Dir = "/workspaces/ocom/go/makemigrations"
	cmd.Env = append(os.Environ(), "GOROOT=/home/ocom/go/go1.24.2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running init: %v\nOutput: %s\n", err, output)
		return
	}

	// Read the generated migration
	migrationsDir := filepath.Join(tempDir, "migrations")
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		fmt.Printf("Error reading migrations dir: %v\n", err)
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			content, err := ioutil.ReadFile(filepath.Join(migrationsDir, file.Name()))
			if err != nil {
				fmt.Printf("Error reading migration: %v\n", err)
				continue
			}
			fmt.Printf("Migration file: %s\n", file.Name())
			fmt.Println(strings.Repeat("=", 60))
			fmt.Println(string(content))
			fmt.Println(strings.Repeat("=", 60))

			// Check for specific defaults
			if strings.Contains(string(content), "DEFAULT zero") {
				fmt.Println("❌ Found 'DEFAULT zero' - should be 'DEFAULT 0'")
			}
			if strings.Contains(string(content), "DEFAULT true") {
				fmt.Println("❌ Found 'DEFAULT true' - might be correct depending on DB")
			}
			if strings.Contains(string(content), "DEFAULT 0") {
				fmt.Println("✅ Found 'DEFAULT 0' - correct!")
			}
			if strings.Contains(string(content), "DEFAULT CURRENT_TIMESTAMP") {
				fmt.Println("✅ Found 'DEFAULT CURRENT_TIMESTAMP' - correct!")
			}
		}
	}
}
