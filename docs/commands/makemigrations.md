# makemigrations Command

The `makemigrations` command is the **primary command** for generating database migrations from YAML schema definitions. This is the core functionality that makes makemigrations a YAML-first database migration tool.

## Overview

The `makemigrations` command analyzes your YAML schema files, compares them against the current database state (stored in a snapshot), and generates SQL migrations to bring your database schema in sync with your YAML definitions.

## Usage

```bash
makemigrations makemigrations [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--check` | bool | `false` | Exit with error code if migrations are needed (CI/CD mode) |
| `--dry-run` | bool | `false` | Show what migrations would be generated without creating files |
| `--name` | string | auto-generated | Override the migration name |
| `--silent` | bool | `false` | Skip interactive prompts for destructive operations |
| `--verbose` | bool | `false` | Enable detailed output during processing |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## How It Works

### 1. Schema Discovery
The command scans for YAML schema files starting from the current directory:

```bash
# Default search patterns
./schema/schema.yaml
./*/schema.yaml
./*/*/schema.yaml
```

### 2. Schema Processing
- Parses and validates YAML schema syntax
- Merges multiple schema files if found
- Applies database-specific defaults and transformations
- Generates normalized internal schema representation

### 3. Change Detection
- Loads the current schema snapshot (`.schema_snapshot.yaml`)
- Compares new schema against snapshot
- Identifies changes: additions, modifications, removals, renames

### 4. Migration Generation
- Generates database-specific SQL for detected changes
- Creates timestamped migration files with UP and DOWN operations
- Updates schema snapshot with new state
- Handles destructive operations with review comments

## Schema File Structure

The command expects YAML files with this basic structure:

```yaml
database:
  name: myapp
  version: 1.0.0

defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()

tables:
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
      - name: email
        type: varchar
        length: 255
        nullable: false
```

## Examples

### Basic Usage

```bash
# Generate migrations from schema changes
makemigrations makemigrations

# Output
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
▶ Processing YAML schema...
✓ Schema validation completed
▶ Comparing with current snapshot...
✓ Detected 3 changes
▶ Generating migration: 20240122134500_add_user_table
✓ Migration generated successfully
```

### Dry Run Mode

```bash
# Preview what would be generated
makemigrations makemigrations --dry-run

# Output
▶ DRY RUN MODE - No files will be created
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
▶ Would generate migration: 20240122134500_add_user_table

--- UP Migration Preview ---
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

--- DOWN Migration Preview ---
DROP TABLE users;
```

### Custom Migration Name

```bash
# Specify custom migration name
makemigrations makemigrations --name "initial_user_schema"

# Generates: 20240122134500_initial_user_schema.sql
```

### CI/CD Check Mode

```bash
# Exit with error if migrations needed (useful in CI)
makemigrations makemigrations --check

# Exit codes:
# 0 = No migrations needed
# 1 = Migrations needed or error occurred
```

### Silent Mode

```bash
# Skip prompts for destructive operations
makemigrations makemigrations --silent

# Automatically proceeds with:
# - Table removals
# - Field removals  
# - Field modifications
# - Table/field renames
```

## Generated Migration Files

### File Naming Convention

```
migrations/YYYYMMDDHHMMSS_description.sql
```

Example: `migrations/20240122134500_add_user_table.sql`

### Migration File Format

Generated files use the Goose migration format:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down  
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

## Change Detection Types

The command detects and handles these types of schema changes:

### Safe Changes (Auto-Generated)
- **Table added** - Creates new tables
- **Field added** - Adds new columns
- **Index added** - Creates new indexes
- **Field nullable changed** (false → true) - Makes columns nullable

### Destructive Changes (Review Required)
- **Table removed** - Drops tables
- **Field removed** - Drops columns
- **Table renamed** - Renames tables
- **Field renamed** - Renames columns
- **Field modified** - Changes column types/constraints

### Review Comments

Destructive operations are marked with review comments:

```sql
-- +goose Up
-- REVIEW: The following operation is destructive and requires manual review
-- +goose StatementBegin
DROP TABLE old_users;
-- +goose StatementEnd
```

## Interactive Prompts

For destructive operations, the command prompts for confirmation:

```
⚠️  DESTRUCTIVE OPERATION DETECTED ⚠️
Table 'old_users' will be REMOVED

This operation will:
- Drop the table 'old_users'
- Permanently delete all data in this table
- Remove any dependent foreign keys

Do you want to proceed with this destructive operation?
[y]es / [n]o / [r]eview SQL / [a]lways proceed: 
```

Options:
- **[y]es** - Proceed with the operation
- **[n]o** - Skip this operation (adds REJECTED comment)
- **[r]eview SQL** - Show the SQL that would be generated
- **[a]lways proceed** - Apply to all remaining destructive operations

## Configuration Integration

The command respects all configuration settings:

### From Config File (`migrations/makemigrations.config.yaml`)

```yaml
database:
  type: postgresql              # Target database type

migration:
  directory: migrations         # Migration file location
  silent: false                # Skip destructive prompts
  include_down_sql: true        # Generate rollback SQL
  review_comment_prefix: "-- REVIEW: "
  
schema:
  schema_file_name: schema.yaml # Schema filename to search for
  validate_strict: false        # Enable strict validation

output:
  verbose: false               # Detailed output
  color_enabled: true          # Colored terminal output
```

### From Environment Variables

```bash
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
export MAKEMIGRATIONS_MIGRATION_SILENT=true
export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
```

## Database Support

The command generates database-specific SQL for:

### PostgreSQL
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### MySQL
```sql
CREATE TABLE users (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### SQLite
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY DEFAULT '',
    email VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### SQL Server
```sql
CREATE TABLE users (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    email VARCHAR(255) NOT NULL,
    created_at DATETIME2 DEFAULT GETDATE()
);
```

## Error Handling

### Common Errors and Solutions

1. **No schema files found**
   ```bash
   Error: No schema files found in search paths
   Solution: Create schema/schema.yaml or check search paths in config
   ```

2. **YAML syntax error**
   ```bash
   Error: yaml: line 10: found character that cannot start any token
   Solution: Check YAML indentation and syntax
   ```

3. **Invalid schema structure**
   ```bash
   Error: table 'users' is missing required 'fields' array
   Solution: Ensure all tables have fields defined
   ```

4. **Database connection failure**
   ```bash
   Error: failed to connect to database for snapshot comparison
   Solution: Check database configuration and connectivity
   ```

## Best Practices

### 1. Schema Organization
```bash
# Good structure
schema/
├── schema.yaml           # Main schema
├── core/
│   └── users.yaml       # User management tables  
└── modules/
    ├── products.yaml    # Product catalog
    └── orders.yaml      # Order management
```

### 2. Migration Workflow
```bash
# 1. Make schema changes
vim schema/schema.yaml

# 2. Review changes with dry run
makemigrations makemigrations --dry-run

# 3. Generate migration
makemigrations makemigrations --name "add_user_preferences"

# 4. Review generated SQL
cat migrations/20240122134500_add_user_preferences.sql

# 5. Apply migration
makemigrations goose up
```

### 3. Team Coordination
```bash
# Check for conflicts before generating
makemigrations makemigrations --check

# Use descriptive migration names
makemigrations makemigrations --name "add_user_email_verification"

# Always test rollback
makemigrations goose down
makemigrations goose up
```

### 4. Production Safety
```bash
# Use silent mode in production scripts
makemigrations makemigrations --silent

# Always backup before destructive changes
pg_dump mydb > backup_$(date +%Y%m%d).sql
makemigrations makemigrations
```

## Troubleshooting

### Debug Mode
```bash
# Enable verbose output for troubleshooting
makemigrations makemigrations --verbose --dry-run
```

### Schema Validation
```bash
# Test schema syntax without generating migrations
makemigrations dump_sql --verbose
```

### Snapshot Issues
```bash
# Reset snapshot if corrupted
rm migrations/.schema_snapshot.yaml
makemigrations makemigrations --dry-run
```

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/migrations.yml
- name: Check for pending migrations
  run: |
    makemigrations makemigrations --check
    if [ $? -eq 1 ]; then
      echo "Migrations needed - failing build"
      exit 1
    fi
```

### Development Script

```bash
#!/bin/bash
# dev-migrate.sh

set -e

echo "Checking for schema changes..."
if makemigrations makemigrations --check; then
    echo "No migrations needed"
else
    echo "Generating migrations..."
    makemigrations makemigrations --verbose
    
    echo "Applying migrations..."
    makemigrations goose up
    
    echo "Migration complete!"
fi
```

## See Also

- [Schema Format Guide](../schema-format.md) - Complete YAML schema reference
- [Configuration Guide](../configuration.md) - Configuration options
- [Goose Integration](./goose.md) - Applying migrations to database
- [Installation Guide](../installation.md) - Setup and installation