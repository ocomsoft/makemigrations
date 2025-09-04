# Installation Guide

This guide covers installing and setting up makemigrations for YAML-based database schema management.

## Prerequisites

- **Go 1.24 or later** - Required for building and running makemigrations
- **Git** - For cloning the repository  
- **Database** - One of 12 supported databases: PostgreSQL, MySQL, SQLite, SQL Server, Redshift, ClickHouse, TiDB, Vertica, YDB, Turso, StarRocks, or Aurora DSQL

## Installation Methods

### Option 1: Install from Source (Recommended)

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ocomsoft/makemigrations.git
   cd makemigrations
   ```

2. **Build the binary:**
   ```bash
   go build -o makemigrations .
   ```

3. **Install globally (optional):**
   ```bash
   # Install to GOPATH/bin
   go install .
   
   # Or copy to system PATH
   sudo cp makemigrations /usr/local/bin/
   ```

### Option 2: Go Install (Recommended)

```bash
go install github.com/ocomsoft/makemigrations@latest
```

### Option 3: Download Binary

Download pre-built binaries from the [releases page](https://github.com/ocomsoft/makemigrations/releases):

```bash
# Linux/macOS
curl -L https://github.com/ocomsoft/makemigrations/releases/latest/download/makemigrations-linux-amd64 -o makemigrations
chmod +x makemigrations

# Windows
curl -L https://github.com/ocomsoft/makemigrations/releases/latest/download/makemigrations-windows-amd64.exe -o makemigrations.exe
```

## Project Setup

### 1. Initialize Your Project

Create a new project with YAML schema management:

```bash
# Initialize migrations directory with YAML support
makemigrations init

# This creates:
# migrations/
# ├── makemigrations.config.yaml
# └── .schema_snapshot.yaml
```

### 2. Create Schema Directory

Create your schema directory structure:

```bash
mkdir -p schema
touch schema/schema.yaml
```

### 3. Define Your First Schema

Edit `schema/schema.yaml`:

```yaml
database:
  name: myapp
  version: 1.0.0

defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    # ... other defaults

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
      - name: created_at
        type: timestamp
        default: now
        auto_create: true
```

### 4. Generate Your First Migration

```bash
# Generate migration from schema
makemigrations makemigrations --name initial_schema

# Review the generated migration
cat migrations/YYYYMMDDHHMMSS_initial_schema.sql
```

## Database Setup

### PostgreSQL

1. **Install PostgreSQL driver dependencies:**
   ```bash
   # Already included in makemigrations binary
   ```

2. **Set environment variables:**
   ```bash
   export MAKEMIGRATIONS_DB_HOST=localhost
   export MAKEMIGRATIONS_DB_PORT=5432
   export MAKEMIGRATIONS_DB_USER=postgres
   export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
   export MAKEMIGRATIONS_DB_NAME=yourdatabase
   export MAKEMIGRATIONS_DB_SSLMODE=disable
   ```

3. **Test connection:**
   ```bash
   makemigrations goose status
   ```

### MySQL

1. **Set environment variables:**
   ```bash
   export MAKEMIGRATIONS_DB_HOST=localhost
   export MAKEMIGRATIONS_DB_PORT=3306
   export MAKEMIGRATIONS_DB_USER=root
   export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
   export MAKEMIGRATIONS_DB_NAME=yourdatabase
   ```

2. **Update config for MySQL:**
   ```yaml
   # migrations/makemigrations.config.yaml
   database:
     type: mysql
   ```

### SQLite

1. **Set database path:**
   ```bash
   export MAKEMIGRATIONS_DB_PATH=./database.db
   ```

2. **Update config:**
   ```yaml
   # migrations/makemigrations.config.yaml
   database:
     type: sqlite
   ```

## Verification

### Test Installation

```bash
# Check version and help
makemigrations --help

# Test YAML schema parsing
makemigrations makemigrations --dry-run

# Test database connection (requires setup)
makemigrations goose status
```

### Verify Schema Processing

```bash
# Scan for schema files
makemigrations dump_sql --verbose

# Generate a test migration
makemigrations makemigrations --dry-run --name test_migration
```

## IDE Integration

### VS Code

1. **Install YAML extension:**
   - Install "YAML" by Red Hat
   - Install "Go" by Google (for Go module support)

2. **Configure YAML schema validation:**
   ```json
   // .vscode/settings.json
   {
     "yaml.schemas": {
       "./schema_format.md": "schema/schema.yaml"
     }
   }
   ```

### GoLand/IntelliJ

1. **Enable YAML support** in File → Settings → Plugins
2. **Configure file associations** for `*.yaml` files in schema directories

## Environment Configuration

### Development Environment

Create a `.env` file for development:

```bash
# Database connection
MAKEMIGRATIONS_DB_HOST=localhost
MAKEMIGRATIONS_DB_PORT=5432
MAKEMIGRATIONS_DB_USER=dev_user
MAKEMIGRATIONS_DB_PASSWORD=dev_password
MAKEMIGRATIONS_DB_NAME=myapp_development

# Migration settings
MAKEMIGRATIONS_MIGRATION_SILENT=false
MAKEMIGRATIONS_OUTPUT_VERBOSE=true
```

### Production Environment

Set environment variables in your deployment:

```bash
# Required
MAKEMIGRATIONS_DB_HOST=production-db-host
MAKEMIGRATIONS_DB_USER=app_user
MAKEMIGRATIONS_DB_PASSWORD=secure_password
MAKEMIGRATIONS_DB_NAME=myapp_production

# Optional overrides
MAKEMIGRATIONS_MIGRATION_SILENT=true
MAKEMIGRATIONS_OUTPUT_COLOR_ENABLED=false
```

## Troubleshooting

### Common Issues

1. **"Command not found"**
   ```bash
   # Ensure binary is in PATH
   export PATH=$PATH:$(go env GOPATH)/bin
   
   # Or use full path
   ./makemigrations --help
   ```

2. **"No schema files found"**
   ```bash
   # Check schema directory structure
   find . -name "schema.yaml" -type f
   
   # Verify file naming
   ls -la schema/
   ```

3. **Database connection issues**
   ```bash
   # Test environment variables
   echo $MAKEMIGRATIONS_DB_HOST
   
   # Test direct connection
   psql -h $MAKEMIGRATIONS_DB_HOST -U $MAKEMIGRATIONS_DB_USER -d $MAKEMIGRATIONS_DB_NAME
   ```

4. **YAML parsing errors**
   ```bash
   # Validate YAML syntax
   yamllint schema/schema.yaml
   
   # Check for indentation issues
   cat -A schema/schema.yaml
   ```

### Debug Mode

Enable verbose output for troubleshooting:

```bash
# Verbose schema processing
makemigrations makemigrations --verbose --dry-run

# Verbose goose operations
makemigrations goose status --verbose
```

## Next Steps

1. **Read the [Configuration Guide](configuration.md)** to customize settings
2. **Review [Schema Format Documentation](schema-format.md)** for YAML schema syntax
3. **Explore [Command Documentation](commands/)** for detailed usage
4. **Set up CI/CD integration** using `--check` and `--silent` flags

## Support

- **Documentation**: Browse `/docs` directory for detailed guides
- **Examples**: Check `/example` directory for sample schemas
- **Issues**: Report problems on GitHub Issues
- **Discussions**: Join GitHub Discussions for questions

For detailed command usage, see the [Commands Documentation](commands/).