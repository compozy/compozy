## markdown

## status: pending

<task_context>
<domain>docs/content/docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 7.0: Complete Database Documentation

## Overview

Create comprehensive documentation for the multi-database feature, including decision guides, configuration references, CLI documentation, and troubleshooting guides. This task covers all database-related documentation updates to help users choose the right database and configure it correctly.

<critical>
- **ALWAYS READ** @tasks/prd-postgres/_docs.md for complete documentation plan
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md for technical details
- **CAN RUN IN PARALLEL** with implementation tasks (starting Week 3)
- **MANDATORY:** Include decision matrix for PostgreSQL vs SQLite
- **MANDATORY:** Emphasize vector DB requirement for SQLite
- **MANDATORY:** Document concurrency limitations clearly
- **MANDATORY:** Provide working code examples
</critical>

<requirements>
- Create 4 new database documentation pages
- Update 6+ existing documentation pages
- Update navigation structure in `source.config.ts`
- Include decision matrix and comparison tables
- Provide configuration examples for both drivers
- Document CLI flags (`--db-driver`, `--db-path`)
- Create troubleshooting guide for common issues
- Add diagrams/flowcharts where helpful
</requirements>

## Subtasks

- [ ] 7.1 Create `docs/content/docs/database/overview.mdx`
- [ ] 7.2 Create `docs/content/docs/database/postgresql.mdx`
- [ ] 7.3 Create `docs/content/docs/database/sqlite.mdx`
- [ ] 7.4 Create `docs/content/docs/troubleshooting/database.mdx`
- [ ] 7.5 Update `docs/content/docs/configuration/database.mdx`
- [ ] 7.6 Update `docs/content/docs/cli/start.mdx`
- [ ] 7.7 Update `docs/content/docs/cli/migrate.mdx`
- [ ] 7.8 Update `docs/content/docs/getting-started/installation.mdx`
- [ ] 7.9 Update `docs/content/docs/getting-started/quickstart.mdx`
- [ ] 7.10 Update `docs/content/docs/knowledge-bases/vector-databases.mdx`
- [ ] 7.11 Update `docs/content/docs/deployment/production.mdx`
- [ ] 7.12 Update navigation in `docs/source.config.ts`
- [ ] 7.13 Review and test all code examples

## Implementation Details

### 7.1 Database Overview (`docs/content/docs/database/overview.mdx`)

**Content Outline:**
- Introduction to multi-database support
- Decision matrix: When to use PostgreSQL vs SQLite
- Architecture overview (both drivers)
- Migration considerations
- Links to specific database docs

**Decision Matrix Table:**

```markdown
## Choosing Your Database

| Criterion | PostgreSQL ✅ | SQLite ✅ |
|-----------|--------------|----------|
| **Use Case** | Production, Multi-tenant | Development, Edge, Single-tenant |
| **Concurrency** | High (100+ workflows) | Low (5-10 workflows) |
| **Scalability** | Excellent (horizontal/vertical) | Limited (single file) |
| **Vector Search** | pgvector (built-in) | External DB required |
| **Deployment** | Separate database server | Embedded (single binary) |
| **Setup Complexity** | Moderate | Low (just a file path) |
| **Performance** | Optimized for high load | Optimized for reads |
| **Backup** | PostgreSQL tools (pg_dump) | File copy |
| **Recommended For** | ✅ Production deployments | ✅ Development, testing, edge |

### When to Use PostgreSQL

Choose PostgreSQL when you need:
- High concurrency (25+ concurrent workflows)
- Production-grade reliability and performance
- Built-in vector search with pgvector
- Horizontal scaling capabilities
- Advanced PostgreSQL features

### When to Use SQLite

Choose SQLite when you need:
- Quick local development setup
- Single-binary deployment (no external dependencies)
- Edge/IoT deployments with limited resources
- Testing and CI/CD pipelines
- Single-tenant, low-concurrency workloads
```

### 7.2 PostgreSQL Documentation (`docs/content/docs/database/postgresql.mdx`)

**Content Outline:**
- PostgreSQL features and benefits
- Configuration options (connection string, individual params, SSL/TLS)
- pgvector for knowledge bases
- Performance tuning
- Production deployment guide
- Troubleshooting

**Configuration Example:**

```yaml
# compozy.yaml (PostgreSQL)
database:
  driver: postgres  # default, can be omitted
  host: localhost
  port: 5432
  user: compozy
  password: ${DB_PASSWORD}  # from environment
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

### 7.3 SQLite Documentation (`docs/content/docs/database/sqlite.mdx`)

**Content Outline:**
- SQLite features and limitations
- Ideal use cases (development, testing, edge)
- Configuration options (file-based, in-memory, PRAGMA settings)
- **CRITICAL: Vector Database Requirement** (highlighted section)
- Performance characteristics
- Concurrency limitations (5-10 workflows recommended)
- Backup and export
- Troubleshooting

**CRITICAL Section:**

```markdown
## ⚠️ Vector Database Requirement

SQLite does **not have native vector database support**. If you plan to use knowledge bases or RAG features, you **must configure an external vector database**.

### Supported Vector Databases

When using SQLite, configure one of the following:

1. **Qdrant** (Recommended for production)
2. **Redis** with RediSearch
3. **Filesystem** (Development only)

### Example Configuration

```yaml
database:
  driver: sqlite
  path: ./data/compozy.db

knowledge:
  vector_dbs:
    - id: main
      provider: qdrant  # External vector DB required
      url: http://localhost:6333
      dimension: 1536
```

### Why Not pgvector?

The `pgvector` provider is **incompatible** with SQLite. If you attempt to configure SQLite with pgvector, Compozy will fail at startup with a clear error message.
```

**Concurrency Limitations:**

```markdown
## Concurrency Limitations

SQLite uses **database-level locking** (not row-level), which limits write concurrency:

- ✅ **Recommended:** 5-10 concurrent workflows
- ⚠️ **Not recommended:** 25+ concurrent workflows
- 🔴 **High-concurrency production:** Use PostgreSQL instead

SQLite is designed for:
- Single-tenant applications
- Development and testing
- Edge deployments with moderate load
```

### 7.4 Troubleshooting Guide (`docs/content/docs/troubleshooting/database.mdx`)

**Content Outline:**
- Common PostgreSQL issues (connection failures, migration errors, performance)
- Common SQLite issues (database locked, concurrency, file permissions, vector DB not configured)
- Diagnostic commands
- Error messages reference

**Example Sections:**

```markdown
## SQLite: Database Locked Error

**Error:** `database is locked`

**Cause:** Multiple processes/threads attempting concurrent writes

**Solution:**
1. Reduce concurrent workflow limit in configuration
2. Implement retry logic with exponential backoff
3. Consider PostgreSQL for high-concurrency workloads

```yaml
runtime:
  max_concurrent_workflows: 5  # Conservative for SQLite
```

---

## SQLite: Vector DB Not Configured

**Error:** `pgvector provider is incompatible with SQLite driver`

**Cause:** Attempted to use pgvector with SQLite

**Solution:** Configure an external vector database:

```yaml
database:
  driver: sqlite
  path: ./data/compozy.db

knowledge:
  vector_dbs:
    - id: main
      provider: qdrant  # Use external vector DB
      url: http://localhost:6333
```

See: [SQLite Vector Database Requirement](/docs/database/sqlite#vector-database-requirement)
```

### 7.5 Update Configuration Reference (`docs/content/docs/configuration/database.mdx`)

**Add Section:**

```markdown
## Database Driver Selection

### `database.driver`

Select the database backend.

- **Type:** `string`
- **Options:** `postgres` | `sqlite`
- **Default:** `postgres`
- **Environment Variable:** `DB_DRIVER`

### PostgreSQL Configuration

When `driver: postgres` (or omitted), configure PostgreSQL-specific fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host` | string | Yes* | PostgreSQL server host |
| `port` | string | No | PostgreSQL server port (default: 5432) |
| `user` | string | Yes | Database user |
| `password` | string | Yes | Database password |
| `dbname` | string | Yes | Database name |
| `sslmode` | string | No | SSL mode (disable, require, verify-ca, verify-full) |

*Or provide `conn_string` instead

### SQLite Configuration

When `driver: sqlite`, configure SQLite-specific fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Database file path or `:memory:` |

**Example:**

```yaml
database:
  driver: sqlite
  path: ./data/compozy.db  # File-based
  # OR
  path: ":memory:"         # In-memory (ephemeral)
```
```

### 7.6 Update CLI Start Documentation (`docs/content/docs/cli/start.mdx`)

**Add Section:**

```markdown
## Database Options

### `--db-driver`

Select database driver.

- **Values:** `postgres` | `sqlite`
- **Default:** `postgres`

**Examples:**

```bash
# PostgreSQL (default)
compozy start

# SQLite (file-based)
compozy start --db-driver=sqlite --db-path=./compozy.db

# SQLite (in-memory)
compozy start --db-driver=sqlite --db-path=:memory:
```

### `--db-path`

SQLite database file path (required when `--db-driver=sqlite`).

**Examples:**

```bash
# File-based database
compozy start --db-driver=sqlite --db-path=./data/compozy.db

# In-memory database (data lost on restart)
compozy start --db-driver=sqlite --db-path=:memory:
```
```

### 7.7 Update CLI Migrate Documentation (`docs/content/docs/cli/migrate.mdx`)

**Add Section:**

```markdown
## Multi-Database Support

Migrations work with both PostgreSQL and SQLite. The driver is automatically detected from your configuration.

```bash
# Apply migrations (auto-detects driver from compozy.yaml)
compozy migrate up

# Check migration status
compozy migrate status

# Rollback migrations
compozy migrate down
```

**Note:** PostgreSQL and SQLite use separate migration files optimized for each database's SQL dialect.
```

### 7.8-7.11 Update Cross-Page Documentation

**Updates:**
- `getting-started/installation.mdx`: Add "Quick Start with SQLite" section
- `getting-started/quickstart.mdx`: Add "5-Minute Setup (SQLite)" path
- `knowledge-bases/vector-databases.mdx`: Add note about SQLite requirement
- `deployment/production.mdx`: Emphasize PostgreSQL recommendation

### 7.12 Update Navigation (`docs/source.config.ts`)

**Add Database Section:**

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
  title: "Troubleshooting",
  pages: [
    "troubleshooting/overview",
    "troubleshooting/database",  // NEW
    "troubleshooting/workflows",
  ]
}
```

### Relevant Files

**New Files:**
- `docs/content/docs/database/overview.mdx`
- `docs/content/docs/database/postgresql.mdx`
- `docs/content/docs/database/sqlite.mdx`
- `docs/content/docs/troubleshooting/database.mdx`

**Modified Files:**
- `docs/content/docs/configuration/database.mdx`
- `docs/content/docs/cli/start.mdx`
- `docs/content/docs/cli/migrate.mdx`
- `docs/content/docs/getting-started/installation.mdx`
- `docs/content/docs/getting-started/quickstart.mdx`
- `docs/content/docs/knowledge-bases/vector-databases.mdx`
- `docs/content/docs/deployment/production.mdx`
- `docs/source.config.ts`

### Dependent Files

- None (documentation can be written in parallel with implementation)

## Deliverables

- [ ] 4 new database documentation pages created
- [ ] 7+ existing pages updated with database references
- [ ] Navigation structure updated in `source.config.ts`
- [ ] Decision matrix included in overview
- [ ] Vector DB requirement clearly documented for SQLite
- [ ] Concurrency limitations documented
- [ ] All configuration examples tested and working
- [ ] CLI flags documented
- [ ] Troubleshooting guide complete
- [ ] All internal links working
- [ ] Documentation builds without errors: `npm run dev`

## Tests

### Documentation Quality Checks

- [ ] All code examples are valid and tested
- [ ] All internal links resolve correctly (no 404s)
- [ ] Search functionality finds new database pages
- [ ] Mobile view renders correctly
- [ ] Dark mode styling consistent
- [ ] Syntax highlighting works for code blocks

### Manual Testing Checklist

- [ ] Follow PostgreSQL setup guide → successful workflow execution
- [ ] Follow SQLite setup guide → successful workflow execution
- [ ] Try invalid config (SQLite + pgvector) → clear error message matches docs
- [ ] Copy/paste config examples → they work as-is
- [ ] Click all internal database links → no broken links

### Automated Checks

- [ ] Link checker passes: `npm run check-links`
- [ ] Build completes without warnings: `npm run build`
- [ ] No broken schema references
- [ ] No typos in code examples

## Success Criteria

- [ ] All new documentation pages created and complete
- [ ] All updated pages reflect multi-database support
- [ ] Decision matrix helps users choose database
- [ ] Vector DB requirement for SQLite clearly documented and emphasized
- [ ] Configuration examples work when copy-pasted
- [ ] CLI documentation includes new flags
- [ ] Troubleshooting guide covers common issues
- [ ] Navigation structure logical and easy to follow
- [ ] All internal links work
- [ ] Documentation builds successfully: `npm run build`
- [ ] Search indexes new pages: search for "sqlite" returns relevant results
- [ ] Mobile and dark mode rendering correct
- [ ] Code examples follow project standards
