# VHS Demo Script for makemigrations
# Demonstrates struct2schema, db2schema, init, and makemigrations subcommands
# https://github.com/charmbracelet/vhs

# Setup
Output demo.gif
Output demo.mp4

Set FontSize 14
Set Width 1200
Set Height 800
Set Theme "Dracula"
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

# Clear and set up demo environment
Type "clear"
Enter
Sleep 1s

# Show title
Type "# makemigrations Demo - YAML-First Database Schema Management"
Enter
Sleep 2s
Type "clear"
Enter

# ========================================
# Part 1: Initialize a new project
# ========================================
Type "# 1. Initialize a new makemigrations project"
Enter
Sleep 1s
Type "makemigrations init --verbose"
Enter
Sleep 3s

Type "# View the created structure"
Enter
Type "tree -a migrations/ schema/"
Enter
Sleep 2s

Type "# Check the generated config file"
Enter
Type "cat migrations/makemigrations.config.yaml | head -20"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 2: struct2schema - Convert Go structs to YAML
# ========================================
Type "# 2. Convert Go structs to YAML schema"
Enter
Sleep 1s

Type "# First, let's create some example Go structs"
Enter
Type "mkdir -p models"
Enter
Type "cat > models/user.go << 'EOF'"
Enter
Type "package models"
Enter
Type ""
Enter
Type "import ("
Enter
Type "    \"time\""
Enter
Type ")"
Enter
Type ""
Enter
Type "type User struct {"
Enter
Type "    ID        uint      \`db:\"id\" gorm:\"primaryKey\"\`"
Enter
Type "    Email     string    \`db:\"email\" gorm:\"not null;unique\"\`"
Enter
Type "    Username  string    \`db:\"username\" gorm:\"not null;unique;size:100\"\`"
Enter
Type "    CreatedAt time.Time \`db:\"created_at\" gorm:\"autoCreateTime\"\`"
Enter
Type "    UpdatedAt time.Time \`db:\"updated_at\" gorm:\"autoUpdateTime\"\`"
Enter
Type "    Posts     []Post    \`gorm:\"foreignKey:AuthorID\"\`"
Enter
Type "}"
Enter
Type ""
Enter
Type "type Post struct {"
Enter
Type "    ID        uint      \`db:\"id\" gorm:\"primaryKey\"\`"
Enter
Type "    Title     string    \`db:\"title\" gorm:\"not null\"\`"
Enter
Type "    Content   string    \`db:\"content\" gorm:\"type:text\"\`"
Enter
Type "    AuthorID  uint      \`db:\"author_id\"\`"
Enter
Type "    Author    User      \`gorm:\"foreignKey:AuthorID;constraint:OnDelete:CASCADE\"\`"
Enter
Type "    Tags      []Tag     \`gorm:\"many2many:post_tags\"\`"
Enter
Type "    CreatedAt time.Time \`db:\"created_at\" gorm:\"autoCreateTime\"\`"
Enter
Type "}"
Enter
Type ""
Enter
Type "type Tag struct {"
Enter
Type "    ID   uint   \`db:\"id\" gorm:\"primaryKey\"\`"
Enter
Type "    Name string \`db:\"name\" gorm:\"not null;unique;size:50\"\`"
Enter
Type "}"
Enter
Type "EOF"
Enter
Sleep 2s

Type "# Convert structs to YAML schema"
Enter
Type "makemigrations struct2schema --input ./models --output schema/from_structs.yaml --verbose"
Enter
Sleep 3s

Type "# View the generated schema"
Enter
Type "cat schema/from_structs.yaml | head -40"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 3: Create a schema from scratch
# ========================================
Type "# 3. Create a more complex schema manually"
Enter
Sleep 1s

Type "cat > schema/ecommerce.yaml << 'EOF'"
Enter
Type "database:"
Enter
Type "  name: ecommerce"
Enter
Type "  version: 1.0.0"
Enter
Type ""
Enter
Type "defaults:"
Enter
Type "  postgresql:"
Enter
Type "    blank: ''"
Enter
Type "    now: CURRENT_TIMESTAMP"
Enter
Type "    new_uuid: gen_random_uuid()"
Enter
Type "    today: CURRENT_DATE"
Enter
Type ""
Enter
Type "tables:"
Enter
Type "  - name: products"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: uuid"
Enter
Type "        primary_key: true"
Enter
Type "        default: new_uuid"
Enter
Type "      - name: name"
Enter
Type "        type: varchar"
Enter
Type "        length: 255"
Enter
Type "        nullable: false"
Enter
Type "      - name: price"
Enter
Type "        type: decimal"
Enter
Type "        precision: 10"
Enter
Type "        scale: 2"
Enter
Type "        nullable: false"
Enter
Type "      - name: stock"
Enter
Type "        type: integer"
Enter
Type "        default: zero"
Enter
Type "        nullable: false"
Enter
Type "      - name: created_at"
Enter
Type "        type: timestamp"
Enter
Type "        default: now"
Enter
Type "        auto_create: true"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_products_name"
Enter
Type "        fields: [name]"
Enter
Type "        unique: false"
Enter
Type ""
Enter
Type "  - name: customers"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: uuid"
Enter
Type "        primary_key: true"
Enter
Type "        default: new_uuid"
Enter
Type "      - name: email"
Enter
Type "        type: varchar"
Enter
Type "        length: 255"
Enter
Type "        nullable: false"
Enter
Type "      - name: name"
Enter
Type "        type: varchar"
Enter
Type "        length: 100"
Enter
Type "        nullable: false"
Enter
Type "      - name: phone"
Enter
Type "        type: varchar"
Enter
Type "        length: 20"
Enter
Type "        nullable: true"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_customers_email"
Enter
Type "        fields: [email]"
Enter
Type "        unique: true"
Enter
Type "EOF"
Enter
Sleep 2s

Type "clear"
Enter

# ========================================
# Part 4: Generate initial migration
# ========================================
Type "# 4. Generate the initial migration from our schemas"
Enter
Sleep 1s

Type "# First, let's see what will be generated (dry run)"
Enter
Type "makemigrations makemigrations --dry-run --verbose"
Enter
Sleep 4s

Type "# Now generate the actual migration"
Enter
Type "makemigrations makemigrations --name \"initial_schema\""
Enter
Sleep 3s

Type "# View the generated migration"
Enter
Type "ls -la migrations/*.sql"
Enter
Sleep 2s

Type "cat migrations/*_initial_schema.sql | head -50"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 5: Modify schema and generate new migration
# ========================================
Type "# 5. Modify schema and generate incremental migration"
Enter
Sleep 1s

Type "# Add a new table and modify existing one"
Enter
Type "cat >> schema/ecommerce.yaml << 'EOF'"
Enter
Type ""
Enter
Type "  - name: orders"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: uuid"
Enter
Type "        primary_key: true"
Enter
Type "        default: new_uuid"
Enter
Type "      - name: customer_id"
Enter
Type "        type: foreign_key"
Enter
Type "        nullable: false"
Enter
Type "        foreign_key:"
Enter
Type "          table: customers"
Enter
Type "          on_delete: RESTRICT"
Enter
Type "      - name: total_amount"
Enter
Type "        type: decimal"
Enter
Type "        precision: 10"
Enter
Type "        scale: 2"
Enter
Type "        nullable: false"
Enter
Type "      - name: status"
Enter
Type "        type: varchar"
Enter
Type "        length: 50"
Enter
Type "        default: \"'pending'\""
Enter
Type "        nullable: false"
Enter
Type "      - name: created_at"
Enter
Type "        type: timestamp"
Enter
Type "        default: now"
Enter
Type "        auto_create: true"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_orders_customer"
Enter
Type "        fields: [customer_id]"
Enter
Type "        unique: false"
Enter
Type "      - name: idx_orders_status"
Enter
Type "        fields: [status]"
Enter
Type "        unique: false"
Enter
Type ""
Enter
Type "  - name: order_items"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: serial"
Enter
Type "        primary_key: true"
Enter
Type "      - name: order_id"
Enter
Type "        type: foreign_key"
Enter
Type "        nullable: false"
Enter
Type "        foreign_key:"
Enter
Type "          table: orders"
Enter
Type "          on_delete: CASCADE"
Enter
Type "      - name: product_id"
Enter
Type "        type: foreign_key"
Enter
Type "        nullable: false"
Enter
Type "        foreign_key:"
Enter
Type "          table: products"
Enter
Type "          on_delete: RESTRICT"
Enter
Type "      - name: quantity"
Enter
Type "        type: integer"
Enter
Type "        nullable: false"
Enter
Type "      - name: price"
Enter
Type "        type: decimal"
Enter
Type "        precision: 10"
Enter
Type "        scale: 2"
Enter
Type "        nullable: false"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_order_items_unique"
Enter
Type "        fields: [order_id, product_id]"
Enter
Type "        unique: true"
Enter
Type "EOF"
Enter
Sleep 2s

Type "# Generate migration for the new changes"
Enter
Type "makemigrations makemigrations --name \"add_orders_tables\" --verbose"
Enter
Sleep 3s

Type "# View the new migration"
Enter
Type "cat migrations/*_add_orders_tables.sql | head -40"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 6: Simulate a destructive change
# ========================================
Type "# 6. Demonstrate handling of destructive changes"
Enter
Sleep 1s

Type "# Let's rename a field and remove another (destructive operations)"
Enter
Type "# First, backup our current schema"
Enter
Type "cp schema/ecommerce.yaml schema/ecommerce.yaml.bak"
Enter
Sleep 1s

Type "# Modify the customers table - rename 'phone' to 'mobile' and add 'address'"
Enter
Type "sed -i 's/name: phone/name: mobile/' schema/ecommerce.yaml"
Enter
Sleep 1s

Type "# Generate migration with destructive changes (using silent mode to auto-accept)"
Enter
Type "makemigrations makemigrations --name \"update_customer_fields\" --silent"
Enter
Sleep 3s

Type "# View the migration with review comments for destructive operations"
Enter
Type "grep -A5 -B5 \"REVIEW\" migrations/*_update_customer_fields.sql || echo \"No destructive operations detected\""
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 7: db2schema - Extract from existing database
# ========================================
Type "# 7. Extract schema from an existing database (db2schema)"
Enter
Sleep 1s

Type "# First, let's create a sample PostgreSQL database with Docker"
Enter
Type "# (For demo purposes, we'll simulate the output)"
Enter
Sleep 1s

Type "# Simulate extracting from a production database"
Enter
Type "makemigrations db2schema --database=myapp --host=localhost --username=postgres --output=schema/extracted.yaml --dry-run"
Enter
Sleep 3s

Type "# In a real scenario, this would connect to your database and extract:"
Enter
Type "# - All tables with their fields"
Enter
Type "# - Foreign key relationships"
Enter
Type "# - Indexes and constraints"
Enter
Type "# - Default values and data types"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Part 8: Advanced features demonstration
# ========================================
Type "# 8. Advanced Features"
Enter
Sleep 1s

Type "# Check if migrations are needed (useful for CI/CD)"
Enter
Type "makemigrations makemigrations --check"
Enter
Type "echo \"Exit code: $?\""
Enter
Sleep 2s

Type "# Generate SQL without creating migration files"
Enter
Type "makemigrations dump_sql --database postgresql"
Enter
Sleep 3s

Type "# View migration history"
Enter
Type "ls -la migrations/*.sql | tail -5"
Enter
Sleep 2s

Type "clear"
Enter

# ========================================
# Part 9: Different database types
# ========================================
Type "# 9. Generate migrations for different database types"
Enter
Sleep 1s

Type "# PostgreSQL (default)"
Enter
Type "MAKEMIGRATIONS_DATABASE_TYPE=postgresql makemigrations makemigrations --dry-run --name \"postgres_migration\" | head -20"
Enter
Sleep 3s

Type "# MySQL"
Enter
Type "MAKEMIGRATIONS_DATABASE_TYPE=mysql makemigrations makemigrations --dry-run --name \"mysql_migration\" | head -20"
Enter
Sleep 3s

Type "# SQLite"
Enter
Type "MAKEMIGRATIONS_DATABASE_TYPE=sqlite makemigrations makemigrations --dry-run --name \"sqlite_migration\" | head -20"
Enter
Sleep 3s

Type "clear"
Enter

# ========================================
# Closing
# ========================================
Type "# makemigrations - YAML-First Database Schema Management"
Enter
Type "# "
Enter
Type "# Key Features Demonstrated:"
Enter
Type "# ✓ init         - Initialize new projects"
Enter
Type "# ✓ struct2schema - Convert Go structs to YAML schemas"
Enter
Type "# ✓ db2schema    - Extract schemas from existing databases"
Enter
Type "# ✓ makemigrations - Generate SQL migrations from YAML changes"
Enter
Type "# "
Enter
Type "# Benefits:"
Enter
Type "# • Version control friendly YAML schemas"
Enter
Type "# • Automatic migration generation"
Enter
Type "# • Support for multiple database types"
Enter
Type "# • Safe handling of destructive operations"
Enter
Type "# • Integration with existing Go projects"
Enter
Type "# "
Enter
Type "# Learn more: https://github.com/ocomsoft/makemigrations"
Enter
Sleep 5s

# End of demo