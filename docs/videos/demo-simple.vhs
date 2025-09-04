# Simple VHS Demo Script for makemigrations
# Demonstrates core functionality with actual runnable commands

Output demo-simple.gif
Output demo-simple.mp4

Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set TypingSpeed 75ms

# Clear screen
Type "clear"
Enter
Sleep 1s

Type "# makemigrations Demo - Core Features"
Enter
Sleep 2s
Type "clear"
Enter

# Create a demo directory
Type "# Setup demo environment"
Enter
Type "mkdir -p demo && cd demo"
Enter
Sleep 1s

Type "# Initialize Go module"
Enter
Type "go mod init demo"
Enter
Sleep 2s

# ========================================
# Initialize project
# ========================================
Type "# 1. Initialize makemigrations project"
Enter
Sleep 1s
Type "../makemigrations init --verbose"
Enter
Sleep 3s

Type "# View created structure"
Enter
Type "ls -la"
Enter
Sleep 2s

Type "tree migrations schema"
Enter
Sleep 2s

# ========================================
# Create Go structs for struct2schema
# ========================================
Type "# 2. Create Go structs to convert"
Enter
Type "mkdir models"
Enter
Type "cat > models/user.go << 'EOF'"
Enter
Type "package models"
Enter
Enter
Type "import \"time\""
Enter
Enter
Type "type User struct {"
Enter
Type "    ID        uint      \`db:\"id\"\`"
Enter
Type "    Email     string    \`db:\"email\"\`"
Enter
Type "    Username  string    \`db:\"username\"\`"
Enter
Type "    CreatedAt time.Time \`db:\"created_at\"\`"
Enter
Type "}"
Enter
Type "EOF"
Enter
Sleep 2s

# ========================================
# Run struct2schema
# ========================================
Type "# 3. Convert structs to YAML schema"
Enter
Type "../makemigrations struct2schema --input ./models --verbose"
Enter
Sleep 3s

Type "# View generated schema"
Enter
Type "cat schema/schema.yaml | head -30"
Enter
Sleep 3s

# ========================================
# Generate first migration
# ========================================
Type "# 4. Generate initial migration"
Enter
Type "../makemigrations makemigrations --name \"initial\""
Enter
Sleep 3s

Type "# View migration"
Enter
Type "ls migrations/*.sql"
Enter
Sleep 1s
Type "cat migrations/*_initial.sql | head -25"
Enter
Sleep 3s

# ========================================
# Modify schema
# ========================================
Type "# 5. Add a new table to schema"
Enter
Type "cat >> schema/schema.yaml << 'EOF'"
Enter
Enter
Type "  - name: posts"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: serial"
Enter
Type "        primary_key: true"
Enter
Type "      - name: title"
Enter
Type "        type: varchar"
Enter
Type "        length: 255"
Enter
Type "        nullable: false"
Enter
Type "      - name: user_id"
Enter
Type "        type: foreign_key"
Enter
Type "        foreign_key:"
Enter
Type "          table: user"
Enter
Type "          on_delete: CASCADE"
Enter
Type "EOF"
Enter
Sleep 2s

# ========================================
# Generate incremental migration
# ========================================
Type "# 6. Generate migration for changes"
Enter
Type "../makemigrations makemigrations --name \"add_posts\""
Enter
Sleep 3s

Type "# View new migration"
Enter
Type "cat migrations/*_add_posts.sql"
Enter
Sleep 3s

# ========================================
# Check status
# ========================================
Type "# 7. Check migration status"
Enter
Type "../makemigrations makemigrations --check"
Enter
Type "echo \"No pending migrations (exit code: $?)\""
Enter
Sleep 2s

# ========================================
# Clean up
# ========================================
Type "# Demo complete!"
Enter
Type "cd .. && rm -rf demo"
Enter
Sleep 2s

Type "# Key commands demonstrated:"
Enter
Type "# • init - Initialize project"
Enter
Type "# • struct2schema - Convert Go to YAML"
Enter
Type "# • makemigrations - Generate migrations"
Enter
Type "# • --check - Verify migration status"
Enter
Sleep 5s