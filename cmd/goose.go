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
package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"github.com/fatih/color"
	"github.com/ocomsoft/makemigrations/internal/config"
	goose "github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	_ "github.com/go-sql-driver/mysql"  // MySQL driver
	_ "github.com/lib/pq"               // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3"     // SQLite driver
	_ "github.com/microsoft/go-mssqldb" // SQL Server driver
)

const (
	DatabaseTypePostgreSQL = "postgresql"
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypeSQLServer  = "sqlserver"
)

var (
	cyan   = color.New(color.FgCyan).SprintFunc()
	blue   = color.New(color.FgBlue).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

// gooseCmd represents the goose command
var gooseCmd = &cobra.Command{
	Use:   "goose",
	Short: "Database migration commands using goose",
	Long: `Database migration commands using goose library.
	
This command provides access to all goose migration operations using the
same configuration as makemigrations (database type, connection settings,
and migrations directory).

Available subcommands:
  up          Migrate the DB to the most recent version available
  up-by-one   Migrate the DB up by 1
  up-to       Migrate the DB to a specific VERSION
  down        Roll back the version by 1
  down-to     Roll back to a specific VERSION
  redo        Re-run the latest migration
  reset       Roll back all migrations
  status      Print the status of all migrations
  version     Print the current version of the database
  create      Create a new migration file
  fix         Apply sequential ordering to migrations`,
}

// buildDatabaseURL constructs a database URL from the configuration
func buildDatabaseURL(cfg *config.Config) (string, error) {
	// For this implementation, we'll need database connection info
	// Since the current config doesn't include connection details,
	// we'll read from environment variables that are commonly used

	switch cfg.Database.Type {
	case "postgresql":
		host := getEnvOrDefault("MAKEMIGRATIONS_DB_HOST", "localhost")
		port := getEnvOrDefault("MAKEMIGRATIONS_DB_PORT", "5432")
		user := getEnvOrDefault("MAKEMIGRATIONS_DB_USER", "postgres")
		password := getEnvOrDefault("MAKEMIGRATIONS_DB_PASSWORD", "")
		dbname := getEnvOrDefault("MAKEMIGRATIONS_DB_NAME", "postgres")
		sslmode := getEnvOrDefault("MAKEMIGRATIONS_DB_SSLMODE", "disable")

		if password != "" {
			return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
				user, password, host, port, dbname, sslmode), nil
		}
		return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s",
			user, host, port, dbname, sslmode), nil

	case DatabaseTypeMySQL:
		host := getEnvOrDefault("MAKEMIGRATIONS_DB_HOST", "localhost")
		port := getEnvOrDefault("MAKEMIGRATIONS_DB_PORT", "3306")
		user := getEnvOrDefault("MAKEMIGRATIONS_DB_USER", "root")
		password := getEnvOrDefault("MAKEMIGRATIONS_DB_PASSWORD", "")
		dbname := getEnvOrDefault("MAKEMIGRATIONS_DB_NAME", "mysql")

		if password != "" {
			return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
				user, password, host, port, dbname), nil
		}
		return fmt.Sprintf("%s@tcp(%s:%s)/%s?parseTime=true",
			user, host, port, dbname), nil

	case "sqlite":
		dbpath := getEnvOrDefault("MAKEMIGRATIONS_DB_PATH", "database.db")
		return dbpath, nil

	case DatabaseTypeSQLServer:
		host := getEnvOrDefault("MAKEMIGRATIONS_DB_HOST", "localhost")
		port := getEnvOrDefault("MAKEMIGRATIONS_DB_PORT", "1433")
		user := getEnvOrDefault("MAKEMIGRATIONS_DB_USER", "sa")
		password := getEnvOrDefault("MAKEMIGRATIONS_DB_PASSWORD", "")
		dbname := getEnvOrDefault("MAKEMIGRATIONS_DB_NAME", "master")

		return fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			user, password, host, port, dbname), nil

	default:
		return "", fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupGooseDB sets up the database connection and goose configuration
func setupGooseDB(cfg *config.Config) (*sql.DB, error) {
	// Build database URL
	dbURL, err := buildDatabaseURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build database URL: %w", err)
	}

	// Map our database types to goose driver names
	var driver string
	switch cfg.Database.Type {
	case "postgresql":
		driver = "postgres"
	case DatabaseTypeMySQL:
		driver = "mysql"
	case "sqlite":
		driver = "sqlite3"
	case DatabaseTypeSQLServer:
		driver = "sqlserver"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	// Open database connection
	db, err := sql.Open(driver, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set goose dialect
	if err := goose.SetDialect(driver); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return db, nil
}

// runGooseCommand executes a goose command with proper error handling
func runGooseCommand(cfg *config.Config, command string, args ...string) error {
	fmt.Printf("%s Running goose %s...\n", blue("▶"), command)

	// Setup database connection
	db, err := setupGooseDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// Execute the goose command
	switch command {
	case "up":
		err = goose.Up(db, cfg.Migration.Directory)
	case "up-by-one":
		err = goose.UpByOne(db, cfg.Migration.Directory)
	case "up-to":
		if len(args) == 0 {
			return fmt.Errorf("up-to requires a version argument")
		}
		version, parseErr := strconv.ParseInt(args[0], 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid version: %s", args[0])
		}
		err = goose.UpTo(db, cfg.Migration.Directory, version)
	case "down":
		err = goose.Down(db, cfg.Migration.Directory)
	case "down-to":
		if len(args) == 0 {
			return fmt.Errorf("down-to requires a version argument")
		}
		version, parseErr := strconv.ParseInt(args[0], 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid version: %s", args[0])
		}
		err = goose.DownTo(db, cfg.Migration.Directory, version)
	case "redo":
		err = goose.Redo(db, cfg.Migration.Directory)
	case "reset":
		err = goose.Reset(db, cfg.Migration.Directory)
	case "status":
		err = goose.Status(db, cfg.Migration.Directory)
	case "version":
		version, versionErr := goose.GetDBVersion(db)
		if versionErr != nil {
			err = versionErr
		} else {
			fmt.Printf("goose: version %d\n", version)
		}
	case "create":
		if len(args) == 0 {
			return fmt.Errorf("create requires a name argument")
		}
		err = goose.Create(db, cfg.Migration.Directory, args[0], "sql")
	case "fix":
		err = goose.Fix(cfg.Migration.Directory)
	default:
		return fmt.Errorf("unknown goose command: %s", command)
	}

	if err != nil {
		return fmt.Errorf("goose %s failed: %w", command, err)
	}

	fmt.Printf("%s goose %s completed successfully\n", green("✓"), command)
	return nil
}

// createGooseSubcommand creates a goose subcommand
func createGooseSubcommand(name, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg := config.LoadOrDefault(configFile)

			return runGooseCommand(cfg, name, args...)
		},
	}
}

func init() {
	rootCmd.AddCommand(gooseCmd)

	// Add all goose subcommands
	gooseCmd.AddCommand(createGooseSubcommand("up",
		"Migrate the DB to the most recent version available",
		"Migrate the DB to the most recent version available"))

	gooseCmd.AddCommand(createGooseSubcommand("up-by-one",
		"Migrate the DB up by 1",
		"Migrate the DB up by 1"))

	upToCmd := createGooseSubcommand("up-to",
		"Migrate the DB to a specific VERSION",
		"Migrate the DB to a specific VERSION")
	upToCmd.Args = cobra.ExactArgs(1)
	gooseCmd.AddCommand(upToCmd)

	gooseCmd.AddCommand(createGooseSubcommand("down",
		"Roll back the version by 1",
		"Roll back the version by 1"))

	downToCmd := createGooseSubcommand("down-to",
		"Roll back to a specific VERSION",
		"Roll back to a specific VERSION")
	downToCmd.Args = cobra.ExactArgs(1)
	gooseCmd.AddCommand(downToCmd)

	gooseCmd.AddCommand(createGooseSubcommand("redo",
		"Re-run the latest migration",
		"Re-run the latest migration"))

	gooseCmd.AddCommand(createGooseSubcommand("reset",
		"Roll back all migrations",
		"Roll back all migrations"))

	gooseCmd.AddCommand(createGooseSubcommand("status",
		"Print the status of all migrations",
		"Print the status of all migrations"))

	gooseCmd.AddCommand(createGooseSubcommand("version",
		"Print the current version of the database",
		"Print the current version of the database"))

	createCmd := createGooseSubcommand("create",
		"Create a new migration file",
		"Create a new migration file")
	createCmd.Args = cobra.ExactArgs(1)
	gooseCmd.AddCommand(createCmd)

	gooseCmd.AddCommand(createGooseSubcommand("fix",
		"Apply sequential ordering to migrations",
		"Apply sequential ordering to migrations"))
}
