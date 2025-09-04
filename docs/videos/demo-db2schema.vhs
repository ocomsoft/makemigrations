# VHS Demo - db2schema: Extract Schema from Existing Database
# Shows reverse engineering database schemas

Output demo-db2schema.gif

Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "GitHub Dark"
Set TypingSpeed 50ms

Type "clear"
Enter
Sleep 1s

Type "# db2schema - Extract Schema from Existing Database"
Enter
Sleep 2s
Type "clear"
Enter

# ========================================
# Setup PostgreSQL with Docker
# ========================================
Type "# 1. Setup PostgreSQL database for demo"
Enter
Sleep 1s

Type "# Start PostgreSQL container"
Enter
Type "docker run -d --name demo-postgres \\"
Enter
Type "  -e POSTGRES_PASSWORD=demo \\"
Enter
Type "  -e POSTGRES_DB=demodb \\"
Enter
Type "  -p 5432:5432 \\"
Enter
Type "  postgres:15-alpine"
Enter
Sleep 3s

Type "# Wait for database to be ready"
Enter
Type "sleep 5"
Enter
Sleep 5s

# ========================================
# Create sample database schema
# ========================================
Type "# 2. Create sample database schema"
Enter
Sleep 1s

Type "# Create tables using psql"
Enter
Type "PGPASSWORD=demo psql -h localhost -U postgres -d demodb << 'EOF'"
Enter
Type "-- Create users table"
Enter
Type "CREATE TABLE users ("
Enter
Type "    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),"
Enter
Type "    email VARCHAR(255) NOT NULL UNIQUE,"
Enter
Type "    username VARCHAR(100) NOT NULL UNIQUE,"
Enter
Type "    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,"
Enter
Type "    updated_at TIMESTAMP"
Enter
Type ");"
Enter
Enter
Type "-- Create categories table"
Enter
Type "CREATE TABLE categories ("
Enter
Type "    id SERIAL PRIMARY KEY,"
Enter
Type "    name VARCHAR(100) NOT NULL UNIQUE,"
Enter
Type "    description TEXT"
Enter
Type ");"
Enter
Enter
Type "-- Create products table with foreign key"
Enter
Type "CREATE TABLE products ("
Enter
Type "    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),"
Enter
Type "    name VARCHAR(255) NOT NULL,"
Enter
Type "    price DECIMAL(10,2) NOT NULL,"
Enter
Type "    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,"
Enter
Type "    stock INTEGER DEFAULT 0,"
Enter
Type "    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
Enter
Type ");"
Enter
Enter
Type "-- Create orders table"
Enter
Type "CREATE TABLE orders ("
Enter
Type "    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),"
Enter
Type "    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,"
Enter
Type "    total_amount DECIMAL(10,2) NOT NULL,"
Enter
Type "    status VARCHAR(50) DEFAULT 'pending',"
Enter
Type "    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
Enter
Type ");"
Enter
Enter
Type "-- Create order_items junction table"
Enter
Type "CREATE TABLE order_items ("
Enter
Type "    id SERIAL PRIMARY KEY,"
Enter
Type "    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,"
Enter
Type "    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,"
Enter
Type "    quantity INTEGER NOT NULL,"
Enter
Type "    price DECIMAL(10,2) NOT NULL,"
Enter
Type "    UNIQUE(order_id, product_id)"
Enter
Type ");"
Enter
Enter
Type "-- Create indexes"
Enter
Type "CREATE INDEX idx_products_category ON products(category_id);"
Enter
Type "CREATE INDEX idx_orders_user ON orders(user_id);"
Enter
Type "CREATE INDEX idx_orders_status ON orders(status);"
Enter
Type "CREATE INDEX idx_order_items_order ON order_items(order_id);"
Enter
Type "CREATE INDEX idx_order_items_product ON order_items(product_id);"
Enter
Enter
Type "-- Show created tables"
Enter
Type "\\dt"
Enter
Type "EOF"
Enter
Sleep 5s

# ========================================
# Extract schema using db2schema
# ========================================
Type "clear"
Enter
Type "# 3. Extract schema from database using db2schema"
Enter
Sleep 1s

Type "# Create output directory"
Enter
Type "mkdir -p extracted"
Enter
Sleep 1s

Type "# Run db2schema to extract the schema"
Enter
Type "./makemigrations db2schema \\"
Enter
Type "  --host localhost \\"
Enter
Type "  --port 5432 \\"
Enter
Type "  --database demodb \\"
Enter
Type "  --username postgres \\"
Enter
Type "  --password demo \\"
Enter
Type "  --output extracted/schema.yaml \\"
Enter
Type "  --verbose"
Enter
Sleep 4s

Type "# View extracted schema structure"
Enter
Type "cat extracted/schema.yaml | head -50"
Enter
Sleep 3s

Type "# Check the users table definition"
Enter
Type "grep -A20 \"name: users\" extracted/schema.yaml"
Enter
Sleep 3s

Type "# Check foreign key relationships"
Enter
Type "grep -B2 -A3 \"foreign_key:\" extracted/schema.yaml | head -20"
Enter
Sleep 3s

# ========================================
# Use extracted schema
# ========================================
Type "clear"
Enter
Type "# 4. Use extracted schema with makemigrations"
Enter
Sleep 1s

Type "# Initialize new project with extracted schema"
Enter
Type "mkdir -p migrated && cd migrated"
Enter
Type "go mod init migrated"
Enter
Sleep 1s

Type "../makemigrations init"
Enter
Sleep 2s

Type "# Copy extracted schema"
Enter
Type "cp ../extracted/schema.yaml schema/"
Enter
Sleep 1s

Type "# Generate initial migration from extracted schema"
Enter
Type "../makemigrations makemigrations --name \"from_existing_db\""
Enter
Sleep 3s

Type "# View generated migration"
Enter
Type "ls -la migrations/*.sql"
Enter
Sleep 1s

Type "cat migrations/*_from_existing_db.sql | head -30"
Enter
Sleep 3s

# ========================================
# Demonstrate schema evolution
# ========================================
Type "clear"
Enter
Type "# 5. Evolve the extracted schema"
Enter
Sleep 1s

Type "# Add a new table to the extracted schema"
Enter
Type "cat >> schema/schema.yaml << 'EOF'"
Enter
Enter
Type "  - name: reviews"
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
Type "          on_delete: CASCADE"
Enter
Type "      - name: user_id"
Enter
Type "        type: foreign_key"
Enter
Type "        nullable: false"
Enter
Type "        foreign_key:"
Enter
Type "          table: users"
Enter
Type "          on_delete: CASCADE"
Enter
Type "      - name: rating"
Enter
Type "        type: integer"
Enter
Type "        nullable: false"
Enter
Type "      - name: comment"
Enter
Type "        type: text"
Enter
Type "      - name: created_at"
Enter
Type "        type: timestamp"
Enter
Type "        default: now"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_reviews_product"
Enter
Type "        fields: [product_id]"
Enter
Type "      - name: idx_reviews_user"
Enter
Type "        fields: [user_id]"
Enter
Type "EOF"
Enter
Sleep 2s

Type "# Generate migration for new changes"
Enter
Type "../makemigrations makemigrations --name \"add_reviews\""
Enter
Sleep 2s

Type "# View the incremental migration"
Enter
Type "cat migrations/*_add_reviews.sql"
Enter
Sleep 3s

# ========================================
# Clean up
# ========================================
Type "clear"
Enter
Type "# db2schema Features Demonstrated:"
Enter
Type "# ✓ Extract complete schema from PostgreSQL"
Enter
Type "# ✓ Preserve table structures and data types"
Enter
Type "# ✓ Capture foreign key relationships"
Enter
Type "# ✓ Extract indexes and constraints"
Enter
Type "# ✓ Generate YAML schema compatible with makemigrations"
Enter
Type "# ✓ Enable schema evolution from existing databases"
Enter
Sleep 1s
Enter
Type "# Supported databases:"
Enter
Type "# • PostgreSQL (full support)"
Enter
Type "# • MySQL (planned)"
Enter
Type "# • SQLite (planned)"
Enter
Type "# • SQL Server (planned)"
Enter
Sleep 3s

Type "# Clean up"
Enter
Type "cd .."
Enter
Type "docker stop demo-postgres && docker rm demo-postgres"
Enter
Type "rm -rf extracted migrated"
Enter
Sleep 2s