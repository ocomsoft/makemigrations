# Architecture Documentation

## Overview

Makemigrations is a YAML-first database migration tool for Go that generates SQL migrations from declarative schema definitions. It follows a Django-inspired approach while leveraging Go's ecosystem and supporting 12 different database engines.

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     User Interface Layer                     │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   CLI    │  │  Config  │  │  Cobra   │  │  Viper   │   │
│  │ Commands │  │  Files   │  │  Router  │  │  Config  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────────┐
│                      Core Processing Layer                   │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Scanner │  │  Parser  │  │  Merger  │  │ Analyzer │   │
│  │          │  │          │  │          │  │          │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   Diff   │  │Generator │  │  State   │  │  Writer  │   │
│  │  Engine  │  │          │  │ Manager  │  │          │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────────┐
│                    Database Provider Layer                   │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │PostgreSQL│  │  MySQL   │  │  SQLite  │  │SQLServer │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Redshift │  │ClickHouse│  │   TiDB   │  │ Vertica  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   YDB    │  │  Turso   │  │StarRocks │  │AuroraDSQL│   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                               │
┌─────────────────────────────────────────────────────────────┐
│                      External Systems                        │
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Goose   │  │    Go    │  │   File   │  │ Database │   │
│  │Migration │  │  Module  │  │  System  │  │  Engines │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Command Layer (`/cmd`)

The command layer implements the CLI interface using Cobra framework:

- **root.go**: Main command orchestrator, defaults to `makemigrations_sql`
- **init.go**: Project initialization with YAML schema setup
- **makemigrations.go**: Primary YAML migration generation
- **struct2schema.go**: Go struct to YAML schema conversion
- **goose.go**: Goose migration tool integration
- **dump_sql.go**: SQL generation without migration files
- **db2schema.go**: Database reverse engineering
- **schema2diagram.go**: Visual schema representation

### 2. Configuration System (`/internal/config`)

Hierarchical configuration management with multiple sources:

```go
type Config struct {
    Database  DatabaseConfig  // Database type and settings
    Migration MigrationConfig // Migration generation settings
    Schema    SchemaConfig    // Schema processing settings
    Output    OutputConfig    // Display and logging settings
}
```

**Configuration Priority (highest to lowest):**
1. Command-line flags
2. Environment variables (MAKEMIGRATIONS_*)
3. Configuration file (makemigrations.config.yaml)
4. Default values

### 3. Type System (`/internal/types`)

Central type definitions for schema representation:

```go
type Schema struct {
    Database Database
    Include  []Include  // External schema imports
    Defaults Defaults  // Database-specific defaults
    Tables   []Table
}

type Field struct {
    Name       string
    Type       string
    PrimaryKey bool
    Nullable   *bool
    Default    string
    ForeignKey *ForeignKey
    // ... additional properties
}
```

### 4. Processing Pipeline

#### 4.1 Scanner (`/internal/scanner`)
- Discovers schema files in project and dependencies
- Supports multiple search patterns
- Filters based on configuration

#### 4.2 Parser (`/internal/parser`, `/internal/yaml/parser.go`)
- YAML schema validation and parsing
- SQL schema parsing (for SQL-based workflows)
- Syntax error detection and reporting

#### 4.3 Merger (`/internal/merger`, `/internal/yaml/merger.go`)
- Combines multiple schema sources
- Resolves conflicts between schemas
- Handles include dependencies
- Manages table and field merging

#### 4.4 Analyzer (`/internal/analyzer`, `/internal/yaml/analyzer.go`)
- Validates schema semantics
- Checks referential integrity
- Identifies circular dependencies
- Validates data types and constraints

#### 4.5 Diff Engine (`/internal/diff`, `/internal/yaml/diff.go`)
- Compares old and new schemas
- Detects changes:
  - Table additions/removals/renames
  - Field modifications
  - Index changes
  - Constraint updates
- Categorizes destructive vs safe operations

#### 4.6 Generator (`/internal/generator`, `/internal/yaml/migration_generator.go`)
- Converts diffs to SQL migrations
- Generates Goose-compatible format
- Adds review comments for destructive operations
- Creates both UP and DOWN migrations

### 5. Database Provider System (`/internal/providers`)

#### Provider Interface
```go
type Provider interface {
    // DDL Generation
    GenerateCreateTable(schema *Schema, table *Table) (string, error)
    GenerateDropTable(tableName string) string
    GenerateAddColumn(tableName string, field *Field) string
    GenerateAlterColumn(tableName string, oldField, newField *Field) (string, error)
    
    // Type Conversion
    ConvertFieldType(field *Field) string
    GetDefaultValue(defaultRef string, defaults map[string]string) (string, error)
    
    // Utilities
    QuoteName(name string) string
    SupportsOperation(operation string) bool
}
```

#### Provider Factory Pattern
```go
func NewProvider(dbType DatabaseType) (Provider, error) {
    switch dbType {
    case DatabasePostgreSQL:
        return postgresql.New(), nil
    case DatabaseMySQL:
        return mysql.New(), nil
    // ... other databases
    }
}
```

### 6. State Management (`/internal/state`)

- Maintains schema snapshots (.schema_snapshot.yaml)
- Tracks migration history
- Enables incremental migration generation
- Supports rollback tracking

### 7. YAML Processing (`/internal/yaml`)

#### Core Components:
- **include_processor.go**: Handles external schema includes
- **module_resolver.go**: Resolves Go module dependencies
- **sql_converter.go**: YAML to SQL transformation
- **migration_generator.go**: Migration file generation

#### SQL Converter Features:
- Database-specific SQL generation
- Safe type change detection
- Review comment injection
- Destructive operation handling

### 8. Specialized Features

#### 8.1 Struct2Schema (`/internal/struct2schema`)
Converts Go structs to YAML schemas:
- AST-based struct parsing
- Tag processing (db, gorm, sql, bun)
- Relationship detection
- Type mapping configuration

#### 8.2 Database Reverse Engineering
- Introspects existing databases
- Generates YAML schemas from database
- Supports all 12 database engines

## Design Patterns

### 1. Factory Pattern
Used extensively for database provider creation, ensuring proper abstraction between database-specific implementations.

### 2. Strategy Pattern
Database providers implement a common interface while providing database-specific SQL generation strategies.

### 3. Pipeline Pattern
Schema processing follows a clear pipeline: Scan → Parse → Merge → Analyze → Diff → Generate

### 4. Command Pattern
Each CLI command is encapsulated as a separate command object with its own flags and execution logic.

### 5. Builder Pattern
Migration and schema construction uses builder patterns for complex object assembly.

## Key Architectural Decisions

### 1. YAML-First Approach
**Decision**: Use YAML as the primary schema definition format.
**Rationale**: 
- Human-readable and version-control friendly
- Declarative approach reduces complexity
- Easy to merge and diff
- Supports modular composition via includes

### 2. Database Provider Abstraction
**Decision**: Implement a provider interface with database-specific implementations.
**Rationale**:
- Supports 12 different databases without code duplication
- Easy to add new database support
- Isolates database-specific logic
- Enables testing with mock providers

### 3. Goose Integration
**Decision**: Generate Goose-compatible migrations rather than custom format.
**Rationale**:
- Leverages mature, battle-tested migration runner
- Avoids reinventing migration execution
- Compatible with existing Go projects
- Supports both up and down migrations

### 4. Modular Schema Composition
**Decision**: Support schema includes from Go modules.
**Rationale**:
- Enables schema reuse across projects
- Supports microservice architectures
- Allows versioned schema dependencies
- Facilitates team collaboration

### 5. Interactive Destructive Operation Handling
**Decision**: Prompt for confirmation on destructive operations by default.
**Rationale**:
- Prevents accidental data loss
- Provides clear visibility of risks
- Allows override with --silent flag for CI/CD
- Generates review comments in SQL

### 6. Configuration Hierarchy
**Decision**: Support multiple configuration sources with clear precedence.
**Rationale**:
- Flexibility for different environments
- Easy override for CI/CD pipelines
- Maintains secure defaults
- Supports team-wide standards

## Data Flow

### Migration Generation Flow

```
1. User modifies schema.yaml
       │
2. Scanner discovers schema files
       │
3. Parser validates and loads YAML
       │
4. Include Processor resolves dependencies
       │
5. Merger combines multiple schemas
       │
6. Analyzer validates complete schema
       │
7. Diff Engine compares with snapshot
       │
8. Generator creates SQL migrations
       │
9. Writer saves migration files
       │
10. State Manager updates snapshot
```

### Struct2Schema Flow

```
1. Scanner finds Go source files
       │
2. AST Parser extracts structs
       │
3. Tag Processor interprets struct tags
       │
4. Type Mapper converts Go types to SQL
       │
5. Relationship Detector finds associations
       │
6. Schema Generator creates YAML
       │
7. Writer saves schema.yaml
```

## Error Handling Strategy

### Validation Layers
1. **Syntax Validation**: YAML/SQL parsing errors
2. **Schema Validation**: Type checking, constraint validation
3. **Semantic Validation**: Referential integrity, circular dependencies
4. **Runtime Validation**: Database connectivity, permissions

### Error Categories
- **Fatal Errors**: Stop execution immediately (invalid config, parse errors)
- **Validation Errors**: Collect and report all issues
- **Warnings**: Log but continue (deprecated features, best practice violations)
- **User Prompts**: Interactive decisions for destructive operations

## Performance Considerations

### Optimization Strategies
1. **Parallel Processing**: File scanning and parsing in parallel where possible
2. **Caching**: Module resolution and type information caching
3. **Lazy Loading**: Load schemas only when needed
4. **Minimal Memory**: Stream processing for large schemas

### Scalability
- Handles projects with 1000+ tables
- Supports deep module dependency trees
- Efficient diff algorithms for large schemas
- Optimized SQL generation

## Security Considerations

### Security Features
1. **No Direct SQL Execution**: Only generates SQL files
2. **Input Validation**: Strict YAML schema validation
3. **SQL Injection Prevention**: Proper identifier quoting
4. **Review Comments**: Destructive operations marked clearly
5. **No Credential Storage**: Database credentials via environment only

### Best Practices
- Never store credentials in config files
- Use read-only database access for reverse engineering
- Review generated migrations before execution
- Test migrations in staging environments

## Extension Points

### Adding New Database Support
1. Implement the Provider interface
2. Add provider to factory
3. Define type mappings
4. Add database-specific defaults
5. Implement tests

### Adding New Commands
1. Create command file in `/cmd`
2. Register with root command
3. Implement business logic in `/internal`
4. Add documentation in `/docs/commands`

### Custom Type Mappings
1. Extend type system in `/internal/types`
2. Update validation in analyzer
3. Add provider support
4. Document in schema format guide

## Testing Strategy

### Test Levels
1. **Unit Tests**: Component-level testing with mocks
2. **Integration Tests**: Pipeline testing with real schemas
3. **Provider Tests**: Database-specific SQL generation
4. **E2E Tests**: Full command execution tests

### Test Coverage Areas
- Schema parsing and validation
- Diff detection accuracy
- SQL generation correctness
- Configuration loading
- Error handling

## Future Architecture Considerations

### Potential Enhancements
1. **Plugin System**: Support for custom providers and processors
2. **Web UI**: Visual schema designer and migration preview
3. **Schema Versioning**: Built-in schema version management
4. **Distributed Schemas**: Support for federated schema management
5. **AI-Assisted Migration**: Intelligent migration suggestions

### Scalability Improvements
1. **Incremental Processing**: Process only changed modules
2. **Distributed Processing**: Support for large-scale projects
3. **Schema Caching**: Persistent schema cache
4. **Parallel Migration Generation**: Multi-threaded SQL generation

## Conclusion

The makemigrations architecture follows clean architecture principles with clear separation of concerns, extensive use of interfaces for abstraction, and a modular design that supports extensibility. The pipeline-based processing model ensures data flows predictably through the system, while the provider pattern enables support for multiple databases without compromising code quality or maintainability.