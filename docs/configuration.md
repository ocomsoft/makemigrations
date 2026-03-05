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

# Migration generation and execution settings
migration:
  directory: migrations               # Directory for migration files

# Output formatting and display settings
output:
  verbose: false                      # Enable verbose output
  color_enabled: true                 # Enable colored output
```

## Configuration Sections

### Database Section

Controls database SQL generation behavior.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `type` | string | `postgresql` | Target database type |

**Supported Database Types:**
- `postgresql` - PostgreSQL 9.6+
- `mysql` - MySQL 5.7+ / MariaDB 10.2+
- `sqlserver` - SQL Server 2016+
- `sqlite` - SQLite 3.8+

**Environment Variable Example:**
```bash
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
```

### Migration Section

Controls migration file storage.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `directory` | string | `migrations` | Directory for migration files |

**Environment Variable Example:**
```bash
export MAKEMIGRATIONS_MIGRATION_DIRECTORY=db/migrations
```

### Output Section

Controls display formatting and verbosity.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `verbose` | bool | `false` | Enable detailed output |
| `color_enabled` | bool | `true` | Use colored terminal output |

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
export MAKEMIGRATIONS_MIGRATION_DIRECTORY=db/migrations
export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
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

output:
  verbose: true
  color_enabled: true
```

### CI/CD Environment

```yaml
output:
  verbose: false
  color_enabled: false             # No ANSI colors in logs
```

**Usage in CI:**
```bash
# Check if migrations are needed (exits with code 1 if true)
makemigrations makemigrations --check

# Generate migrations in CI
makemigrations makemigrations --name "automated_$(date +%Y%m%d)"
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
```

## Migration to Environment Variables

### Converting Config File to Environment Variables

**From config file:**
```yaml
database:
  type: mysql
migration:
  directory: db/migrations
output:
  verbose: false
```

**To environment variables:**
```bash
export MAKEMIGRATIONS_DATABASE_TYPE=mysql
export MAKEMIGRATIONS_MIGRATION_DIRECTORY=db/migrations
export MAKEMIGRATIONS_OUTPUT_VERBOSE=false
```

For detailed command usage, see the [Commands Documentation](commands/).