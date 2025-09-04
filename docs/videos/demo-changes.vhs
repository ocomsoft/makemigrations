# VHS Demo - Different Types of Schema Changes
# Shows various migration scenarios

Output demo-changes.gif

Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Monokai Pro"
Set TypingSpeed 60ms

Type "clear"
Enter
Sleep 1s

Type "# makemigrations - Schema Change Scenarios"
Enter
Sleep 2s
Type "clear"
Enter

# Setup
Type "# Setup demo environment"
Enter
Type "mkdir -p demo-changes && cd demo-changes"
Enter
Type "go mod init demo"
Enter
Sleep 1s

Type "../makemigrations init"
Enter
Sleep 2s

# ========================================
# Scenario 1: Adding tables and fields
# ========================================
Type "# Scenario 1: Adding new tables and fields"
Enter
Sleep 1s

Type "cat > schema/schema.yaml << 'EOF'"
Enter
Type "database:"
Enter
Type "  name: demo"
Enter
Type "  version: 1.0.0"
Enter
Enter
Type "defaults:"
Enter
Type "  postgresql:"
Enter
Type "    now: CURRENT_TIMESTAMP"
Enter
Type "    new_uuid: gen_random_uuid()"
Enter
Enter
Type "tables:"
Enter
Type "  - name: users"
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
Type "EOF"
Enter
Sleep 2s

Type "# Generate initial migration"
Enter
Type "../makemigrations makemigrations --name \"create_users\""
Enter
Sleep 2s

Type "# Add a new field to existing table"
Enter
Type "cat >> schema/schema.yaml << 'EOF'"
Enter
Type "      - name: created_at"
Enter
Type "        type: timestamp"
Enter
Type "        default: now"
Enter
Type "EOF"
Enter
Sleep 1s

Type "../makemigrations makemigrations --name \"add_created_at\""
Enter
Sleep 2s

Type "cat migrations/*_add_created_at.sql"
Enter
Sleep 3s

# ========================================
# Scenario 2: Adding indexes
# ========================================
Type "clear"
Enter
Type "# Scenario 2: Adding indexes"
Enter
Sleep 1s

Type "cat >> schema/schema.yaml << 'EOF'"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_users_email"
Enter
Type "        fields: [email]"
Enter
Type "        unique: true"
Enter
Type "EOF"
Enter
Sleep 1s

Type "../makemigrations makemigrations --name \"add_email_index\""
Enter
Sleep 2s

Type "cat migrations/*_add_email_index.sql"
Enter
Sleep 3s

# ========================================
# Scenario 3: Foreign keys and relationships
# ========================================
Type "clear"
Enter
Type "# Scenario 3: Foreign keys and relationships"
Enter
Sleep 1s

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
Type "      - name: content"
Enter
Type "        type: text"
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
Type "    indexes:"
Enter
Type "      - name: idx_posts_user"
Enter
Type "        fields: [user_id]"
Enter
Type "EOF"
Enter
Sleep 2s

Type "../makemigrations makemigrations --name \"add_posts_table\""
Enter
Sleep 2s

Type "cat migrations/*_add_posts_table.sql | head -20"
Enter
Sleep 3s

# ========================================
# Scenario 4: Many-to-many relationships
# ========================================
Type "clear"
Enter
Type "# Scenario 4: Many-to-many relationships"
Enter
Sleep 1s

Type "cat >> schema/schema.yaml << 'EOF'"
Enter
Enter
Type "  - name: tags"
Enter
Type "    fields:"
Enter
Type "      - name: id"
Enter
Type "        type: serial"
Enter
Type "        primary_key: true"
Enter
Type "      - name: name"
Enter
Type "        type: varchar"
Enter
Type "        length: 50"
Enter
Type "        nullable: false"
Enter
Enter
Type "  - name: post_tags"
Enter
Type "    fields:"
Enter
Type "      - name: post_id"
Enter
Type "        type: foreign_key"
Enter
Type "        foreign_key:"
Enter
Type "          table: posts"
Enter
Type "          on_delete: CASCADE"
Enter
Type "      - name: tag_id"
Enter
Type "        type: foreign_key"
Enter
Type "        foreign_key:"
Enter
Type "          table: tags"
Enter
Type "          on_delete: CASCADE"
Enter
Type "    indexes:"
Enter
Type "      - name: idx_post_tags_unique"
Enter
Type "        fields: [post_id, tag_id]"
Enter
Type "        unique: true"
Enter
Type "EOF"
Enter
Sleep 2s

Type "../makemigrations makemigrations --name \"add_tags_m2m\""
Enter
Sleep 2s

Type "# View the junction table creation"
Enter
Type "cat migrations/*_add_tags_m2m.sql | grep -A10 \"post_tags\""
Enter
Sleep 3s

# ========================================
# Scenario 5: Field modifications (destructive)
# ========================================
Type "clear"
Enter
Type "# Scenario 5: Destructive changes (with review comments)"
Enter
Sleep 1s

Type "# Simulate removing a field (destructive operation)"
Enter
Type "# In real usage, this would prompt for confirmation"
Enter
Type "../makemigrations makemigrations --dry-run --silent --name \"example_destructive\""
Enter
Sleep 3s

# ========================================
# Summary
# ========================================
Type "clear"
Enter
Type "# Migration Scenarios Demonstrated:"
Enter
Type "# ✓ Adding new tables"
Enter
Type "# ✓ Adding fields to existing tables"
Enter
Type "# ✓ Creating indexes (unique and regular)"
Enter
Type "# ✓ Foreign key relationships"
Enter
Type "# ✓ Many-to-many relationships with junction tables"
Enter
Type "# ✓ Handling destructive operations"
Enter
Sleep 1s
Enter
Type "# Generated migrations:"
Enter
Type "ls -1 migrations/*.sql"
Enter
Sleep 3s

Type "# Clean up"
Enter
Type "cd .. && rm -rf demo-changes"
Enter
Sleep 2s