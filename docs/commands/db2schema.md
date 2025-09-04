# db2schema Command

The `db2schema` command extracts database schema information from a PostgreSQL database and generates a YAML schema file compatible with makemigrations. This reverse engineering tool allows you to convert existing database structures into makemigrations schema format.

## Overview

The `db2schema` command connects to a PostgreSQL database, reads the INFORMATION_SCHEMA tables, and extracts complete metadata to generate a comprehensive YAML schema file. It's useful for:

- Converting existing databases to makemigrations format
- Reverse engineering database structures for documentation
- Migrating from other schema management tools
- Creating schema files from production databases
- Generating starting points for new schema-driven development

## Usage

```bash
makemigrations db2schema [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--host` | string | `localhost` | Database host |
| `--port` | int | `5432` | Database port |
| `--database` | string | `postgres` | Database name |
| `--username` | string | `postgres` | Database username |
| `--password` | string | | Database password |
| `--sslmode` | string | `disable` | SSL mode (disable, require, verify-ca, verify-full) |
| `--output` | string | `schema.yaml` | Output YAML schema file path |
| `--verbose` | bool | `false` | Show detailed processing information |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## What It Extracts

### 1. Database Tables
Extracts all user-defined tables from the public schema:

```yaml
tables:
  - name: users
    fields: [...]
  - name: products  
    fields: [...]
  - name: orders
    fields: [...]
```

### 2. Field Information
Complete field metadata including:
- **Name**: Column name
- **Data Type**: Converted to makemigrations YAML types
- **Length**: For VARCHAR and TEXT fields
- **Precision/Scale**: For DECIMAL fields  
- **Nullable**: Whether the field accepts NULL values
- **Primary Key**: Primary key constraints
- **Default Values**: Converted to makemigrations format

```yaml
fields:
  - name: id
    type: uuid
    primary_key: true
    default: new_uuid
    nullable: false
  - name: email
    type: varchar
    length: 255
    nullable: false
  - name: created_at
    type: timestamp
    default: now
    nullable: false
```

### 3. Foreign Key Relationships
Extracts foreign key constraints with ON DELETE actions:

```yaml
- name: user_id
  type: foreign_key
  nullable: false
  foreign_key:
    table: users
    on_delete: CASCADE
```

### 4. Indexes
All indexes including unique constraints:

```yaml
indexes:
  - name: idx_users_email
    fields: [email]
    unique: true
  - name: idx_users_created_at
    fields: [created_at]
    unique: false
```

### 5. Default Values
Intelligent conversion of SQL defaults to YAML format:

| SQL Default | YAML Default | Description |
|-------------|--------------|-------------|
| `CURRENT_TIMESTAMP` | `now` | Current timestamp |
| `CURRENT_DATE` | `today` | Current date |
| `gen_random_uuid()` | `new_uuid` | PostgreSQL UUID generation |
| `true` | `true` | Boolean true |
| `false` | `false` | Boolean false |
| `''` | `blank` | Empty string |
| `0` | `zero` | Numeric zero |

## Examples

### Basic Usage

```bash
# Extract schema from local PostgreSQL
makemigrations db2schema --database=myapp --username=myuser --password=mypass

# Output
Database schema successfully extracted to: schema.yaml

Extracted 5 tables:
  - users
  - products
  - categories
  - orders
  - order_items

You can now use this schema file with other makemigrations commands.
```

### Verbose Mode

```bash
# Show detailed processing information
makemigrations db2schema --verbose --database=myapp --username=myuser

# Output
Extracting database schema to YAML
==================================
Database type: postgresql
Output file: schema.yaml

1. Connecting to database...
Successfully extracted schema with 5 tables
  - users: 8 fields, 2 indexes
  - products: 6 fields, 3 indexes
  - categories: 4 fields, 1 indexes
  - orders: 7 fields, 2 indexes
  - order_items: 5 fields, 2 indexes

2. Converting to YAML format...

3. Writing YAML schema file...
Database schema successfully extracted to: schema.yaml
```

### Custom Output Location

```bash
# Extract to specific file
makemigrations db2schema --output=extracted_schema.yaml --database=myapp

# Extract to directory
makemigrations db2schema --output=schemas/production.yaml --database=prod_db
```

### Remote Database Connection

```bash
# Connect to remote database
makemigrations db2schema \
  --host=db.example.com \
  --port=5432 \
  --database=production \
  --username=readonly \
  --password=secretpass \
  --sslmode=require \
  --output=production_schema.yaml
```

### Using Environment Variables

```bash
# Set connection via environment
export PGHOST=localhost
export PGPORT=5432
export PGDATABASE=myapp
export PGUSER=myuser
export PGPASSWORD=mypass

# Extract schema
makemigrations db2schema --verbose
```

## Database Support

### PostgreSQL (Full Support)
- ✅ All data types supported
- ✅ Primary keys and foreign keys
- ✅ Unique and regular indexes
- ✅ Default values and constraints
- ✅ Nullable field detection
- ✅ SERIAL and auto-increment fields
- ✅ ON DELETE cascade rules

### Other Databases (Planned)
- 🔄 MySQL - Placeholder implementation
- 🔄 SQLite - Placeholder implementation  
- 🔄 SQL Server - Placeholder implementation
- 🔄 Oracle - Planned for future release

## Generated YAML Structure

The command generates a complete YAML schema file:

```yaml
database:
  name: extracted_schema
  version: 1.0.0

defaults:
  postgresql:
    blank: "''"
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    today: CURRENT_DATE
    zero: "'0'"
    true: "'true'"
    false: "'false'"
    null: null

tables:
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
        nullable: false
      - name: email
        type: varchar
        length: 255
        nullable: false
      - name: username
        type: varchar
        length: 100
        nullable: false
      - name: password_hash
        type: varchar
        length: 255
        nullable: false
      - name: is_active
        type: boolean
        default: true
        nullable: true
      - name: created_at
        type: timestamp
        default: now
        nullable: false
      - name: updated_at
        type: timestamp
        nullable: true
    indexes:
      - name: idx_users_email
        fields: [email]
        unique: true
      - name: idx_users_username
        fields: [username]
        unique: true

  - name: user_profiles
    fields:
      - name: id
        type: serial
        primary_key: true
        nullable: false
      - name: user_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: users
          on_delete: CASCADE
      - name: first_name
        type: varchar
        length: 100
        nullable: true
      - name: last_name
        type: varchar
        length: 100
        nullable: true
      - name: bio
        type: text
        length: 1000
        nullable: true
        default: blank
```

## Type Mapping

### PostgreSQL to YAML Type Conversion

| PostgreSQL Type | YAML Type | Notes |
|-----------------|-----------|-------|
| `VARCHAR(n)` | `varchar` | Length preserved |
| `TEXT` | `text` | |
| `INTEGER` | `integer` | |
| `BIGINT` | `bigint` | |
| `SERIAL` | `serial` | Auto-increment |
| `REAL` | `float` | |
| `NUMERIC(p,s)` | `decimal` | Precision/scale preserved |
| `BOOLEAN` | `boolean` | |
| `DATE` | `date` | |
| `TIME` | `time` | |
| `TIMESTAMP` | `timestamp` | |
| `UUID` | `uuid` | |
| `JSONB` | `jsonb` | |
| `JSON` | `jsonb` | Converted to JSONB |

### Serial Field Detection

The command intelligently detects PostgreSQL SERIAL fields:

```sql
-- PostgreSQL table
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255)
);
```

```yaml
# Generated YAML
- name: id
  type: serial
  primary_key: true
  nullable: false
```

## Connection Security

### SSL Modes

| SSL Mode | Description | Use Case |
|----------|-------------|----------|
| `disable` | No SSL connection | Local development |
| `require` | SSL required, no verification | Basic security |
| `verify-ca` | SSL with CA verification | Trusted networks |
| `verify-full` | Full SSL verification | Production systems |

### Example with SSL

```bash
# Production connection with full SSL verification
makemigrations db2schema \
  --host=prod-db.company.com \
  --port=5432 \
  --database=production \
  --username=readonly \
  --sslmode=verify-full \
  --verbose
```

## Error Handling

### Connection Errors

```bash
$ makemigrations db2schema --host=nonexistent --database=test
Error: failed to extract database schema: failed to ping database: 
dial tcp: lookup nonexistent: no such host
```

### Authentication Errors

```bash
$ makemigrations db2schema --username=invalid --password=wrong
Error: failed to extract database schema: failed to ping database: 
pq: password authentication failed for user "invalid"
```

### Database Access Errors

```bash
$ makemigrations db2schema --database=forbidden
Error: failed to extract database schema: failed to ping database:
pq: permission denied for database "forbidden"
```

### Permission Errors

```bash
$ makemigrations db2schema --username=limited_user
Error: failed to extract database schema: failed to extract tables: 
failed to query tables: pq: permission denied for schema information_schema
```

## Troubleshooting

### Common Issues and Solutions

1. **Connection Refused**
   ```bash
   # Check if PostgreSQL is running
   sudo systemctl status postgresql
   
   # Check port and host
   netstat -tlnp | grep 5432
   ```

2. **Permission Denied**
   ```bash
   # Grant necessary permissions
   GRANT USAGE ON SCHEMA information_schema TO readonly_user;
   GRANT SELECT ON ALL TABLES IN SCHEMA information_schema TO readonly_user;
   ```

3. **Empty Schema Output**
   ```bash
   # Check if tables exist in public schema
   psql -d mydb -c "\dt"
   
   # Check schema search path
   psql -d mydb -c "SHOW search_path;"
   ```

4. **Type Conversion Issues**
   ```bash
   # Use verbose mode to see detailed type processing
   makemigrations db2schema --verbose --database=mydb
   ```

## Integration with Makemigrations

### Workflow Example

```bash
# 1. Extract existing database schema
makemigrations db2schema --database=production --output=baseline_schema.yaml

# 2. Initialize migrations directory with extracted schema
makemigrations init --schema=baseline_schema.yaml

# 3. Make schema modifications
vim schema/schema.yaml

# 4. Generate migration for changes
makemigrations makemigrations --name="add_new_features"

# 5. Apply migrations
goose -dir migrations postgres $DATABASE_URL up
```

### Schema Evolution Workflow

```bash
# Before making changes - capture current state
makemigrations db2schema --database=staging --output=current_state.yaml

# Make schema changes in YAML
vim schema/schema.yaml

# Compare what will change
diff current_state.yaml schema/schema.yaml

# Generate and review migration
makemigrations makemigrations --dry-run
```

## Performance Considerations

### Large Databases

For databases with many tables:

```bash
# Use verbose mode to monitor progress
time makemigrations db2schema --verbose --database=large_db

# Consider extracting specific schemas (future feature)
# makemigrations db2schema --schema=specific_schema --database=large_db
```

### Network Considerations

```bash
# For remote databases, consider connection timeouts
timeout 300 makemigrations db2schema \
  --host=remote-db.example.com \
  --database=myapp \
  --verbose
```

## Configuration Integration

The command respects the configuration file for database type settings:

```yaml
# migrations/makemigrations.config.yaml
database:
  type: postgresql              # Used to determine provider
  default_schema: public        # Schema to extract from
  quote_identifiers: true       # How to handle identifiers
```

### Environment Variable Support

```bash
# Database connection can be specified via environment
export MAKEMIGRATIONS_DATABASE_TYPE=postgresql
export DATABASE_URL=postgres://user:pass@host:port/dbname

# Run extraction
makemigrations db2schema --verbose
```

## Use Cases

### 1. Legacy Database Migration

```bash
# Extract legacy database structure
makemigrations db2schema \
  --host=legacy-db.company.com \
  --database=legacy_system \
  --username=readonly \
  --output=legacy_schema.yaml

# Review and clean up generated schema
vim legacy_schema.yaml

# Initialize new schema-based project
makemigrations init --schema=legacy_schema.yaml
```

### 2. Development Environment Setup

```bash
# Extract production schema
makemigrations db2schema \
  --host=prod-db.company.com \
  --database=production \
  --username=readonly \
  --sslmode=verify-full \
  --output=prod_schema.yaml

# Use for local development
cp prod_schema.yaml schema/schema.yaml
makemigrations init
```

### 3. Documentation Generation

```bash
# Extract schema for documentation
makemigrations db2schema --database=myapp --verbose > schema_extraction.log
makemigrations dump_sql --database=postgresql > schema_structure.sql

# Generate complete documentation
echo "# Database Schema Documentation" > docs/database.md
echo "## Extraction Log" >> docs/database.md
cat schema_extraction.log >> docs/database.md
echo "## SQL Structure" >> docs/database.md
cat schema_structure.sql >> docs/database.md
```

### 4. Schema Comparison

```bash
# Compare staging vs production
makemigrations db2schema --database=staging --output=staging_schema.yaml
makemigrations db2schema --database=production --output=prod_schema.yaml

# Compare schemas
diff staging_schema.yaml prod_schema.yaml
```

### 5. CI/CD Integration

```bash
#!/bin/bash
# ci/extract-schema.sh

# Extract current production schema
makemigrations db2schema \
  --host=$PROD_DB_HOST \
  --database=$PROD_DB_NAME \
  --username=$READONLY_USER \
  --password=$READONLY_PASS \
  --output=current_prod_schema.yaml

# Compare with committed schema
if ! diff current_prod_schema.yaml schema/schema.yaml > /dev/null; then
    echo "Schema drift detected between code and production!"
    echo "Differences:"
    diff current_prod_schema.yaml schema/schema.yaml
    exit 1
fi
```

## Advanced Features

### Custom Database Names

```bash
# Override extracted database name
makemigrations db2schema --database=myapp --output=temp_schema.yaml

# Edit the database section
sed -i 's/extracted_schema/myapp_v2/g' temp_schema.yaml
```

### Selective Table Extraction (Future Feature)

```bash
# Future planned feature
# makemigrations db2schema --tables="users,products,orders" --database=myapp
```

### Schema Filtering (Future Feature)

```bash
# Future planned feature  
# makemigrations db2schema --exclude-tables="audit_*,temp_*" --database=myapp
```

## Migration Path Examples

### From Django to Makemigrations

```bash
# 1. Extract Django database
makemigrations db2schema --database=django_app --output=django_schema.yaml

# 2. Convert Django-specific types (manual review needed)
# - Review auto_now and auto_now_add fields
# - Check CharField max_length mappings
# - Verify foreign key on_delete behaviors

# 3. Initialize makemigrations
makemigrations init --schema=django_schema.yaml
```

### From Rails to Makemigrations  

```bash
# 1. Extract Rails database
makemigrations db2schema --database=rails_app --output=rails_schema.yaml

# 2. Review Rails conventions
# - Convert created_at/updated_at to auto_create/auto_update
# - Review ActiveRecord foreign key naming
# - Check index naming conventions

# 3. Initialize makemigrations
makemigrations init --schema=rails_schema.yaml
```

### From Liquibase to Makemigrations

```bash
# 1. Extract current database state
makemigrations db2schema --database=liquibase_db --output=current_schema.yaml

# 2. Clean up and organize
# - Remove Liquibase system tables from extracted schema
# - Review and consolidate field definitions
# - Standardize naming conventions

# 3. Initialize clean migration history
makemigrations init --schema=current_schema.yaml
```

## Limitations and Considerations

### Current Limitations

1. **Schema Scope**: Only extracts from `public` schema
2. **PostgreSQL Only**: Full support limited to PostgreSQL currently  
3. **Basic Types**: Complex types may map to simpler YAML equivalents
4. **Views**: Database views are not extracted
5. **Procedures**: Stored procedures and functions not included
6. **Triggers**: Database triggers not extracted
7. **Partitioning**: Partitioned tables treated as regular tables

### Data Type Considerations

Some PostgreSQL types have no direct YAML equivalent:
- Arrays → converted to text
- Custom types → converted to text
- Geometric types → converted to text
- Network types → converted to text

### Performance Considerations

- Large databases may take time to process
- Network latency affects remote database extraction
- Complex schemas with many foreign keys require more processing time

## See Also

- [init Command](./init.md) - Initialize new projects with extracted schemas
- [makemigrations Command](./makemigrations.md) - Generate migrations from schemas  
- [dump_sql Command](./dump_sql.md) - View generated SQL from extracted schemas
- [Schema Format Guide](../schema-format.md) - YAML schema syntax reference
- [Configuration Guide](../configuration.md) - Configuration options