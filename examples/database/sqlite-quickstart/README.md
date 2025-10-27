# SQLite Quickstart Example

Minimal example demonstrating Compozy with a SQLite database backend and filesystem vector DB.

## Features

- ✅ **No external database required** — SQLite embedded storage
- ✅ **Single file database** — All data in `./data/compozy.db`
- ✅ **Filesystem vector DB** — No Qdrant or Redis dependency
- ✅ **Simple setup** — Configure an API key and run

## Prerequisites

- Compozy CLI installed
- LLM API key (Groq, OpenAI, or another supported provider)

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

The SQLite database is created automatically at `./data/compozy.db`.

### 3. Run the Workflow

In another terminal:

```bash
compozy workflow run text-analysis --input='{"text": "Compozy makes AI workflows easy!"}'
```

Expected output (truncated):

```json
{
  "data": {
    "status": "completed",
    "output": {
      "analysis": {
        "summary": "...",
        "themes": ["..."],
        "sentiment": "positive"
      }
    }
  }
}
```

### 4. Verify Database

```bash
# Check database file exists
ls -lh ./data/compozy.db

# View workflow history
compozy workflow list
```

## Configuration Details

### SQLite Database

```yaml
database:
  driver: sqlite
  path: ./data/compozy.db
```

- File-based persistence: data survives restarts
- Location: `./data/compozy.db` (created automatically)
- Zero external dependencies beyond the LLM provider

### Filesystem Vector DB

```yaml
knowledge:
  vector_dbs:
    - id: main
      provider: filesystem
      path: ./data/vectors
      dimension: 1536
```

- Stores embeddings on disk with no external services
- Ideal for local development and CI/CD pipelines
- For production, switch to Qdrant or Redis for scalability

## When to Use This Setup

✅ Local development, demos, edge deployments, single-tenant apps

⚠️ Use PostgreSQL for high concurrency, multi-tenant workloads, or advanced vector search (pgvector).

## Troubleshooting

- **Database file missing:** Ensure the `./data/` directory is writable. You can pre-create it with `mkdir -p ./data`.
- **Vector DB errors:** Filesystem provider is best for development. Configure Qdrant or Redis for production.
- **Permission issues:** Confirm Bun runtime permissions in `compozy.yaml` include `--allow-read` and `--allow-net`.

## Learn More

- [Database Overview](../../../docs/content/docs/database/overview.mdx)
- [SQLite Configuration Guide](../../../docs/content/docs/database/sqlite.mdx)
- [Vector Databases](../../../docs/content/docs/knowledge-bases/vector-databases.mdx)
