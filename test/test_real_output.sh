#!/bin/bash

# Create temporary directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Create go.mod
cat > go.mod <<'EOF'
module test.com/testproject

go 1.21
EOF

# Create schema directory and file
mkdir -p schema
cat > schema/schema.yaml <<'EOF'
database:
  name: test_defaults
  version: 1.0.0

defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    today: CURRENT_DATE
    current_time: CURRENT_TIME
    new_uuid: gen_random_uuid()
    zero: "0"
    true: "true"
    false: "false"
    null: "NULL"
    array: "'[]'::jsonb"
    object: "'{}'::jsonb"

tables:
  - name: test_table
    fields:
      - name: id
        type: serial
        primary_key: true
      - name: int_field
        type: integer
        nullable: false
        default: zero
      - name: bool_field
        type: boolean
        nullable: false
        default: true
      - name: timestamp_field
        type: timestamp
        nullable: false
        default: now
      - name: uuid_field
        type: uuid
        nullable: false
        default: new_uuid
      - name: literal_string
        type: varchar
        length: 100
        nullable: false
        default: "hello"
EOF

# Run makemigrations init
echo "Running makemigrations init..."
/workspaces/ocom/go/makemigrations/makemigrations init --database postgresql

# Show the generated migration
echo -e "\n=== Generated Migration ==="
cat migrations/*.sql | grep -E "(int_field|bool_field|timestamp_field|uuid_field|literal_string)" || cat migrations/*.sql

# Cleanup
cd /
rm -rf "$TEMP_DIR"