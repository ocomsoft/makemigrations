# Configuration Guide

This guide covers all configuration options for makemigrations, including config files, environment variables, and runtime flags.

## Configuration Hierarchy

Configuration is loaded in the following priority order (highest to lowest):

1. **Command line flags** (e.g., `--silent`, `--verbose`)
2. **Environment variables** (prefixed with `MAKEMIGRATIONS_`)
3. **Configuration file** (`migrations/makemigrations.config.yaml`)
4. **Default values**

## Configuration File

### Location and Format

The configuration file is automatically created when running `makemigrations init`:

```
migrations/makemigrations.config.yaml
```

You can specify a custom config file location:

```bash
makemigrations --config /path/to/custom-config.yaml makemigrations
```

### Complete Configuration Example

```yaml
# Makemigrations Configuration File
# 
# This file contains configuration for the makemigrations tool.
# All settings can be overridden using environment variables with the prefix MAKEMIGRATIONS_
# For example: MAKEMIGRATIONS_DATABASE_TYPE=mysql
#
# For nested values, use underscores: MAKEMIGRATIONS_OUTPUT_COLOR_ENABLED=false

# Database connection and behavior settings
database:
  type: postgresql                    # postgresql, mysql, sqlserver, sqlite
  default_schema: public              # Default schema name for databases that support schemas
  quote_identifiers: true             # Whether to quote table/column names

# Migration generation and execution settings
migration:
  directory: migrations               # Directory for migration files
  file_prefix: "20060102150405"       # Go timestamp format for YYYYMMDDHHMMSS
  snapshot_file: .schema_snapshot.yaml # Name of the schema snapshot file
  auto_apply: false                   # Whether to auto-apply migrations (dangerous!)
  include_down_sql: true              # Whether to generate DOWN migrations
  review_comment_prefix: "-- REVIEW: " # Prefix for review comments on destructive operations
  rejection_comment_prefix: "-- REJECTED: " # Prefix for rejected destructive operations
  silent: false                       # Whether to skip prompts for destructive operations
  destructive_operations:             # List of operation types to mark with review comments
    - table_removed
    - field_removed
    - index_removed
    - table_renamed
    - field_renamed
    - field_modified

# Schema scanning and processing settings
schema:
  search_paths: []                    # Additional paths to search for schema files
  ignore_modules: []                  # Module patterns to ignore
  schema_file_name: schema.yaml       # Name of schema files to look for
  validate_strict: false              # Whether to use strict validation

# Output formatting and display settings
output:
  verbose: false                      # Enable verbose output
  color_enabled: true                 # Enable colored output
  progress_bar: false                 # Show progress bars
  timestamp_format: "2006-01-02 15:04:05" # Format for timestamps in output
```

## Configuration Sections

### Database Section

Controls database connection and SQL generation behavior.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `type` | string | `postgresql` | Target database type |
| `default_schema` | string | `public` | Default schema name |
| `quote_identifiers` | bool | `true` | Quote table/column names in SQL |

**Supported Database Types:**
- `postgresql` - PostgreSQL 9.6+
- `mysql` - MySQL 5.7+ / MariaDB 10.2+
- `sqlserver` - SQL Server 2016+
- `sqlite` - SQLite 3.8+

**Environment Variable Examples:**
```bash
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
export MAKEMIGRATIONS_DATABASE_DEFAULT_SCHEMA=myapp
export MAKEMIGRATIONS_DATABASE_QUOTE_IDENTIFIERS=false
```

### Migration Section

Controls migration generation, storage, and safety features.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `directory` | string | `migrations` | Directory for migration files |
| `file_prefix` | string | `20060102150405` | Timestamp format for file naming |
| `snapshot_file` | string | `.schema_snapshot.yaml` | Schema snapshot filename |
| `auto_apply` | bool | `false` | Automatically apply migrations (dangerous) |
| `include_down_sql` | bool | `true` | Generate DOWN migration statements |
| `review_comment_prefix` | string | `-- REVIEW: ` | Prefix for review comments |
| `rejection_comment_prefix` | string | `-- REJECTED: ` | Prefix for rejected operations |
| `silent` | bool | `false` | Skip interactive prompts |
| `destructive_operations` | []string | See below | Operations requiring review |

**Default Destructive Operations:**
```yaml
destructive_operations:
  - table_removed      # DROP TABLE statements
  - field_removed      # DROP COLUMN statements  
  - index_removed      # DROP INDEX statements
  - table_renamed      # ALTER TABLE RENAME statements
  - field_renamed      # ALTER TABLE RENAME COLUMN statements
  - field_modified     # ALTER TABLE ALTER COLUMN statements
```

**Environment Variable Examples:**
```bash
export MAKEMIGRATIONS_MIGRATION_DIRECTORY=db/migrations
export MAKEMIGRATIONS_MIGRATION_INCLUDE_DOWN_SQL=false
export MAKEMIGRATIONS_MIGRATION_SILENT=true
export MAKEMIGRATIONS_MIGRATION_REVIEW_COMMENT_PREFIX="-- DANGER: "
```

### Schema Section

Controls YAML schema file discovery and processing.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `search_paths` | []string | `[]` | Additional search paths |
| `ignore_modules` | []string | `[]` | Module patterns to ignore |
| `schema_file_name` | string | `schema.yaml` | Schema filename to search for |
| `validate_strict` | bool | `false` | Enable strict validation |

**Environment Variable Examples:**
```bash
export MAKEMIGRATIONS_SCHEMA_SCHEMA_FILE_NAME=database.yaml
export MAKEMIGRATIONS_SCHEMA_VALIDATE_STRICT=true
```

### Output Section

Controls display formatting and verbosity.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `verbose` | bool | `false` | Enable detailed output |
| `color_enabled` | bool | `true` | Use colored terminal output |
| `progress_bar` | bool | `false` | Show progress indicators |
| `timestamp_format` | string | `2006-01-02 15:04:05` | Timestamp display format |

**Environment Variable Examples:**
```bash
export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
export MAKEMIGRATIONS_OUTPUT_COLOR_ENABLED=false
```

## Environment Variables

### Database Connection Variables

These variables configure database connections for `goose` commands:

```bash
# PostgreSQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=5432
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=password
export MAKEMIGRATIONS_DB_NAME=myapp
export MAKEMIGRATIONS_DB_SSLMODE=disable

# MySQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=3306
export MAKEMIGRATIONS_DB_USER=root
export MAKEMIGRATIONS_DB_PASSWORD=password
export MAKEMIGRATIONS_DB_NAME=myapp

# SQLite
export MAKEMIGRATIONS_DB_PATH=./database.db

# SQL Server
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=1433
export MAKEMIGRATIONS_DB_USER=sa
export MAKEMIGRATIONS_DB_PASSWORD=password
export MAKEMIGRATIONS_DB_NAME=myapp
```

### Configuration Override Variables

All config file settings can be overridden with environment variables:

```bash
# Format: MAKEMIGRATIONS_SECTION_SETTING
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
export MAKEMIGRATIONS_MIGRATION_SILENT=true
export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
```

**Nested Settings:** Use underscores for nested values:
```bash
# migration.review_comment_prefix
export MAKEMIGRATIONS_MIGRATION_REVIEW_COMMENT_PREFIX="-- WARNING: "

# output.color_enabled  
export MAKEMIGRATIONS_OUTPUT_COLOR_ENABLED=false
```

## Command Line Flags

### Global Flags

Available on all commands:

| Flag | Type | Description |
|------|------|-------------|
| `--config` | string | Config file path |
| `--help` | bool | Show help information |

### Common Command Flags

Available on migration commands:

| Flag | Type | Description |
|------|------|-------------|
| `--verbose` | bool | Enable verbose output |
| `--dry-run` | bool | Show what would be generated |
| `--silent` | bool | Skip destructive operation prompts |
| `--check` | bool | Exit with error if migrations needed |
| `--name` | string | Override migration name |

### Database-Specific Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database` | string | `postgresql` | Target database type |

## Use Case Examples

### Development Environment

**Config file (`migrations/makemigrations.config.yaml`):**
```yaml
database:
  type: postgresql
  
migration:
  silent: false                    # Enable interactive prompts
  review_comment_prefix: "-- REVIEW: "
  
output:
  verbose: true                    # Detailed output
  color_enabled: true             # Colored output
```

**Environment variables:**
```bash
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_USER=dev_user
export MAKEMIGRATIONS_DB_PASSWORD=dev_pass
export MAKEMIGRATIONS_DB_NAME=myapp_dev
```

### Staging Environment

```yaml
database:
  type: postgresql
  
migration:
  silent: false                    # Prompt for review
  review_comment_prefix: "-- STAGING-REVIEW: "
  
output:
  verbose: true
  color_enabled: true
```

### Production Environment

```yaml
database:
  type: postgresql
  
migration:
  silent: true                     # No prompts
  review_comment_prefix: "-- PROD-DANGER: "
  rejection_comment_prefix: "-- PROD-REJECTED: "
  
output:
  verbose: false                   # Minimal output
  color_enabled: false            # No colors for logs
```

**Environment variables:**
```bash
export MAKEMIGRATIONS_DB_HOST=prod-db-cluster
export MAKEMIGRATIONS_DB_USER=app_user
export MAKEMIGRATIONS_DB_PASSWORD=$(vault kv get -field=password secret/db)
export MAKEMIGRATIONS_DB_NAME=myapp_production
export MAKEMIGRATIONS_MIGRATION_SILENT=true
```

### CI/CD Environment

```yaml
migration:
  silent: true                     # No interactive prompts
  
output:
  verbose: false                   # Minimal output
  color_enabled: false            # No ANSI colors in logs
```

**Usage in CI:**
```bash
# Check if migrations are needed (exits with code 1 if true)
makemigrations makemigrations --check --silent

# Generate migrations in CI
makemigrations makemigrations --silent --name "automated_$(date +%Y%m%d)"
```

## Advanced Configuration

### Custom Migration Prefixes

```yaml
migration:
  review_comment_prefix: "-- 🚨 MANUAL-REVIEW-REQUIRED: "
  rejection_comment_prefix: "-- ❌ USER-REJECTED: "
```

### Multi-Environment Schema Paths

```yaml
schema:
  search_paths:
    - "schemas/core"
    - "schemas/modules"
    - "vendor/shared-schemas"
  ignore_modules:
    - "github.com/test/*"
    - "*/internal/*"
```

### Database-Specific Settings

```yaml
# PostgreSQL with custom schema
database:
  type: postgresql
  default_schema: myapp_schema
  quote_identifiers: true

# MySQL with specific requirements  
database:
  type: mysql
  quote_identifiers: false          # MySQL often doesn't need quotes

# SQLite for local development
database:
  type: sqlite
  quote_identifiers: false
```

## Validation and Debugging

### Validate Configuration

```bash
# Show current configuration (including overrides)
makemigrations --config migrations/makemigrations.config.yaml --verbose makemigrations --dry-run

# Test environment variable overrides
MAKEMIGRATIONS_OUTPUT_VERBOSE=true makemigrations makemigrations --dry-run
```

### Debug Configuration Loading

```bash
# Enable verbose output to see config loading
makemigrations --verbose makemigrations --dry-run

# Check environment variables
env | grep MAKEMIGRATIONS
```

### Common Configuration Issues

1. **Environment variables not working:**
   ```bash
   # Check variable names (must be exact)
   env | grep MAKEMIGRATIONS_
   
   # Test with explicit export
   export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
   makemigrations --help
   ```

2. **Config file not found:**
   ```bash
   # Check file exists
   ls -la migrations/makemigrations.config.yaml
   
   # Use custom path
   makemigrations --config /full/path/to/config.yaml
   ```

3. **Database connection issues:**
   ```bash
   # Test connection variables
   echo "Host: $MAKEMIGRATIONS_DB_HOST"
   echo "User: $MAKEMIGRATIONS_DB_USER"
   
   # Test with goose status
   makemigrations goose status
   ```

## Security Considerations

### Sensitive Information

**Never commit sensitive data to config files:**

```yaml
# ❌ DON'T DO THIS
database:
  password: my-secret-password      # Don't commit passwords

# ✅ DO THIS  
database:
  type: postgresql                  # Use environment variables for secrets
```

**Use environment variables for secrets:**
```bash
export MAKEMIGRATIONS_DB_PASSWORD=$(vault kv get -field=password secret/db)
export MAKEMIGRATIONS_DB_PASSWORD=$(cat /run/secrets/db_password)
```

### Access Control

**Limit database permissions:**
- Use dedicated migration user with minimal required permissions
- Grant only DDL permissions (CREATE, ALTER, DROP) for schema changes
- Separate users for migration generation vs. execution

### Audit Trail

**Enable verbose logging in production:**
```yaml
output:
  verbose: true                     # Log all operations
migration:
  review_comment_prefix: "-- AUDIT: " # Clear audit trail in SQL
```

## Migration to Environment Variables

### Converting Config File to Environment Variables

**From config file:**
```yaml
database:
  type: mysql
migration:
  silent: true
  review_comment_prefix: "-- REVIEW: "
output:
  verbose: false
```

**To environment variables:**
```bash
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
export MAKEMIGRATIONS_MIGRATION_SILENT=true
export MAKEMIGRATIONS_MIGRATION_REVIEW_COMMENT_PREFIX="-- REVIEW: "
export MAKEMIGRATIONS_OUTPUT_VERBOSE=false
```

For detailed command usage, see the [Commands Documentation](commands/).