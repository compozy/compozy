# Documentation Plan: SQLite Database Backend Support

## Goals

- Provide comprehensive documentation for database selection and configuration
- Create decision-making guides for when to use PostgreSQL vs SQLite
- Document vector database requirements for SQLite mode
- Update existing pages to reflect multi-database support
- Ensure smooth user experience for both database options

## New/Updated Pages

### 1. `docs/content/docs/database/overview.mdx` (NEW)

- **Purpose:** High-level database overview and decision guide
- **Outline:**
  - Introduction: Multi-database support in Compozy
  - Database Options Comparison Table
  - Decision Matrix: When to use PostgreSQL vs SQLite
  - Architecture Overview (both drivers)
  - Migration Considerations
- **Links:** 
  - → `database/postgresql.mdx`
  - → `database/sqlite.mdx`
  - → `configuration/database.mdx`
  - → `getting-started/installation.mdx`

### 2. `docs/content/docs/database/postgresql.mdx` (NEW)

- **Purpose:** PostgreSQL-specific configuration and best practices
- **Outline:**
  - PostgreSQL Features and Benefits
  - Configuration Options
    - Connection string format
    - Individual parameters (host, port, user, etc.)
    - SSL/TLS configuration
    - Connection pooling settings
  - pgvector for Knowledge Bases
  - Performance Tuning
  - Production Deployment Guide
  - Troubleshooting Common Issues
- **Links:**
  - → `database/overview.mdx`
  - → `knowledge-bases/vector-databases.mdx`
  - → `examples/` (PostgreSQL examples)

### 3. `docs/content/docs/database/sqlite.mdx` (NEW)

- **Purpose:** SQLite-specific configuration and use cases
- **Outline:**
  - SQLite Features and Limitations
  - Ideal Use Cases
    - Development and testing
    - Edge deployments
    - Single-tenant scenarios
  - Configuration Options
    - File-based database
    - In-memory mode (`:memory:`)
    - PRAGMA settings
  - **CRITICAL: Vector Database Requirement**
    - Why external vector DB is required
    - Supported options (Qdrant, Redis, Filesystem)
    - Configuration examples
  - Performance Characteristics
  - Concurrency Limitations (5-10 workflows recommended)
  - Backup and Export
  - Troubleshooting
- **Links:**
  - → `database/overview.mdx`
  - → `knowledge-bases/vector-databases.mdx`
  - → `examples/` (SQLite examples)

### 4. `docs/content/docs/configuration/database.mdx` (UPDATE)

- **Purpose:** Database configuration reference
- **Outline:**
  - Add `driver` field documentation
  - PostgreSQL configuration section
  - SQLite configuration section
  - Vector database validation rules
  - Environment variables
  - CLI flags
  - Configuration precedence
- **Links:**
  - → `database/overview.mdx`
  - → Schema reference for `DatabaseConfig`

### 5. `docs/content/docs/knowledge-bases/vector-databases.mdx` (UPDATE)

- **Purpose:** Update to mention SQLite requirements
- **Updates:**
  - Add note: "When using SQLite, external vector database is mandatory"
  - Update pgvector section: "Available with PostgreSQL only"
  - Add decision matrix for vector DB selection
  - Configuration examples for each provider
- **Links:**
  - → `database/sqlite.mdx`
  - → `database/postgresql.mdx`

### 6. `docs/content/docs/getting-started/installation.mdx` (UPDATE)

- **Purpose:** Update installation guide to cover both databases
- **Updates:**
  - Add "Quick Start with SQLite" section
  - Update "Production Setup" to emphasize PostgreSQL
  - Add database selection step
  - Update CLI examples with `--db-driver` flag
- **Links:**
  - → `database/overview.mdx`
  - → `database/sqlite.mdx`
  - → `database/postgresql.mdx`

### 7. `docs/content/docs/getting-started/quickstart.mdx` (UPDATE)

- **Purpose:** Add SQLite quick start path
- **Updates:**
  - Add "5-Minute Setup (SQLite)" section
  - Show simplified configuration for local development
  - Mention when to switch to PostgreSQL
- **Links:**
  - → `database/overview.mdx`
  - → Tutorial for first workflow

### 8. `docs/content/docs/deployment/production.mdx` (UPDATE)

- **Purpose:** Emphasize PostgreSQL for production
- **Updates:**
  - Add "Database Selection" section
  - Strong recommendation for PostgreSQL in production
  - Performance comparison table
  - Scaling considerations
- **Links:**
  - → `database/postgresql.mdx`
  - → `deployment/scaling.mdx`

### 9. `docs/content/docs/troubleshooting/database.mdx` (NEW)

- **Purpose:** Database-specific troubleshooting guide
- **Outline:**
  - Common PostgreSQL Issues
    - Connection failures
    - Migration errors
    - Performance problems
  - Common SQLite Issues
    - Database locked errors
    - Concurrency problems
    - File permission issues
    - Vector DB not configured
  - Diagnostic Commands
  - Error Messages Reference
- **Links:**
  - → `database/postgresql.mdx`
  - → `database/sqlite.mdx`

## Schema Docs

### 1. `docs/content/docs/reference/configuration/database.mdx` (UPDATE)

- **Renders:** `schemas/config-database.json` (updated with `driver` field)
- **Notes:** 
  - Highlight `driver` field as key selection mechanism
  - Document PostgreSQL-specific vs SQLite-specific fields
  - Show validation rules (e.g., SQLite + pgvector not allowed)

## API Docs

No API changes - database selection is configuration-driven. No new API endpoints.

## CLI Docs

### 1. `docs/content/docs/cli/start.mdx` (UPDATE)

- **Updates:**
  - Add `--db-driver` flag documentation
  - Add `--db-path` flag for SQLite
  - Update examples to show both PostgreSQL and SQLite usage
- **Example:**
  ```bash
  # PostgreSQL (default)
  compozy start
  
  # SQLite (file-based)
  compozy start --db-driver=sqlite --db-path=./compozy.db
  
  # SQLite (in-memory)
  compozy start --db-driver=sqlite --db-path=:memory:
  ```

### 2. `docs/content/docs/cli/migrate.mdx` (UPDATE)

- **Updates:**
  - Document migration behavior for both drivers
  - Show how to run migrations manually
  - Explain dual migration files
- **Commands:**
  ```bash
  # Apply migrations (auto-detects driver from config)
  compozy migrate up
  
  # Check migration status
  compozy migrate status
  
  # Rollback migrations
  compozy migrate down
  ```

### 3. `docs/content/docs/cli/config.mdx` (UPDATE)

- **Updates:**
  - Add `database` section examples
  - Show both PostgreSQL and SQLite configurations
  - Document environment variable overrides

## Cross-page Updates

### `docs/content/docs/index.mdx` (Homepage)

- **Update:** Add mention of multi-database support in feature list
- **Note:** "Flexible database options: PostgreSQL (production) or SQLite (development/edge)"

### `docs/content/docs/concepts/architecture.mdx`

- **Update:** Add section on "Storage Layer" explaining database abstraction
- **Diagram:** Show repository pattern with both drivers

### `docs/content/docs/examples/index.mdx`

- **Update:** Group examples by database type or show dual configs
- **Note:** Indicate which examples work with SQLite vs PostgreSQL

## Navigation & Indexing

### Update `docs/source.config.ts`

**New Section Structure:**

```typescript
{
  title: "Database",
  pages: [
    "database/overview",      // NEW
    "database/postgresql",    // NEW
    "database/sqlite",        // NEW
  ]
},
{
  title: "Configuration",
  pages: [
    "configuration/overview",
    "configuration/database",  // UPDATE
    "configuration/workflows",
    // ... existing pages
  ]
},
{
  title: "Troubleshooting",
  pages: [
    "troubleshooting/overview",
    "troubleshooting/database",  // NEW
    "troubleshooting/workflows",
    // ... existing pages
  ]
}
```

**Sidebar Order:**
1. Getting Started
2. Concepts
3. **Database** (new section)
4. Configuration
5. Workflows
6. Agents
7. Tools
8. Knowledge Bases
9. Examples
10. CLI Reference
11. Troubleshooting

## Decision Matrix Template

**Include in `database/overview.mdx`:**

| Criterion | PostgreSQL | SQLite |
|-----------|-----------|--------|
| **Use Case** | Production, Multi-tenant | Development, Edge, Single-tenant |
| **Concurrency** | High (100+ workflows) | Low (5-10 workflows) |
| **Scalability** | Excellent (horizontal/vertical) | Limited (single file) |
| **Vector Search** | pgvector (built-in) | External DB required |
| **Deployment** | Separate database server | Embedded (single binary) |
| **Setup Complexity** | Moderate (requires PostgreSQL) | Low (just a file path) |
| **Performance** | Optimized for high load | Optimized for reads |
| **Backup** | PostgreSQL tools (pg_dump) | File copy |
| **Recommended For** | ✅ Production deployments | ✅ Development, testing, edge |

## Code Snippets

### PostgreSQL Configuration Example

```yaml
# compozy.yaml
database:
  driver: postgres  # default, can be omitted
  host: localhost
  port: 5432
  user: compozy
  password: ${DB_PASSWORD}  # from env
  dbname: compozy
  sslmode: require
  max_open_conns: 25
  max_idle_conns: 5

knowledge:
  vector_dbs:
    - id: main
      provider: pgvector  # Uses PostgreSQL
      dimension: 1536
```

### SQLite Configuration Example

```yaml
# compozy.yaml
database:
  driver: sqlite
  path: ./data/compozy.db  # or :memory: for in-memory

knowledge:
  vector_dbs:
    - id: main
      provider: qdrant  # External vector DB required
      url: http://localhost:6333
      dimension: 1536
```

## Visual Assets

### Diagrams to Create

1. **Database Architecture Diagram** (`database/overview.mdx`)
   - Show dual driver architecture
   - Repository pattern with factory selection
   - Domain layer independence

2. **Decision Flowchart** (`database/overview.mdx`)
   - "Which database should I use?"
   - Decision tree based on use case, scale, deployment type

3. **Vector DB Options** (`database/sqlite.mdx`)
   - Show SQLite → External Vector DB connection
   - Compare Qdrant, Redis, Filesystem options

## Acceptance Criteria

- [ ] All new pages created with complete outlines
- [ ] All update pages modified with SQLite references
- [ ] Schema documentation reflects new `driver` field
- [ ] CLI reference includes database flags
- [ ] Decision matrix helps users choose database
- [ ] Vector DB requirement for SQLite clearly documented
- [ ] Navigation structure updated in `source.config.ts`
- [ ] Code examples valid and tested
- [ ] Internal links between pages work
- [ ] Docs dev server builds without errors (`npm run dev`)
- [ ] Search index includes new database terms
- [ ] Mobile view renders correctly
- [ ] Dark mode styling consistent

## Testing Documentation

### Manual Testing Checklist

- [ ] Follow PostgreSQL setup guide → successful workflow execution
- [ ] Follow SQLite setup guide → successful workflow execution
- [ ] Try invalid config (SQLite + pgvector) → clear error message
- [ ] Copy/paste code examples → they work as-is
- [ ] Click all internal links → no 404s
- [ ] Search for "database" → relevant results
- [ ] Search for "sqlite" → new pages appear

### Automated Checks

- [ ] Link checker passes (no broken internal links)
- [ ] Code block syntax highlighting works
- [ ] Schema references resolve correctly
- [ ] Build process completes without warnings

## Documentation Timeline

| Week | Deliverable |
|------|-------------|
| 1-2 | Database overview, PostgreSQL/SQLite pages (draft) |
| 3 | Configuration reference, CLI updates |
| 4 | Troubleshooting guide, cross-page updates |
| 5 | Review, editing, diagrams |
| 6 | Final review, publication |

## Notes

- **Priority:** High - Documentation crucial for adoption
- **Audience:** Mixed (beginners need simplicity, experts need depth)
- **Tone:** Practical, prescriptive (guide users to right choice)
- **Maintenance:** Update when adding new features to either driver
- **Feedback:** Collect user feedback on clarity of database selection guide

---

**Plan Version:** 1.0  
**Date:** 2025-01-27  
**Status:** Ready for Implementation
