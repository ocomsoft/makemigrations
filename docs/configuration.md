# Configuration Guide

This guide covers all configuration options for morphic, including config files, environment variables, and runtime flags.

## Configuration Hierarchy

Configuration is loaded in the following priority order (highest to lowest):

1. **Command line flags** (e.g., `--verbose`)
2. **Environment variables** (prefixed with `MORPHIC_`)
3. **Configuration file** (`migrations/morphic.config.yaml`)
4. **Default values**

## Configuration File

### Location and Format

The configuration file is automatically created when running `morphic init`:

```
migrations/morphic.config.yaml
```

You can specify a custom config file location:

```bash
morphic --config /path/to/custom-config.yaml makemigrations
```

### Complete Configuration Example

```yaml
# Morphic Configuration File
#
# This file contains configuration for the morphic tool.
# All settings can be overridden using environment variables with the prefix MORPHIC_
# For example: MORPHIC_DATABASE_TYPE=mysql
#
# For nested values, use underscores: MORPHIC_OUTPUT_COLOR_ENABLED=false

# Database connection and behavior settings
database:
  type: postgresql                    # postgresql, mysql, sqlserver, sqlite
  default_url: ""                     # Fallback database URL when DATABASE_URL env var is not set

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

Controls database SQL generation behavior and connection defaults.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `type` | string | `postgresql` | Target database type |
| `default_url` | string | `""` | Fallback database URL when `DATABASE_URL` env var is not set |

**Supported Database Types:**
- `postgresql` - PostgreSQL 9.6+
- `mysql` - MySQL 5.7+ / MariaDB 10.2+
- `sqlserver` - SQL Server 2016+
- `sqlite` - SQLite 3.8+

**Database URL Precedence:**

When connecting to a database (e.g., `morphic migrate`, `morphic db-to-schema`, `morphic dump-data`), the URL is resolved in this order:

1. Command-line flags (`--host`, `--port`, `--database`, etc.) or `--dsn`
2. `DATABASE_URL` environment variable
3. `database.default_url` from config file
4. Built from individual flag defaults (localhost, default port, etc.)

This allows you to set a project-level database URL in config while still overriding it per-environment via the `DATABASE_URL` env var.

**Environment Variable Examples:**
```bash
export MORPHIC_DATABASE_TYPE=mysql
export MORPHIC_DATABASE_DEFAULT_URL="host=localhost port=5432 dbname=myapp user=dev sslmode=disable"
```

### Migration Section

Controls migration file storage.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `directory` | string | `migrations` | Directory for migration files |

**Environment Variable Example:**
```bash
export MORPHIC_MIGRATION_DIRECTORY=db/migrations
```

### Output Section

Controls display formatting and verbosity.

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `verbose` | bool | `false` | Enable detailed output |
| `color_enabled` | bool | `true` | Use colored terminal output |

**Environment Variable Examples:**
```bash
export MORPHIC_OUTPUT_VERBOSE=true
export MORPHIC_OUTPUT_COLOR_ENABLED=false
```

## Environment Variables

### Configuration Override Variables

All config file settings can be overridden with environment variables:

```bash
# Format: MORPHIC_SECTION_SETTING
export MORPHIC_DATABASE_TYPE=mysql
export MORPHIC_DATABASE_DEFAULT_URL="host=localhost port=5432 dbname=myapp user=dev sslmode=disable"
export MORPHIC_MIGRATION_DIRECTORY=db/migrations
export MORPHIC_OUTPUT_VERBOSE=true
export MORPHIC_OUTPUT_COLOR_ENABLED=false
```

### Database Connection Variables

The `DATABASE_URL` environment variable is used by `morphic migrate`, `morphic db-to-schema`, `morphic dump-data`, and `morphic db-diff` to connect to the database:

```bash
# PostgreSQL
export DATABASE_URL="host=localhost port=5432 dbname=myapp user=postgres sslmode=disable"
```

If `DATABASE_URL` is not set, morphic falls back to `database.default_url` from the config file.

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

**Config file (`migrations/morphic.config.yaml`):**
```yaml
database:
  type: postgresql
  default_url: "host=localhost port=5432 dbname=myapp_dev user=dev sslmode=disable"

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
morphic makemigrations --check

# Generate migrations in CI
morphic makemigrations --name "automated_$(date +%Y%m%d)"
```

## Validation and Debugging

### Validate Configuration

```bash
# Show current configuration (including overrides)
morphic --config migrations/morphic.config.yaml --verbose makemigrations --dry-run

# Test environment variable overrides
MORPHIC_OUTPUT_VERBOSE=true morphic makemigrations --dry-run
```

### Debug Configuration Loading

```bash
# Enable verbose output to see config loading
morphic --verbose makemigrations --dry-run

# Check environment variables
env | grep MORPHIC_
```

### Common Configuration Issues

1. **Environment variables not working:**
   ```bash
   # Check variable names (must be exact)
   env | grep MORPHIC_
   
   # Test with explicit export
   export MORPHIC_OUTPUT_VERBOSE=true
   morphic --help
   ```

2. **Config file not found:**
   ```bash
   # Check file exists
   ls -la migrations/morphic.config.yaml
   
   # Use custom path
   morphic --config /full/path/to/config.yaml
   ```

3. **Database connection issues:**
   ```bash
   # Test connection variables
   echo "DATABASE_URL: $DATABASE_URL"
   
   # Test with migrate status
   morphic migrate status
   ```

## Security Considerations

### Sensitive Information

**Never commit sensitive data to config files:**

```yaml
# DON'T DO THIS
database:
  default_url: "host=prod port=5432 dbname=myapp user=admin password=secret"

# DO THIS - use environment variables for secrets
database:
  type: postgresql
  default_url: ""                   # Set DATABASE_URL env var instead
```

**Use environment variables for secrets:**
```bash
export DATABASE_URL=$(vault kv get -field=url secret/db)
export DATABASE_URL=$(cat /run/secrets/db_url)
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

## Legacy Configuration

Morphic supports backward compatibility with the legacy `makemigrations.config.yaml` filename. If no `morphic.config.yaml` is found, the loader will fall back to `makemigrations.config.yaml`. Environment variables with the `MAKEMIGRATIONS_` prefix are no longer supported; use `MORPHIC_` instead.

## Converting Config File to Environment Variables

**From config file:**
```yaml
database:
  type: mysql
  default_url: "host=localhost port=3306 dbname=myapp user=root"
migration:
  directory: db/migrations
output:
  verbose: false
```

**To environment variables:**
```bash
export MORPHIC_DATABASE_TYPE=mysql
export MORPHIC_DATABASE_DEFAULT_URL="host=localhost port=3306 dbname=myapp user=root"
export MORPHIC_MIGRATION_DIRECTORY=db/migrations
export MORPHIC_OUTPUT_VERBOSE=false
```

For detailed command usage, see the [Commands Documentation](commands/).
