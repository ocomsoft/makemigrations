# Database Schema YAML Format

This document defines the YAML format for representing database schemas that can be used to generate SQL CREATE TABLE statements from Django model definitions.

## Overview

The YAML format is designed to be:
- Database-agnostic but easily convertible to specific SQL dialects
- Human-readable and easy to validate
- Comprehensive enough to capture all necessary SQL information
- Clear in handling relationships (foreign keys, many-to-many)

## YAML Structure

### Root Level

```yaml
database:
  name: string          # Database/application name
  version: string       # Schema version

defaults:               # Default value mappings per database type (12 databases supported)
  mysql:
        blank: ''''''
        array: ('[]')
        object: ('{}')
        zero: "0"
        current_time: (CURTIME())
        new_uuid: (UUID())
        now: CURRENT_TIMESTAMP
        today: (CURDATE())
        false: "0"
        "null": "null"
        true: "1"
    postgresql:
        blank: ''''''
        array: '''[]''::jsonb'
        object: '''{}''::jsonb'
        zero: "0"
        current_time: CURRENT_TIME
        new_uuid: gen_random_uuid()
        now: CURRENT_TIMESTAMP
        today: CURRENT_DATE
        false: "false"
        "null": "null"
        true: "true"
    sqlite:
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        current_time: CURRENT_TIME
        new_uuid: ""
        now: CURRENT_TIMESTAMP
        today: CURRENT_DATE
        false: "0"
        "null": "null"
        true: "1"
    sqlserver:
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        current_time: CAST(GETDATE() AS TIME)
        new_uuid: NEWID()
        now: GETDATE()
        today: CAST(GETDATE() AS DATE)
        false: "0"
        "null": "null"
        true: "1"
    # Extended database support (provider-tested only)
    redshift:       # Amazon Redshift (PostgreSQL-based)
        blank: ''''''
        array: '''[]''::super'
        object: '''{}''::super'
        zero: "0"
        new_uuid: gen_random_uuid()
        now: CURRENT_TIMESTAMP
        false: "false"
        true: "true"
    clickhouse:     # ClickHouse columnar database
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        now: now()
        false: "0"
        true: "1"
    tidb:          # TiDB MySQL-compatible
        blank: ''''''
        array: ('[]')
        object: ('{}')
        zero: "0"
        new_uuid: (UUID())
        now: CURRENT_TIMESTAMP
        false: "false"
        true: "true"
    vertica:       # HP Vertica analytics
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        now: CURRENT_TIMESTAMP
        false: "false"
        true: "true"
    ydb:           # Yandex Database
        blank: ''''''
        zero: "0"
        now: CurrentUtcDatetime()
        false: "false"
        true: "true"
    turso:         # Turso edge SQLite
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        now: CURRENT_TIMESTAMP
        false: "0"
        true: "1"
    starrocks:     # StarRocks MPP analytics
        blank: ''''''
        array: '''[]'''
        object: '''{}'''
        zero: "0"
        now: now()
        false: "false"
        true: "true"
    auroradsql:    # AWS Aurora DSQL
        blank: ''''''
        array: '''[]''::jsonb'
        object: '''{}''::jsonb'
        zero: "0"
        new_uuid: gen_random_uuid()
        now: CURRENT_TIMESTAMP
        false: "false"
        true: "true"

tables:
  - name: string        # Table name
    fields:             # List of field definitions
      - ...
```

### Field Definition

```yaml
- name: string          # Field/column name
  type: string          # Field type (see Type Mapping below)
  primary_key: boolean  # Optional, defaults to false
  nullable: boolean     # Optional, defaults to true
  default: string       # Optional, default value
  length: integer       # Optional, for varchar fields
  precision: integer    # Optional, for decimal fields
  scale: integer        # Optional, for decimal fields
  auto_create: boolean  # Optional, auto-set on creation (e.g., created_date)
  auto_update: boolean  # Optional, auto-update on modification (e.g., modified_date)
  
  # For foreign key fields
  foreign_key:
    table: string       # Referenced table name
    display_field: string  # Field to display in UI
    on_delete: string   # CASCADE, RESTRICT, SET_NULL, PROTECT
    
  # For many-to-many fields
  many_to_many:
    table: string       # Target table name
```

## Type Mapping (Django → YAML)

| Django Field Type | YAML Type | SQL Equivalent | Notes |
|-------------------|-----------|----------------|-------|
| CharField | varchar | VARCHAR(n) | Requires `length` |
| TextField | text | TEXT | |
| IntegerField | integer | INTEGER | |
| BigIntegerField | bigint | BIGINT | |
| FloatField | float | FLOAT | |
| DecimalField | decimal | DECIMAL(p,s) | Requires `precision` and `scale` |
| BooleanField | boolean | BOOLEAN | |
| DateField | date | DATE | |
| DateTimeField | timestamp | TIMESTAMP | |
| TimeField | time | TIME | |
| UUIDField | uuid | UUID | |
| JSONField | jsonb | JSONB | |
| EmailField | varchar | VARCHAR(255) | Treated as varchar with validation |
| URLField | varchar | VARCHAR(255) | Treated as varchar with validation |
| ForeignKey | foreign_key | INTEGER/UUID | Actual type determined by referenced table's PK |
| ManyToManyField | many_to_many | N/A | Creates junction table |

## Default Value Mappings

The `default` field in the YAML schema uses standardized values that are mapped to database-specific SQL during generation. The actual SQL is defined in the `defaults` section at the root of the YAML file, organized by database type.

### How It Works

1. **Field Definition**: A field specifies a standardized default value (e.g., `default: Now`)
2. **Database Selection**: The SQL generator selects the appropriate database section from the `defaults` mapping
3. **SQL Generation**: The standardized value is looked up and replaced with the database-specific SQL

For example:
- Field has `default: Now`
- For PostgreSQL: generates `DEFAULT CURRENT_TIMESTAMP`
- For SQL Server: generates `DEFAULT GETDATE()`

### Standard Default Values

| YAML Default | Description | Django Equivalent |
|--------------|-------------|-------------------|
| `Now` | Current date and time | `timezone.now` |
| `Today` | Current date only | `datetime.date.today` |
| `CurrentTime` | Current time only | `timezone.now().time()` |
| `NewUUID` | Generate UUID v4 | `uuid.uuid4` |
| `""` | Empty string | `""` |
| `0` | Zero (numeric) | `0` |
| `false` | Boolean false | `False` |
| `true` | Boolean true | `True` |
| `null` | NULL value | `None` |
| `[]` | Empty array/list | `list` |
| `{}` | Empty object/dict | `dict` |

### Database-Specific SQL Generation

#### PostgreSQL

| YAML Default | PostgreSQL SQL |
|--------------|----------------|
| `Now` | `CURRENT_TIMESTAMP` |
| `Today` | `CURRENT_DATE` |
| `CurrentTime` | `CURRENT_TIME` |
| `NewUUID` | `gen_random_uuid()` |
| `""` | `''` |
| `0` | `0` |
| `false` | `false` |
| `true` | `true` |
| `null` | null |
| `[]` | `'[]'::jsonb` |
| `{}` | `'{}'::jsonb` |

#### SQL Server

| YAML Default | SQL Server SQL |
|--------------|----------------|
| `Now` | `GETDATE()` |
| `Today` | `CAST(GETDATE() AS DATE)` |
| `CurrentTime` | `CAST(GETDATE() AS TIME)` |
| `NewUUID` | `NEWID()` |
| `""` | `''` |
| `0` | `0` |
| `false` | `0` |
| `true` | `1` |
| `null` | null |
| `[]` | `'[]'` |
| `{}` | `'{}'` |

#### MySQL

| YAML Default | MySQL SQL |
|--------------|-----------|
| `Now` | `CURRENT_TIMESTAMP` |
| `Today` | `(CURDATE())` |
| `CurrentTime` | `(CURTIME())` |
| `NewUUID` | `(UUID())` |
| `""` | `''` |
| `0` | `0` |
| `false` | `0` |
| `true` | `1` |
| `null` | null |
| `[]` | `('[]')` |
| `{}` | `('{}')` |

#### SQLite

| YAML Default | SQLite SQL |
|--------------|------------|
| `Now` | `CURRENT_TIMESTAMP` |
| `Today` | `CURRENT_DATE` |
| `CurrentTime` | `CURRENT_TIME` |
| `NewUUID` | ""  # requires application-level generation |
| `""` | `''` |
| `0` | `0` |
| `false` | `0` |
| `true` | `1` |
| `null` | null |
| `[]` | `'[]'` |
| `{}` | `'{}'` |

## Example YAML Schema

```yaml
database:
  name: app
  version: 1.0.0

defaults:
  postgresql:
    Now: "CURRENT_TIMESTAMP"
    Today: "CURRENT_DATE"
    CurrentTime: "CURRENT_TIME"
    NewUUID: "gen_random_uuid()"
    "": "''"
    "0": "0"
    "false": "false"
    "true": "true"
    "null": "null"
    "[]": "'[]'::jsonb"
    "{}": "'{}'::jsonb"
  
  sqlserver:
    Now: "GETDATE()"
    Today: "CAST(GETDATE() AS DATE)"
    CurrentTime: "CAST(GETDATE() AS TIME)"
    NewUUID: "NEWID()"
    "": "''"
    "0": "0"
    "false": "0"
    "true": "1"
    "null": "null"
    "[]": "'[]'"
    "{}": "'{}'"
  
tables:
  - name: tenant
    fields:
      - name: id
        type: uuid
        primary_key: true
        nullable: false
        default: NewUUID
        
      - name: active_start_date
        type: timestamp
        nullable: true
        default: Now
        
      - name: active_end_date
        type: timestamp
        nullable: true
        
      - name: name
        type: varchar
        length: 255
        nullable: false
        
      - name: code
        type: varchar
        length: 255
        nullable: false
        
      - name: company_code
        type: varchar
        length: 255
        nullable: false
    
  - name: misc_model
    fields:
      - name: id
        type: serial
        primary_key: true
        
      - name: description
        type: varchar
        length: 255
        nullable: false
        default: ""
        
      - name: a_file
        type: foreign_key
        nullable: true
        foreign_key:
          table: filesystem.FileMetaData
          display_field: original_filename
          on_delete: PROTECT
          
      - name: a_user
        type: foreign_key
        nullable: true
        foreign_key:
          table: auth.User
          display_field: username
          on_delete: PROTECT
          
      - name: a_json
        type: jsonb
        nullable: true
        
      - name: an_example
        type: foreign_key
        nullable: true
        foreign_key:
          table: CodeExample
          display_field: description
          on_delete: PROTECT
          
      - name: examples
        type: many_to_many
        many_to_many:
          table: ChildExample
          
      - name: tenant_id
        type: uuid
        nullable: false

  - name: related_model
    fields:
      - name: string_example
        type: foreign_key
        nullable: true
        foreign_key:
          table: StringsModel
          display_field: description
          on_delete: PROTECT
          
      - name: a_file
        type: foreign_key
        nullable: true
        foreign_key:
          table: filesystem.FileMetaData
          display_field: original_filename
          on_delete: PROTECT
          
      - name: description
        type: varchar
        length: 255
        nullable: false
        default: ""
        
      - name: tenant_id
        type: uuid
        nullable: false
```

## Special Considerations

### Auto-Generated Fields

Fields with special behaviors:

```yaml
- name: created_date
  type: timestamp
  nullable: false
  auto_create: true
  default: Now

- name: modified_date
  type: timestamp
  nullable: true
  auto_update: true
```

### Primary Keys

If no field is explicitly marked as `primary_key: true`, the SQL generator should assume an auto-incrementing `id` field:

```yaml
- name: id
  type: serial
  primary_key: true
```

### Many-to-Many Relationships

Many-to-many fields don't create columns in the main table but instead create junction tables. The SQL generator should create a separate table with the naming convention: `{source_table}_{field_name}`.

### Foreign Key Resolution

For foreign key fields, the actual SQL column type (INTEGER, UUID, etc.) should be determined by examining the referenced table's primary key type during SQL generation.

## Usage

This YAML format serves as an intermediate representation that can be:

1. **Generated from Django models** (like the builder.json format)
2. **Used to generate SQL CREATE TABLE statements** for various database systems
3. **Version controlled** to track schema changes
4. **Validated** against the schema definition
5. **Used for documentation** and schema visualization

The format provides enough information to generate complete SQL schemas while remaining readable and maintainable.
