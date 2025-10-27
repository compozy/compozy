## markdown

## status: completed

<task_context>
<domain>examples/database</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 8.0: SQLite Quickstart Example

## Overview

Create a simple, working example demonstrating SQLite backend usage with a basic workflow. This example shows the easiest path to get started with Compozy using SQLite, requiring minimal setup and no external database dependencies.

<critical>
- **ALWAYS READ** @tasks/prd-postgres/_examples.md for example structure
- **ALWAYS READ** @examples/prompt-only/ for reference pattern
- **DEPENDENCY:** Can start after Task 1.0 (Foundation) is complete
- **MANDATORY:** Use filesystem vector DB (no external dependencies)
- **MANDATORY:** Simple workflow (like prompt-only example)
- **MANDATORY:** Clear README with step-by-step instructions
- **MANDATORY:** Working configuration that can be copy-pasted
</critical>

<requirements>
- Create `examples/database/sqlite-quickstart/` directory
- SQLite configuration with file-based database
- Filesystem vector DB (no external services)
- Simple text analysis workflow
- README with setup instructions
- `.env.example` for API keys
- Runnable with `compozy start`
</requirements>

## Subtasks

- [x] 8.1 Create example directory structure
- [x] 8.2 Create `compozy.yaml` with SQLite configuration
- [x] 8.3 Create simple workflow (`workflow.yaml`)
- [x] 8.4 Write comprehensive README
- [x] 8.5 Create `.env.example` file
- [x] 8.6 Test example end-to-end
- [x] 8.7 Add example to main examples index

## Implementation Details

### 8.1 Directory Structure

```
examples/database/sqlite-quickstart/
├── compozy.yaml         # SQLite configuration
├── workflow.yaml        # Simple text analysis workflow
├── README.md            # Setup and run instructions
├── .env.example         # API key placeholder
└── data/                # Created automatically for SQLite database
```

### 8.2 SQLite Configuration (`compozy.yaml`)

**Reference:** `examples/prompt-only/compozy.yaml`

```yaml
name: sqlite-quickstart
version: 0.1.0
description: Minimal SQLite database backend example

workflows:
  - source: ./workflow.yaml

models:
  - provider: groq
    model: llama-3.3-70b-versatile
    api_key: "{{ .env.GROQ_API_KEY }}"
    default: true

# SQLite database configuration
database:
  driver: sqlite
  path: ./data/compozy.db  # File-based database

# Filesystem vector DB (no external dependencies)
knowledge:
  vector_dbs:
    - id: main
      provider: filesystem
      path: ./data/vectors
      dimension: 1536

runtime:
  type: bun
  permissions:
    - --allow-read
    - --allow-net
```

### 8.3 Simple Workflow (`workflow.yaml`)

**Reference:** `examples/prompt-only/workflow.yaml`

```yaml
id: text-analysis
version: 0.1.0
description: Simple text analysis workflow using SQLite backend

config:
  input:
    type: object
    properties:
      text:
        type: string
        description: The text content to analyze

tasks:
  - id: analyze
    type: basic
    prompt: |-
      You are a concise text analysis assistant.
      
      Analyze the following text and provide:
      1. A brief summary (1-2 sentences)
      2. Key themes or topics
      3. Overall sentiment (positive/negative/neutral)
      
      Text to analyze:
      ---
      {{ .workflow.input.text }}
      ---
      
      Provide your analysis in a clear, structured format.
```

### 8.4 README (`README.md`)

```markdown
# SQLite Quickstart Example

Minimal example demonstrating Compozy with SQLite database backend.

## Features

- ✅ **No external database required** - SQLite embedded
- ✅ **Single file database** - All data in `./data/compozy.db`
- ✅ **Filesystem vector DB** - No Qdrant/Redis needed
- ✅ **Simple setup** - Just configure API key and run

## Prerequisites

- Compozy CLI installed
- LLM API key (Groq, OpenAI, or other provider)

## Quick Start

### 1. Configure API Key

```bash
cp .env.example .env
# Edit .env and add your API key
```

### 2. Start Compozy

```bash
compozy start
```

The SQLite database will be created automatically at `./data/compozy.db`.

**You should see:**
```
Database initialized: driver=sqlite path=./data/compozy.db mode=file-based
Server listening on :5001
```

### 3. Run the Workflow

In another terminal:

```bash
compozy workflow run text-analysis --input='{"text": "Compozy makes AI workflows easy!"}'
```

**Expected output:**
```json
{
  "data": {
    "exec_id": "...",
    "status": "completed",
    "output": {
      "analysis": "Summary: ...",
      "themes": ["..."],
      "sentiment": "positive"
    }
  }
}
```

### 4. Verify Database

```bash
# Check database file created
ls -lh ./data/compozy.db

# View workflow history
compozy workflow list
```

## Configuration Details

### SQLite Database

```yaml
database:
  driver: sqlite
  path: ./data/compozy.db  # File-based storage
```

- **File-based:** Data persists across restarts
- **Location:** `./data/compozy.db` (created automatically)
- **No external dependencies:** Single binary + database file

### Filesystem Vector DB

```yaml
knowledge:
  vector_dbs:
    - id: main
      provider: filesystem
      path: ./data/vectors
```

- **No external services:** Vectors stored as files
- **Development-friendly:** No Qdrant/Redis setup needed
- **Note:** For production, use Qdrant or Redis

## When to Use This Setup

✅ **Good for:**
- Local development
- Quick evaluation/testing
- CI/CD pipelines
- Edge deployments
- Single-tenant applications

⚠️ **Not recommended for:**
- High-concurrency production (use PostgreSQL)
- Multi-tenant applications (use PostgreSQL)
- Workloads with 25+ concurrent workflows

## Comparison: SQLite vs PostgreSQL

| Feature | This Example (SQLite) | PostgreSQL |
|---------|----------------------|------------|
| Setup Time | < 1 minute | ~5 minutes |
| External Dependencies | None | PostgreSQL server |
| Concurrency | Low (5-10 workflows) | High (100+ workflows) |
| Vector Search | Filesystem (basic) | pgvector (advanced) |
| Production Use | Edge/Single-tenant | Multi-tenant/Scale |

## Next Steps

- **Production deployment:** See [PostgreSQL setup guide](../../docs/database/postgresql.md)
- **Vector search:** Configure [Qdrant](../../docs/database/sqlite.md#using-qdrant)
- **More examples:** Browse [examples directory](../)

## Troubleshooting

### Database file not created

**Check:** Is the `./data/` directory writable?

```bash
mkdir -p ./data
compozy start
```

### Vector DB errors

**Note:** Filesystem vector DB is for development only. For production, configure Qdrant or Redis.

### Permission errors

**Check:** Bun runtime permissions in `compozy.yaml`:

```yaml
runtime:
  type: bun
  permissions:
    - --allow-read
    - --allow-net
```

## Learn More

- [Database Overview](../../docs/database/overview.md)
- [SQLite Configuration](../../docs/database/sqlite.md)
- [Vector Databases](../../docs/knowledge-bases/vector-databases.md)
```

### 8.5 Environment Variables (`.env.example`)

```bash
# LLM Provider API Key
GROQ_API_KEY=your_api_key_here

# Or use OpenAI
# OPENAI_API_KEY=your_api_key_here
```

### 8.6 Testing the Example

**Manual Test Steps:**

```bash
# 1. Navigate to example
cd examples/database/sqlite-quickstart

# 2. Setup environment
cp .env.example .env
# Edit .env with your API key

# 3. Start Compozy
compozy start

# Expected: Database initialized message

# 4. In another terminal, run workflow
compozy workflow run text-analysis --input='{"text": "Test content"}'

# Expected: Workflow completes successfully

# 5. Verify database created
ls -lh ./data/compozy.db

# Expected: File exists (size > 0)

# 6. List workflows
compozy workflow list

# Expected: See executed workflow

# 7. Stop server
# Ctrl+C in terminal running compozy start

# 8. Restart and verify persistence
compozy start
compozy workflow list

# Expected: Previous workflow still listed
```

### 8.7 Update Examples Index

**Add to:** `examples/README.md` (or main examples index)

```markdown
### Database Examples

#### SQLite Quickstart

**Location:** `database/sqlite-quickstart/`

Minimal example demonstrating SQLite backend with filesystem vector DB. Perfect for local development and testing.

**Features:**
- No external database dependencies
- Single-file database
- Quick setup (< 1 minute)

**Use Case:** Development, testing, edge deployments

[View Example →](./database/sqlite-quickstart/)
```

### Relevant Files

**New Files:**
- `examples/database/sqlite-quickstart/compozy.yaml`
- `examples/database/sqlite-quickstart/workflow.yaml`
- `examples/database/sqlite-quickstart/README.md`
- `examples/database/sqlite-quickstart/.env.example`

**Modified Files:**
- `examples/README.md` (add reference to new example)

**Reference Files:**
- `examples/prompt-only/` - Similar structure and patterns

### Dependent Files

- `engine/infra/sqlite/` - SQLite implementation (from Task 1.0)
- `engine/infra/repo/provider.go` - Factory pattern (from Task 4.0)

## Deliverables

- [x] `examples/database/sqlite-quickstart/` directory created
- [x] `compozy.yaml` with SQLite configuration
- [x] `workflow.yaml` with simple text analysis task
- [x] `README.md` with comprehensive setup instructions
- [x] `.env.example` with API key placeholder
- [x] Example runs successfully end-to-end
- [x] Database file created at `./data/compozy.db`
- [x] Workflow executes and completes
- [x] Added to examples index

## Tests

### Manual Testing Checklist

- [x] Navigate to example directory
- [x] Create `.env` from `.env.example`
- [x] Add valid API key to `.env`
- [x] Run `compozy start` - server starts successfully
- [x] Database file created at `./data/compozy.db`
- [x] Run workflow - completes successfully
- [x] Run `compozy workflow list` - shows executed workflow
- [x] Stop server (Ctrl+C)
- [x] Restart server - data persists
- [x] README instructions accurate and complete
- [x] All code examples work when copy-pasted
- [x] No errors in logs

### Edge Cases

- [x] Run without `.env` file - clear error message
- [x] Run with invalid API key - clear error message
- [x] Run with missing `./data/` directory - creates automatically
- [x] Run twice - database not recreated, data preserved

## Success Criteria

- [x] Example runs successfully from scratch
- [x] Database file created automatically
- [x] Workflow executes and persists to database
- [x] Data survives server restart
- [x] README instructions complete and accurate
- [x] All configuration examples work as-is
- [x] No external dependencies required (except LLM API)
- [x] Example demonstrates key SQLite features (file-based DB, persistence)
- [x] Clear documentation on when to use SQLite vs PostgreSQL
- [x] Example added to main examples index
