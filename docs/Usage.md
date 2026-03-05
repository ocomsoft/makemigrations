# makemigrations Usage Guide

A practical, end-to-end walkthrough of the makemigrations workflow using PostgreSQL. This guide covers everything from initial project setup through schema evolution, SQL inspection, data seeding, and stored procedures — including custom type mappings and custom defaults.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Project Setup](#project-setup)
3. [Creating the Initial Schema](#creating-the-initial-schema)
4. [Generating the Initial Migration](#generating-the-initial-migration)
5. [Checking the Generated SQL](#checking-the-generated-sql)
6. [Applying Migrations](#applying-migrations)
7. [Adding a Table](#adding-a-table)
8. [Adding Fields to an Existing Table](#adding-fields-to-an-existing-table)
9. [Adding Indexes](#adding-indexes)
10. [Altering Fields](#altering-fields)
11. [Removing a Field](#removing-a-field)
12. [Removing a Table](#removing-a-table)
13. [Inserting Seed Data](#inserting-seed-data)
14. [Adding a Stored Procedure](#adding-a-stored-procedure)
15. [Rolling Back](#rolling-back)
16. [Day-to-Day Workflow Summary](#day-to-day-workflow-summary)

---

## Prerequisites

- Go 1.24 or later
- makemigrations installed: `go install github.com/ocomsoft/makemigrations@latest`
- A running PostgreSQL instance
- `DATABASE_URL` environment variable set, for example:

```bash
export DATABASE_URL="postgresql://dev_user:dev_pass@localhost:5432/myapp_dev"
```

In the Postgres Server run the following
```sql
CREATE ROLE dev_user LOGIN
  PASSWORD 'dev_pass'
  NOSUPERUSER INHERIT NOCREATEDB NOCREATEROLE NOREPLICATION;

CREATE DATABASE myapp_dev
  WITH OWNER = dev_user
       ENCODING = 'UTF8'
       TABLESPACE = pg_default
       LC_COLLATE = 'en_US.utf8'
       LC_CTYPE = 'en_US.utf8'
       CONNECTION LIMIT = -1;

USE myapp_dev;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```
---

## Project Setup

Initialise the project from your Go application root. This creates the `migrations/` directory with its own Go module, a `main.go` entry point, and a configuration file.

```bash
makemigrations init --database postgresql
```

This produces:

```
myapp/
├── go.mod
├── schema/                     # you create this directory and schema.yaml
└── migrations/
    ├── go.mod                  # module: myapp/migrations
    ├── main.go                 # migrations binary entry point
    └── makemigrations.config.yaml
```

Create the schema directory:

```bash
mkdir -p schema
```

---

## Creating the Initial Schema

The schema is defined in `schema/schema.yaml`. This file describes every table, field, index, and relationship in a database-agnostic way.

The example below demonstrates:

- **Custom defaults** — named aliases for database-specific SQL expressions
- **Custom type mappings** — override how schema field types translate to PostgreSQL SQL types
- **UUID primary keys** using `gen_random_uuid()`
- **Timestamps** that auto-populate on create and update
- **Decimal** fields for monetary values
- **JSONB** fields for flexible metadata
- **Foreign keys** with cascading deletes
- **Indexes** for query performance

Create `schema/schema.yaml`:

```yaml
database:
  name: myapp
  version: 1.0.0

# ---------------------------------------------------------------------------
# Custom type mappings — override the default SQL types for this schema.
# This is useful when you want to use a PostgreSQL-specific type instead of
# the generic default (e.g., CITEXT for case-insensitive text, MONEY for
# currency, DOUBLE PRECISION for higher-precision floats).
# ---------------------------------------------------------------------------
type_mappings:
  postgresql:
    # Use CITEXT so email comparisons are case-insensitive without lower()
    text: "CITEXT"
    # Use DOUBLE PRECISION instead of the default REAL for floats
    float: "DOUBLE PRECISION"

# ---------------------------------------------------------------------------
# Custom defaults — named aliases for database-specific SQL expressions.
# Fields reference these by name (e.g., default: new_uuid) rather than
# embedding raw SQL in the schema, keeping the schema database-agnostic.
# ---------------------------------------------------------------------------
defaults:
  postgresql:
    blank: "''"
    now: "CURRENT_TIMESTAMP"
    new_uuid: "gen_random_uuid()"
    today: "CURRENT_DATE"
    zero: "0"
    true: "true"
    false: "false"
    empty_json: "'{}'"
    # Custom defaults specific to this project
    default_status: "'active'"
    default_role: "'member'"

tables:

  # -------------------------------------------------------------------------
  # users — core account table
  # -------------------------------------------------------------------------
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid           # resolves to gen_random_uuid() at runtime
        nullable: false

      - name: email
        type: text                  # maps to CITEXT (see type_mappings above)
        nullable: false

      - name: username
        type: varchar
        length: 100
        nullable: false

      - name: password_hash
        type: varchar
        length: 255
        nullable: false

      - name: role
        type: varchar
        length: 50
        nullable: false
        default: default_role       # resolves to 'member'

      - name: status
        type: varchar
        length: 50
        nullable: false
        default: default_status     # resolves to 'active'

      - name: metadata
        type: jsonb
        nullable: true
        default: empty_json         # resolves to '{}'

      - name: created_at
        type: timestamp
        nullable: false
        default: now
        auto_create: true           # automatically set on INSERT

      - name: updated_at
        type: timestamp
        nullable: true
        auto_update: true           # automatically set on UPDATE

    indexes:
      - name: idx_users_email
        fields: [email]
        unique: true
      - name: idx_users_username
        fields: [username]
        unique: true
      - name: idx_users_status
        fields: [status]
        unique: false

  # -------------------------------------------------------------------------
  # categories — product categories (self-referencing for hierarchy)
  # -------------------------------------------------------------------------
  - name: categories
    fields:
      - name: id
        type: serial
        primary_key: true

      - name: name
        type: varchar
        length: 100
        nullable: false

      - name: slug
        type: varchar
        length: 100
        nullable: false

      - name: parent_id
        type: foreign_key
        nullable: true
        foreign_key:
          table: categories
          on_delete: SET_NULL       # removing a parent keeps the children

    indexes:
      - name: idx_categories_slug
        fields: [slug]
        unique: true

  # -------------------------------------------------------------------------
  # products — items for sale
  # -------------------------------------------------------------------------
  - name: products
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
        nullable: false

      - name: name
        type: varchar
        length: 255
        nullable: false

      - name: description
        type: text
        nullable: true

      - name: price
        type: decimal
        precision: 10
        scale: 2
        nullable: false

      - name: weight_kg
        type: float                 # maps to DOUBLE PRECISION (see type_mappings)
        nullable: true

      - name: stock_count
        type: integer
        nullable: false
        default: zero               # resolves to 0

      - name: is_active
        type: boolean
        nullable: false
        default: true               # resolves to true

      - name: metadata
        type: jsonb
        nullable: true
        default: empty_json

      - name: created_at
        type: timestamp
        nullable: false
        default: now
        auto_create: true

      - name: updated_at
        type: timestamp
        nullable: true
        auto_update: true

    indexes:
      - name: idx_products_name
        fields: [name]
        unique: false
      - name: idx_products_is_active_created
        fields: [is_active, created_at]
        unique: false

  # -------------------------------------------------------------------------
  # product_categories — many-to-many junction table
  # -------------------------------------------------------------------------
  - name: product_categories
    fields:
      - name: id
        type: serial
        primary_key: true

      - name: product_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: products
          on_delete: CASCADE

      - name: category_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: categories
          on_delete: CASCADE

    indexes:
      - name: idx_product_categories_unique
        fields: [product_id, category_id]
        unique: true                # prevents duplicate associations
```

---

## Generating the Initial Migration

With the schema defined, generate the first migration file:

```bash
makemigrations makemigrations --name "initial"
```

Output:

```
Created migrations/0001_initial.go
```

The generated file (`migrations/0001_initial.go`) looks like this:

```go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0001_initial",
        Dependencies: []string{},
        Operations: []m.Operation{
            &m.SetDefaults{
                Defaults: map[string]string{
                    "blank":          "''",
                    "now":            "CURRENT_TIMESTAMP",
                    "new_uuid":       "gen_random_uuid()",
                    "today":          "CURRENT_DATE",
                    "zero":           "0",
                    "true":           "true",
                    "false":          "false",
                    "empty_json":     "'{}'",
                    "default_status": "'active'",
                    "default_role":   "'member'",
                },
            },
            &m.SetTypeMappings{
                TypeMappings: map[string]string{
                    "text":  "CITEXT",
                    "float": "DOUBLE PRECISION",
                },
            },
            &m.CreateTable{
                Name: "users",
                Fields: []m.Field{
                    {Name: "id",            Type: "uuid",      PrimaryKey: true, Default: "new_uuid"},
                    {Name: "email",         Type: "text"},
                    {Name: "username",      Type: "varchar",   Length: 100},
                    {Name: "password_hash", Type: "varchar",   Length: 255},
                    {Name: "role",          Type: "varchar",   Length: 50,  Default: "default_role"},
                    {Name: "status",        Type: "varchar",   Length: 50,  Default: "default_status"},
                    {Name: "metadata",      Type: "jsonb",     Nullable: true, Default: "empty_json"},
                    {Name: "created_at",    Type: "timestamp", AutoCreate: true, Default: "now"},
                    {Name: "updated_at",    Type: "timestamp", Nullable: true, AutoUpdate: true},
                },
                Indexes: []m.Index{
                    {Name: "idx_users_email",    Fields: []string{"email"},    Unique: true},
                    {Name: "idx_users_username", Fields: []string{"username"}, Unique: true},
                    {Name: "idx_users_status",   Fields: []string{"status"},   Unique: false},
                },
            },
            // ... categories, products, product_categories CreateTable ops
        },
    })
}
```

> **Note:** `SetDefaults` and `SetTypeMappings` are prepended automatically whenever your schema defines those sections. They carry no SQL — they record the configuration in the migration DAG so subsequent migrations and `showsql` use the correct values.

---

## Checking the Generated SQL

Before applying anything, inspect the SQL that will be executed. There are two ways to do this.

### Option 1 — dump_sql (schema preview, no migration state)

`dump_sql` shows the CREATE TABLE statements that your YAML schema would generate, without building the migration binary:

```bash
makemigrations dump_sql --database postgresql
```

Output:

```sql
-- Database: myapp (v1.0.0)
-- Target: postgresql

CREATE TABLE users (
    id         UUID         NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    email      CITEXT       NOT NULL,
    username   VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role       VARCHAR(50)  NOT NULL DEFAULT 'member',
    status     VARCHAR(50)  NOT NULL DEFAULT 'active',
    metadata   JSONB        DEFAULT '{}',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE UNIQUE INDEX idx_users_email    ON users (email);
CREATE UNIQUE INDEX idx_users_username ON users (username);
CREATE        INDEX idx_users_status   ON users (status);

CREATE TABLE categories (
    id        SERIAL PRIMARY KEY,
    name      VARCHAR(100) NOT NULL,
    slug      VARCHAR(100) NOT NULL,
    parent_id INTEGER REFERENCES categories(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX idx_categories_slug ON categories (slug);

CREATE TABLE products (
    id          UUID           NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255)   NOT NULL,
    description CITEXT,
    price       DECIMAL(10,2)  NOT NULL,
    weight_kg   DOUBLE PRECISION,
    stock_count INTEGER        NOT NULL DEFAULT 0,
    is_active   BOOLEAN        NOT NULL DEFAULT true,
    metadata    JSONB          DEFAULT '{}',
    created_at  TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP
);

CREATE INDEX idx_products_name              ON products (name);
CREATE INDEX idx_products_is_active_created ON products (is_active, created_at);

CREATE TABLE product_categories (
    id          SERIAL  PRIMARY KEY,
    product_id  UUID    NOT NULL REFERENCES products(id)   ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_product_categories_unique ON product_categories (product_id, category_id);
```

Notice that:
- `text` fields render as `CITEXT` (from `type_mappings`)
- `float` fields render as `DOUBLE PRECISION` (from `type_mappings`)
- `default: new_uuid` resolves to `gen_random_uuid()` (from `defaults`)
- `default: default_role` resolves to `'member'`

### Option 2 — showsql (migration binary SQL)

After generating the migration file, `showsql` shows exactly what SQL the migration binary will execute when you run `up`. This includes only the pending migrations:

```bash
makemigrations migrate showsql
```

Output:

```sql
-- 0001_initial
CREATE TABLE users ( ... );
CREATE UNIQUE INDEX idx_users_email ON users (email);
-- ... all DDL for all tables and indexes
```

Use `showsql` for final review before production deployments.

---

## Applying Migrations

`makemigrations migrate` compiles the migration binary automatically and runs it:

```bash
makemigrations migrate up
```

Output:

```
Applying 0001_initial... done
```

Check the applied state:

```bash
makemigrations migrate status
```

Output:

```
Migration            Status
---------------------------------
0001_initial         Applied
```

---

## Adding a Table

Suppose you want to add an `orders` table. Edit `schema/schema.yaml` and add the new table definition:

```yaml
  # -------------------------------------------------------------------------
  # orders — customer orders
  # -------------------------------------------------------------------------
  - name: orders
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
        nullable: false

      - name: user_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: users
          on_delete: RESTRICT       # prevent deleting users with orders

      - name: status
        type: varchar
        length: 50
        nullable: false
        default: default_status     # resolves to 'active'

      - name: total_amount
        type: decimal
        precision: 12
        scale: 2
        nullable: false
        default: zero

      - name: notes
        type: text                  # CITEXT via type_mappings
        nullable: true

      - name: placed_at
        type: timestamp
        nullable: false
        default: now
        auto_create: true

    indexes:
      - name: idx_orders_user_id
        fields: [user_id]
        unique: false
      - name: idx_orders_status_placed
        fields: [status, placed_at]
        unique: false
```

Generate the migration:

```bash
makemigrations makemigrations --name "add_orders"
```

Output:

```
Created migrations/0002_add_orders.go
```

The generated file contains a single `CreateTable` operation. Review then apply:

```bash
makemigrations migrate showsql
makemigrations migrate up
```

---

## Adding Fields to an Existing Table

Add a `phone` field and a `last_login_at` timestamp to the `users` table by editing `schema/schema.yaml`:

```yaml
  - name: users
    fields:
      # ... existing fields ...

      - name: phone
        type: varchar
        length: 30
        nullable: true              # nullable so existing rows are not affected

      - name: last_login_at
        type: timestamp
        nullable: true
```

Generate and apply:

```bash
makemigrations makemigrations --name "add_user_phone_and_last_login"
makemigrations migrate up
```

The generated migration uses `AddField` for each new column:

```go
&m.AddField{
    Table: "users",
    Field: m.Field{Name: "phone", Type: "varchar", Length: 30, Nullable: true},
},
&m.AddField{
    Table: "users",
    Field: m.Field{Name: "last_login_at", Type: "timestamp", Nullable: true},
},
```

> **Tip:** Always add new columns as `nullable: true` or with a default value when the table already has rows. Otherwise the `ALTER TABLE ADD COLUMN` will fail on databases that enforce NOT NULL immediately.

---

## Adding Indexes

Add a composite index on `users(role, status)` for a query that filters by both:

```yaml
    indexes:
      # ... existing indexes ...
      - name: idx_users_role_status
        fields: [role, status]
        unique: false
```

Generate and apply:

```bash
makemigrations makemigrations --name "add_user_role_status_index"
makemigrations migrate up
```

Generated operation:

```go
&m.AddIndex{
    Table: "users",
    Index: m.Index{
        Name:   "idx_users_role_status",
        Fields: []string{"role", "status"},
        Unique: false,
    },
},
```

---

## Altering Fields

Suppose you need to expand `status` from `varchar(50)` to `varchar(100)` on the `users` table, and make `phone` non-nullable (after a data backfill).

### Simple type change — expand varchar length

Update the field in `schema/schema.yaml`:

```yaml
      - name: status
        type: varchar
        length: 100               # was 50
        nullable: false
        default: default_status
```

Generate:

```bash
makemigrations makemigrations --name "expand_user_status_length"
```

Generated operation:

```go
&m.AlterField{
    Table:    "users",
    OldField: m.Field{Name: "status", Type: "varchar", Length: 50,  Default: "default_status"},
    NewField: m.Field{Name: "status", Type: "varchar", Length: 100, Default: "default_status"},
},
```

### Safe NOT NULL migration — add column, backfill, tighten

For `phone`, edit the schema to remove `nullable: true`:

```yaml
      - name: phone
        type: varchar
        length: 30
        nullable: false
        default: blank            # resolves to ''
```

After generating, edit the migration file **before applying** to insert a backfill step:

```go
// migrations/0005_make_phone_required.go
Operations: []m.Operation{
    // Step 1: backfill any NULL values with an empty string
    &m.RunSQL{
        ForwardSQL:  "UPDATE users SET phone = '' WHERE phone IS NULL",
        BackwardSQL: "",  // intentionally irreversible
    },
    // Step 2: tighten to NOT NULL
    &m.AlterField{
        Table:    "users",
        OldField: m.Field{Name: "phone", Type: "varchar", Length: 30, Nullable: true},
        NewField: m.Field{Name: "phone", Type: "varchar", Length: 30, Default: "blank"},
    },
},
```

Apply:

```bash
makemigrations migrate showsql   # review the SQL
makemigrations migrate up
```

---

## Removing a Field

Remove the `notes` field from `orders`. Delete it from `schema/schema.yaml`, then generate:

```bash
makemigrations makemigrations --name "remove_order_notes"
```

Because removing a column is destructive (data loss), makemigrations prompts you:

```
  Destructive operation detected: field_removed on "orders.notes"
  1) Generate  — include operation in migration
  2) Review    — include with // REVIEW comment
  3) Omit      — skip operation; schema state still advances (SchemaOnly)
  4) Exit      — cancel migration generation
  5) All       — generate all remaining destructive ops without prompting
Choice [1-5]: 1
```

Choose **1** to generate the drop, or **2** to flag it for peer review first. The generated operation:

```go
&m.DropField{Table: "orders", Field: "notes"},
```

Apply after review:

```bash
makemigrations migrate up
```

---

## Removing a Table

Remove `product_categories` from `schema/schema.yaml` entirely, then generate:

```bash
makemigrations makemigrations --name "remove_product_categories"
```

Again you will be prompted for the destructive operation. The generated operation:

```go
&m.DropTable{Name: "product_categories"},
```

> **Warning:** This permanently drops the table and all its data. Verify with `makemigrations migrate showsql` before running `up`.

---

## Inserting Seed Data

Data migrations use `RunSQL` and are written **by hand** — the diff engine only generates DDL operations. Create a new file in `migrations/`:

```go
// migrations/0008_seed_categories.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0008_seed_categories",
        Dependencies: []string{"0007_remove_product_categories"},
        Operations: []m.Operation{
            &m.RunSQL{
                ForwardSQL: `
INSERT INTO categories (name, slug, parent_id) VALUES
    ('Electronics',       'electronics',       NULL),
    ('Clothing',          'clothing',          NULL),
    ('Books',             'books',             NULL),
    ('Smartphones',       'smartphones',       1),
    ('Laptops',           'laptops',           1),
    ('Men''s Clothing',   'mens-clothing',     2),
    ('Women''s Clothing', 'womens-clothing',   2);
`,
                BackwardSQL: `
DELETE FROM categories
WHERE slug IN (
    'electronics', 'clothing', 'books',
    'smartphones', 'laptops',
    'mens-clothing', 'womens-clothing'
);
`,
            },
        },
    })
}
```

Apply:

```bash
makemigrations migrate up
```

> **Tip:** Keep schema changes (DDL) and data changes (DML) in separate migrations. This makes rollback cleaner and reduces lock contention on large tables.

---

## Adding a Stored Procedure

Stored procedures (and any other PostgreSQL-specific DDL like views, triggers, or custom functions) are added using `RunSQL`. The `BackwardSQL` should drop the procedure so rollback works cleanly.

```go
// migrations/0009_add_calculate_order_total_proc.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0009_add_calculate_order_total_proc",
        Dependencies: []string{"0008_seed_categories"},
        Operations: []m.Operation{
            &m.RunSQL{
                ForwardSQL: `
CREATE OR REPLACE FUNCTION calculate_order_total(p_user_id UUID)
RETURNS TABLE (
    order_id    UUID,
    placed_at   TIMESTAMP,
    item_count  BIGINT,
    total       DECIMAL(12,2)
)
LANGUAGE sql
STABLE
AS $$
    SELECT
        o.id                        AS order_id,
        o.placed_at,
        COUNT(*)                    AS item_count,
        SUM(p.price)                AS total
    FROM orders o
    JOIN products p
        ON p.id = ANY(
            -- placeholder join — replace with your actual order_items table
            ARRAY[]::UUID[]
        )
    WHERE o.user_id = p_user_id
    GROUP BY o.id, o.placed_at
    ORDER BY o.placed_at DESC;
$$;

COMMENT ON FUNCTION calculate_order_total(UUID)
    IS 'Returns a summary of all orders for a given user, including item count and total price.';
`,
                BackwardSQL: `
DROP FUNCTION IF EXISTS calculate_order_total(UUID);
`,
            },
        },
    })
}
```

For a simpler example — a trigger that keeps `updated_at` current without relying on the application layer:

```go
// migrations/0010_add_updated_at_trigger.go
package main

import m "github.com/ocomsoft/makemigrations/migrate"

func init() {
    m.Register(&m.Migration{
        Name:         "0010_add_updated_at_trigger",
        Dependencies: []string{"0009_add_calculate_order_total_proc"},
        Operations: []m.Operation{
            // Create the shared trigger function once
            &m.RunSQL{
                ForwardSQL: `
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;
`,
                BackwardSQL: `
DROP FUNCTION IF EXISTS set_updated_at() CASCADE;
`,
            },
            // Attach the trigger to the users table
            &m.RunSQL{
                ForwardSQL: `
CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
`,
                BackwardSQL: `
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
`,
            },
            // Attach the trigger to the orders table
            &m.RunSQL{
                ForwardSQL: `
CREATE TRIGGER trg_orders_updated_at
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
`,
                BackwardSQL: `
DROP TRIGGER IF EXISTS trg_orders_updated_at ON orders;
`,
            },
        },
    })
}
```

Apply:

```bash
makemigrations migrate showsql   # confirm the SQL looks right
makemigrations migrate up
```

---

## Rolling Back

Roll back the most recent migration:

```bash
makemigrations migrate down
```

Roll back the last three migrations:

```bash
makemigrations migrate down --steps 3
```

Roll back until (but not including) a specific migration:

```bash
makemigrations migrate down --to 0005_make_phone_required
```

Each operation's `Down` reverses the forward change automatically for typed operations (`CreateTable` → `DROP TABLE`, `AddField` → `DROP COLUMN`, etc.). `RunSQL` operations use the `BackwardSQL` you provided.

---

## Day-to-Day Workflow Summary

```
Edit schema/schema.yaml
        │
        ▼
makemigrations makemigrations --name "describe_the_change"
        │
        ▼
(optional) edit the generated .go file to add RunSQL data steps
        │
        ▼
makemigrations migrate showsql          ← review SQL before touching the DB
        │
        ▼
makemigrations migrate up               ← apply
        │
        ▼
makemigrations migrate status           ← verify
```

### Useful Commands Reference

| Command | What it does |
|---------|-------------|
| `makemigrations init` | Bootstrap the `migrations/` directory |
| `makemigrations makemigrations` | Generate a migration from schema changes |
| `makemigrations makemigrations --dry-run` | Preview migration source without writing a file |
| `makemigrations makemigrations --check` | CI mode: exit 1 if migrations are needed |
| `makemigrations makemigrations --merge` | Generate a merge migration for concurrent branches |
| `makemigrations dump_sql` | Show full CREATE TABLE SQL from the YAML schema |
| `makemigrations dump_sql --verbose` | Include processing detail in the output |
| `makemigrations migrate showsql` | Show SQL for all pending migrations |
| `makemigrations migrate up` | Apply all pending migrations |
| `makemigrations migrate up --to NAME` | Apply up to a named migration |
| `makemigrations migrate down` | Roll back the last applied migration |
| `makemigrations migrate down --steps N` | Roll back N migrations |
| `makemigrations migrate status` | Show applied vs pending migrations |
| `makemigrations migrate fake NAME` | Mark a migration applied without running SQL |
| `makemigrations migrate dag` | Print the migration dependency graph |

---

## See Also

- [Schema Format Reference](schema-format.md) — complete YAML schema syntax
- [Migrations Writing Guide](migrations.md) — anatomy of migration files and all operation types
- [init Command](commands/init.md) — detailed init options
- [makemigrations Command](commands/makemigrations.md) — all generation flags
- [migrate Command](commands/migrate.md) — all runtime commands and flags
- [dump_sql Command](commands/dump_sql.md) — schema inspection command
- [Configuration Guide](configuration.md) — full configuration reference
