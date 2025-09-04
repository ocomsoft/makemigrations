# Product Requirements Document: Go Migration Generator

## Overview
A Go utility that replicates Django's `makemigrations` functionality for PostgreSQL databases by collecting and merging `schema.sql` files from Go modules and generating Goose migrations.

## Core Functionality

### 1. Schema Discovery
- Scan direct dependencies from `go.mod`
- Look for `sql/schema.sql` files in each dependency
- Optionally check for `-- MIGRATION_SCHEMA` marker on first line
- Collect all discovered schema files

### 2. Schema Merging
- Concatenate all discovered schema files into a single unified schema
- Handle table merging when multiple modules define the same table
- Conflict resolution rules:
  - VARCHAR fields: larger size wins (255 over 100)
  - NULL constraints: NOT NULL wins over nullable
  - Other conflicts: implement sensible defaults

### 3. Dependency Resolution
- Analyze foreign key relationships
- Topologically sort tables to ensure correct creation order
- Handle circular dependencies gracefully

### 4. PostgreSQL Schema Elements Support
- Tables and columns
- Indexes (handle duplicates and naming conflicts)
- Constraints (primary key, foreign key, unique, check)
- Triggers and functions
- Sequences and custom types
- Schema namespaces

### 5. Migration Generation
- Compare current merged schema against last recorded schema
- Generate Goose-compatible migration files
- Store in `migrations/` directory
- Use Goose naming convention (e.g., `00001_add_users_table.sql`)
- Generate both UP and DOWN migrations
- Auto-generate meaningful descriptions based on changes

### 6. State Management
- Store last complete merged schema in migrations directory
- Use well-known filename that Goose ignores (e.g., `migrations/.schema_snapshot.sql`)
- Update snapshot after successful migration generation

### 7. Safety Features
- Add `-- REVIEW` comment to destructive operations (DROP, DELETE)
- Validate schema syntax before generating migrations
- Detect and warn about potential data loss

## Command Line Interface

### Primary Commands
```bash
# First time setup - initialize migrations directory
makemigrations init [flags]

# Default behavior (runs makemigrations_sql)
makemigrations [flags]

# Explicit subcommand
makemigrations makemigrations_sql [flags]
```

### Flags
- `--dry-run` - Show what would be generated without creating files
- `--name <name>` - Override auto-generated migration name
- `--check` - Exit with error code if migrations are needed (for CI/CD)
- `--verbose` - Show detailed processing information
- `--help` - Display help information

## Technical Architecture

### Dependencies
- `github.com/stripe/pg-schema-diff` - For schema comparison and diff generation
- `github.com/pressly/goose` - For migration file format compatibility
- Go standard library for module parsing and file operations

### Core Components

1. **Module Scanner**
   - Parse go.mod for direct dependencies
   - Locate and read schema.sql files
   - Validate schema markers

2. **Schema Merger**
   - Parse SQL statements
   - Identify and merge duplicate table definitions
   - Apply conflict resolution rules
   - Maintain element metadata (source module, line numbers)

3. **Dependency Analyzer**
   - Build dependency graph from foreign keys
   - Perform topological sort
   - Detect circular references

4. **Diff Engine**
   - Load previous schema snapshot
   - Compare with current merged schema
   - Generate migration statements
   - Create rollback statements

5. **Migration Writer**
   - Format according to Goose conventions
   - Generate descriptive names
   - Add safety comments
   - Write to migrations directory

## Migration File Format

### Example Up Migration
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
-- +goose StatementEnd
```

### Example Down Migration
```sql
-- +goose Down
-- +goose StatementBegin
-- REVIEW
DROP INDEX IF EXISTS idx_users_email;
-- REVIEW
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
```

## Error Handling

### Fatal Errors
- No go.mod file found
- Invalid SQL syntax in schema files
- Unresolvable circular dependencies
- File system write failures

### Warnings
- Destructive operations detected
- Potential data type incompatibilities
- Missing schema markers (when expected)
- Naming conflicts in indexes/constraints

## Success Metrics
- Successfully merges schemas from multiple modules
- Generates valid Goose migrations
- Handles complex PostgreSQL schemas
- Provides clear error messages
- Maintains migration history accurately

## Future Enhancements
- Support for custom merge strategies
- Integration with CI/CD pipelines
- Schema validation rules
- Migration squashing
- Rollback verification
- Support for other SQL databases

## Example Workflow

1. Developer adds/modifies `sql/schema.sql` in their module
2. Run `makemigrations`
3. Utility scans dependencies and merges schemas
4. Compares with last snapshot
5. Generates migration file `migrations/00002_add_orders_table.sql`
6. Updates schema snapshot
7. Developer reviews migration (especially `-- REVIEW` sections)
8. Run `goose up` to apply migration

## Configuration (Optional Future Enhancement)
Consider adding `.makemigrations.yaml` for:
- Custom schema file locations
- Module inclusion/exclusion rules
- Conflict resolution overrides
- Custom naming conventions