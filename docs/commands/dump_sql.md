# dump_sql Command

The `dump_sql` command analyzes and displays the current YAML schema definitions without generating migration files. This is primarily used for debugging, validation, and understanding what SQL would be generated from your schema.

## Overview

The `dump_sql` command processes YAML schema files and outputs the database-specific SQL that would be generated, along with detailed information about the schema processing steps. It's useful for:

- Validating YAML schema syntax and structure
- Previewing database-specific SQL generation
- Debugging schema processing issues
- Understanding how YAML translates to SQL
- Checking schema file discovery

## Usage

```bash
makemigrations dump_sql [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | `false` | Enable detailed output showing processing steps |
| `--database` | string | `postgresql` | Target database type for SQL generation |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## What It Shows

### 1. Schema File Discovery
Lists all YAML schema files found during the scanning process:

```
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
✓ Found schema file: modules/products/schema.yaml
✓ Found schema file: modules/orders/schema.yaml
```

### 2. Schema Processing (with --verbose)
Shows detailed processing steps:

```
▶ Processing YAML schema...
  - Parsing schema/schema.yaml
  - Validating schema structure
  - Applying database defaults
  - Merging multiple schema files
  - Resolving field types and relationships
✓ Schema validation completed
```

### 3. Generated SQL Preview
Displays the CREATE TABLE statements that would be generated:

```sql
-- Database: ecommerce (v1.0.0)
-- Target: postgresql

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    inventory_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 4. Database-Specific Differences
Shows how the same schema generates different SQL for different databases.

## Examples

### Basic Usage

```bash
# Show SQL for current schema
makemigrations dump_sql

# Output
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
▶ Processing YAML schema...
✓ Schema validation completed

-- Database: myapp (v1.0.0)
-- Target: postgresql

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Verbose Mode

```bash
# Show detailed processing information
makemigrations dump_sql --verbose

# Output
▶ Scanning for schema files...
  - Searching in: .
  - Pattern: **/schema.yaml
✓ Found schema file: schema/schema.yaml
  - File size: 1.2KB
  - Last modified: 2024-01-22 13:45:00

▶ Processing YAML schema...
  - Parsing YAML content
  - Validating required fields
  - Applying postgresql defaults
  - Processing table: users (3 fields)
  - Processing field: id (type: uuid, primary_key: true)
  - Processing field: email (type: varchar, length: 255)
  - Processing field: created_at (type: timestamp, auto_create: true)
✓ Schema validation completed

-- Database: myapp (v1.0.0)
-- Target: postgresql
-- Generated at: 2024-01-22 13:45:30

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Different Database Types

```bash
# Generate MySQL-specific SQL
makemigrations dump_sql --database mysql

# Output
-- Database: myapp (v1.0.0)
-- Target: mysql

CREATE TABLE users (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

# Generate SQLite-specific SQL
makemigrations dump_sql --database sqlite

# Output
-- Database: myapp (v1.0.0)
-- Target: sqlite

CREATE TABLE users (
    id TEXT PRIMARY KEY DEFAULT '',
    email VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Output Sections

### Header Information

```sql
-- Database: myapp (v1.0.0)
-- Target: postgresql
-- Generated at: 2024-01-22 13:45:30
-- Schema files processed: 3
```

### Table Creation SQL

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL
);
```

### Index Creation SQL

```sql
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_created_at ON users(created_at);
```

### Foreign Key Constraints

```sql
ALTER TABLE user_profiles 
ADD CONSTRAINT fk_user_profiles_user_id 
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
```

### Junction Tables (Many-to-Many)

```sql
CREATE TABLE product_categories (
    id SERIAL PRIMARY KEY,
    product_id UUID NOT NULL,
    category_id INTEGER NOT NULL,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE,
    UNIQUE(product_id, category_id)
);
```

## Database-Specific Output

### PostgreSQL

```sql
-- PostgreSQL-specific features
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metadata JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_metadata ON products USING GIN(metadata);
```

### MySQL

```sql
-- MySQL-specific features
CREATE TABLE products (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    metadata JSON DEFAULT '{}',
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### SQLite

```sql
-- SQLite-specific features (simplified)
CREATE TABLE products (
    id TEXT PRIMARY KEY DEFAULT '',
    metadata TEXT DEFAULT '{}',
    price NUMERIC DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### SQL Server

```sql
-- SQL Server-specific features
CREATE TABLE products (
    id UNIQUEIDENTIFIER PRIMARY KEY DEFAULT NEWID(),
    metadata NVARCHAR(MAX) DEFAULT '{}',
    price DECIMAL(10,2) NOT NULL,
    created_at DATETIME2 DEFAULT GETDATE()
);
```

## Validation and Debugging

### Schema Validation Errors

The command will show detailed error messages for invalid schemas:

```bash
$ makemigrations dump_sql
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
▶ Processing YAML schema...
✗ Schema validation failed:
  - Table 'users' field 'email': varchar type missing required 'length' property
  - Table 'products' field 'invalid_field': unknown field type 'badtype'
  - Foreign key reference 'nonexistent_table' not found
```

### YAML Syntax Errors

```bash
$ makemigrations dump_sql
▶ Scanning for schema files...
✓ Found schema file: schema/schema.yaml
▶ Processing YAML schema...
✗ YAML parsing failed: schema/schema.yaml:15
  yaml: line 15: found character that cannot start any token
  
  Hint: Check indentation and syntax around line 15
```

### Missing Schema Files

```bash
$ makemigrations dump_sql
▶ Scanning for schema files...
✗ No schema files found in search paths:
  - ./schema/schema.yaml
  - ./*/schema.yaml
  - ./*/*/schema.yaml
  
  Suggestion: Create schema/schema.yaml or check configuration
```

## Use Cases

### 1. Schema Development

```bash
# Iterative schema development
vim schema/schema.yaml
makemigrations dump_sql --verbose    # Check for issues
# Fix issues, repeat
```

### 2. Cross-Database Compatibility

```bash
# Check how schema translates across databases
makemigrations dump_sql --database postgresql > schema-pg.sql
makemigrations dump_sql --database mysql > schema-mysql.sql
makemigrations dump_sql --database sqlite > schema-sqlite.sql

# Compare outputs
diff schema-pg.sql schema-mysql.sql
```

### 3. Documentation Generation

```bash
# Generate schema documentation
makemigrations dump_sql --verbose > docs/database-schema.sql
```

### 4. CI/CD Validation

```bash
# Validate schema in CI pipeline
if ! makemigrations dump_sql --verbose; then
    echo "Schema validation failed"
    exit 1
fi
```

### 5. Debugging Schema Issues

```bash
# Debug processing with maximum verbosity
MAKEMIGRATIONS_OUTPUT_VERBOSE=true makemigrations dump_sql --verbose
```

## Configuration Integration

The command respects configuration settings:

### Database Type Override

```bash
# Override config file database type
makemigrations dump_sql --database mysql

# With environment variable
MAKEMIGRATIONS_DATABASE_TYPE=sqlite makemigrations dump_sql
```

### Schema Search Paths

```yaml
# migrations/makemigrations.config.yaml
schema:
  search_paths:
    - "modules/*/database"
    - "vendor/*/schema"
  schema_file_name: database.yaml
```

### Output Settings

```yaml
# migrations/makemigrations.config.yaml
output:
  verbose: true                    # Default to verbose mode
  color_enabled: false            # Disable colors for file output
  timestamp_format: "15:04:05"    # Custom timestamp format
```

## Advanced Usage

### Multi-Module Projects

```bash
# Dump SQL showing module boundaries
makemigrations dump_sql --verbose

# Output shows file sources
▶ Processing YAML schema...
  - Processing: core/users.yaml (2 tables)
  - Processing: ecommerce/products.yaml (3 tables)  
  - Processing: reporting/analytics.yaml (5 tables)
✓ Total: 10 tables processed
```

### Custom Defaults Testing

```bash
# Test custom defaults
cat > test-schema.yaml << EOF
database:
  name: test
  version: 1.0.0

defaults:
  postgresql:
    custom_uuid: custom_uuid_function()
    custom_timestamp: custom_now()

tables:
  - name: test_table
    fields:
      - name: id
        type: uuid
        default: custom_uuid
      - name: created_at
        type: timestamp
        default: custom_timestamp
EOF

makemigrations dump_sql
```

### Schema Comparison

```bash
# Compare before/after schema changes
makemigrations dump_sql > before.sql
# Make schema changes...
makemigrations dump_sql > after.sql
diff before.sql after.sql
```

## Error Handling

### Common Issues and Solutions

1. **No schema files found**
   ```bash
   # Check current directory
   pwd
   ls -la schema/
   
   # Verify config search paths
   cat migrations/makemigrations.config.yaml
   ```

2. **YAML syntax errors**
   ```bash
   # Validate YAML manually
   python -c "import yaml; yaml.safe_load(open('schema/schema.yaml'))"
   
   # Check indentation
   cat -A schema/schema.yaml
   ```

3. **Type validation errors**
   ```bash
   # Review field definitions
   makemigrations dump_sql --verbose 2>&1 | grep -A5 -B5 "validation failed"
   ```

4. **Foreign key reference errors**
   ```bash
   # Check table dependencies
   grep -n "foreign_key:" schema/schema.yaml
   grep -n "name:" schema/schema.yaml | grep "tables"
   ```

## Performance Considerations

### Large Schemas

For projects with many schema files:

```bash
# Use verbose mode to monitor processing time
time makemigrations dump_sql --verbose

# Consider splitting large schemas
find . -name "schema.yaml" -exec wc -l {} + | sort -n
```

### Output Redirection

```bash
# Redirect output for large schemas
makemigrations dump_sql > full-schema.sql

# Filter specific tables
makemigrations dump_sql | grep -A20 "CREATE TABLE users"
```

## Integration Examples

### Makefile Integration

```makefile
# Makefile
.PHONY: schema-validate schema-docs

schema-validate:
	@echo "Validating schema..."
	@makemigrations dump_sql --verbose > /dev/null

schema-docs:
	@echo "Generating schema documentation..."
	@makemigrations dump_sql --verbose > docs/schema.sql
	@echo "Schema docs generated at docs/schema.sql"

schema-compare:
	@makemigrations dump_sql --database postgresql > schema-pg.sql
	@makemigrations dump_sql --database mysql > schema-mysql.sql
	@echo "Cross-database comparison files generated"
```

### Git Hooks

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Validating schema before commit..."
if ! makemigrations dump_sql --verbose > /dev/null 2>&1; then
    echo "Schema validation failed. Commit aborted."
    exit 1
fi
```

## See Also

- [makemigrations Command](./makemigrations.md) - Generate migrations from schemas
- [Schema Format Guide](../schema-format.md) - YAML schema syntax reference
- [Configuration Guide](../configuration.md) - Configuration options
- [init Command](./init.md) - Initialize new projects