# Examples Plan: SQLite Database Backend Support

## Conventions

- Folder prefix: `examples/database/*`
- Use environment variable interpolation for sensitive data
- Provide clear README with prerequisites and expected outputs
- Show both PostgreSQL and SQLite configurations where applicable

## Example Matrix

### 1. `examples/database/sqlite-quickstart`

- **Purpose:** Demonstrate fastest way to get started with SQLite
- **Files:**
  - `compozy.yaml` – Minimal SQLite configuration
  - `workflows/hello-world.yaml` – Simple workflow
  - `README.md` – Setup instructions and walkthrough
  - `.env.example` – No external dependencies
- **Demonstrates:**
  - SQLite file-based database
  - Single-binary deployment
  - No external services required (except LLM API)
  - Filesystem vector DB for knowledge features
- **Walkthrough:**
  ```bash
  cd examples/database/sqlite-quickstart
  cp .env.example .env
  # Edit .env to add your OpenAI API key
  compozy start
  # Database automatically created at ./data/compozy.db
  compozy workflow run hello-world
  ```

### 2. `examples/database/sqlite-memory`

- **Purpose:** Show in-memory SQLite for testing/ephemeral workloads
- **Files:**
  - `compozy.yaml` – In-memory SQLite (`:memory:`)
  - `workflows/test-workflow.yaml` – Test workflow
  - `README.md` – Use case explanation
- **Demonstrates:**
  - In-memory database (no persistence)
  - Fastest startup time
  - Perfect for CI/CD tests
  - No vector DB required (no knowledge features)
- **Walkthrough:**
  ```bash
  cd examples/database/sqlite-memory
  compozy start
  # Database exists only in memory, lost on restart
  compozy workflow run test-workflow
  ```

### 3. `examples/database/sqlite-qdrant`

- **Purpose:** SQLite with Qdrant for knowledge bases
- **Files:**
  - `compozy.yaml` – SQLite + Qdrant configuration
  - `workflows/rag-workflow.yaml` – RAG workflow
  - `knowledge/documents/*.md` – Sample documents
  - `docker-compose.yml` – Qdrant service
  - `README.md` – Setup guide
  - `.env.example` – Qdrant URL
- **Demonstrates:**
  - SQLite for relational data
  - Qdrant for vector embeddings
  - Knowledge base ingestion and querying
  - Hybrid deployment (embedded SQLite + external Qdrant)
- **Walkthrough:**
  ```bash
  cd examples/database/sqlite-qdrant
  docker-compose up -d  # Start Qdrant
  cp .env.example .env
  compozy knowledge ingest ./knowledge
  compozy start
  compozy workflow run rag-workflow --input='{"query": "What is AI?"}'
  ```

### 4. `examples/database/postgres-comparison`

- **Purpose:** Side-by-side PostgreSQL vs SQLite comparison
- **Files:**
  - `compozy-postgres.yaml` – PostgreSQL config
  - `compozy-sqlite.yaml` – SQLite config
  - `workflows/benchmark.yaml` – Performance test workflow
  - `docker-compose.yml` – PostgreSQL service
  - `README.md` – Comparison guide
  - `benchmark.sh` – Script to run both and compare
- **Demonstrates:**
  - Same workflow, different databases
  - Performance characteristics
  - When to choose which database
- **Walkthrough:**
  ```bash
  cd examples/database/postgres-comparison
  docker-compose up -d  # Start PostgreSQL
  
  # Run with PostgreSQL
  ./benchmark.sh --config=compozy-postgres.yaml
  
  # Run with SQLite
  ./benchmark.sh --config=compozy-sqlite.yaml
  
  # Compare results
  ```

### 5. `examples/database/migration-export-import`

- **Purpose:** Show data export/import between databases
- **Files:**
  - `compozy-source.yaml` – Source database config
  - `compozy-target.yaml` – Target database config
  - `workflows/sample-data.yaml` – Generate test data
  - `scripts/export.sh` – Export script
  - `scripts/import.sh` – Import script
  - `README.md` – Migration guide
- **Demonstrates:**
  - Exporting workflow/task state to JSON
  - Importing state to different database
  - Use case: PostgreSQL → SQLite or vice versa
- **Walkthrough:**
  ```bash
  cd examples/database/migration-export-import
  
  # Run workflows on source DB
  compozy --config=compozy-source.yaml workflow run sample-data
  
  # Export data
  ./scripts/export.sh > backup.json
  
  # Import to target DB
  ./scripts/import.sh backup.json
  
  # Verify
  compozy --config=compozy-target.yaml workflow list
  ```

### 6. `examples/database/edge-deployment`

- **Purpose:** Demonstrate edge/IoT deployment with SQLite
- **Files:**
  - `compozy.yaml` – Minimal SQLite config
  - `workflows/sensor-processing.yaml` – Edge workflow
  - `Dockerfile` – Single-binary container
  - `README.md` – Edge deployment guide
- **Demonstrates:**
  - Minimal dependencies (no PostgreSQL)
  - Single binary + SQLite file
  - Docker container <100MB
  - Suitable for ARM devices
- **Walkthrough:**
  ```bash
  cd examples/database/edge-deployment
  
  # Build minimal image
  docker build -t compozy-edge .
  
  # Run on edge device
  docker run -v $(pwd)/data:/data compozy-edge
  ```

## Minimal YAML Shapes

### SQLite Quickstart Config

```yaml
# compozy.yaml (minimal SQLite)
version: "1.0"

database:
  driver: sqlite
  path: ./data/compozy.db

llm:
  providers:
    - id: openai
      type: openai
      api_key: ${OPENAI_API_KEY}

server:
  port: 8080
```

### SQLite + Qdrant Config

```yaml
# compozy.yaml (SQLite with external vector DB)
version: "1.0"

database:
  driver: sqlite
  path: ./data/compozy.db

knowledge:
  vector_dbs:
    - id: main
      provider: qdrant
      url: ${QDRANT_URL:-http://localhost:6333}
      dimension: 1536

llm:
  providers:
    - id: openai
      type: openai
      api_key: ${OPENAI_API_KEY}
```

### PostgreSQL Config (for comparison)

```yaml
# compozy.yaml (PostgreSQL)
version: "1.0"

database:
  driver: postgres
  host: ${DB_HOST:-localhost}
  port: 5432
  user: ${DB_USER:-compozy}
  password: ${DB_PASSWORD}
  dbname: ${DB_NAME:-compozy}
  sslmode: ${DB_SSLMODE:-disable}

knowledge:
  vector_dbs:
    - id: main
      provider: pgvector  # Uses PostgreSQL
      dimension: 1536

llm:
  providers:
    - id: openai
      type: openai
      api_key: ${OPENAI_API_KEY}
```

### Docker Compose - Qdrant

```yaml
# docker-compose.yml (Qdrant service)
version: "3.8"

services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
    volumes:
      - qdrant_data:/qdrant/storage
    environment:
      - QDRANT_LOG_LEVEL=INFO

volumes:
  qdrant_data:
```

### Docker Compose - PostgreSQL + pgAdmin

```yaml
# docker-compose.yml (PostgreSQL comparison example)
version: "3.8"

services:
  postgres:
    image: pgvector/pgvector:pg16
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_USER=compozy
      - POSTGRES_PASSWORD=compozy
      - POSTGRES_DB=compozy
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  pgadmin:
    image: dpage/pgadmin4:latest
    ports:
      - "5050:80"
    environment:
      - PGADMIN_DEFAULT_EMAIL=admin@compozy.com
      - PGADMIN_DEFAULT_PASSWORD=admin

volumes:
  postgres_data:
```

## Test & CI Coverage

### Integration Tests to Add

- **`test/integration/database/sqlite_test.go`**
  - Test SQLite-specific workflows
  - Verify file-based vs in-memory behavior
  - Test concurrent workflow execution (within SQLite limits)

- **`test/integration/database/multi_driver_test.go`**
  - Parameterized tests running against both drivers
  - Validate same workflow behavior regardless of database
  - Compare performance characteristics

- **`test/integration/database/vector_validation_test.go`**
  - Test SQLite + pgvector rejection (should fail)
  - Test SQLite + Qdrant acceptance (should pass)
  - Test PostgreSQL + pgvector acceptance (should pass)

### CI/CD Matrix

```yaml
# .github/workflows/database-examples.yml
name: Database Examples

on: [push, pull_request]

jobs:
  test-sqlite-examples:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: "1.25.2"
      
      - name: Test SQLite Quickstart
        run: |
          cd examples/database/sqlite-quickstart
          compozy start &
          sleep 5
          compozy workflow run hello-world
      
      - name: Test SQLite Memory
        run: |
          cd examples/database/sqlite-memory
          compozy start &
          sleep 5
          compozy workflow run test-workflow

  test-postgres-examples:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: pgvector/pgvector:pg16
        env:
          POSTGRES_PASSWORD: compozy
          POSTGRES_USER: compozy
          POSTGRES_DB: compozy
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: "1.25.2"
      
      - name: Test PostgreSQL Example
        run: |
          cd examples/database/postgres-comparison
          compozy start --config=compozy-postgres.yaml &
          sleep 5
          compozy workflow run benchmark
```

## Runbooks per Example

### SQLite Quickstart

**Prerequisites:**
- Compozy CLI installed
- OpenAI API key (or other LLM provider)

**Commands:**
```bash
# 1. Navigate to example
cd examples/database/sqlite-quickstart

# 2. Configure
cp .env.example .env
# Edit .env: Set OPENAI_API_KEY=sk-...

# 3. Start Compozy (database auto-created)
compozy start

# 4. Run workflow (in another terminal)
compozy workflow run hello-world

# 5. Verify database created
ls -lh ./data/compozy.db

# 6. View workflow status
compozy workflow list

# 7. Stop server
# Ctrl+C in terminal running compozy start
```

**Expected Output:**
```
Starting Compozy server...
Database initialized: driver=sqlite path=./data/compozy.db
Server listening on :8080

Workflow 'hello-world' completed successfully
Result: { ... }
```

---

### SQLite + Qdrant (Knowledge Base)

**Prerequisites:**
- Docker and Docker Compose
- Compozy CLI installed
- OpenAI API key

**Commands:**
```bash
# 1. Navigate to example
cd examples/database/sqlite-qdrant

# 2. Start Qdrant
docker-compose up -d

# 3. Verify Qdrant is running
curl http://localhost:6333/healthz

# 4. Configure
cp .env.example .env
# Edit .env: Set OPENAI_API_KEY=sk-...

# 5. Ingest knowledge
compozy knowledge ingest ./knowledge

# 6. Start Compozy
compozy start

# 7. Run RAG workflow
compozy workflow run rag-workflow \
  --input='{"query": "What is machine learning?"}'

# 8. Cleanup
docker-compose down
```

**Expected Output:**
```
Ingesting documents from ./knowledge...
✓ Processed 5 documents
✓ Created 42 chunks
✓ Embeddings stored in Qdrant

Workflow 'rag-workflow' completed
Answer: Machine learning is a subset of artificial intelligence...
Sources: [doc1.md, doc2.md]
```

---

### PostgreSQL vs SQLite Comparison

**Prerequisites:**
- Docker and Docker Compose
- Compozy CLI installed

**Commands:**
```bash
# 1. Navigate to example
cd examples/database/postgres-comparison

# 2. Start PostgreSQL
docker-compose up -d

# 3. Run benchmark with PostgreSQL
./benchmark.sh --config=compozy-postgres.yaml

# 4. Run benchmark with SQLite
./benchmark.sh --config=compozy-sqlite.yaml

# 5. Compare results
cat results-postgres.json
cat results-sqlite.json

# 6. Cleanup
docker-compose down
```

**Expected Output:**
```
Running benchmark with PostgreSQL...
10 workflows executed in 2.3s
Average latency: 230ms
Peak concurrency: 25 workflows

Running benchmark with SQLite...
10 workflows executed in 3.1s
Average latency: 310ms
Peak concurrency: 8 workflows
```

## Acceptance Criteria

- [ ] P0 examples (sqlite-quickstart, sqlite-qdrant) runnable locally
- [ ] README in each folder with clear instructions
- [ ] All code examples tested and working
- [ ] Docker Compose files validated (services start successfully)
- [ ] Environment variables documented in `.env.example`
- [ ] Expected outputs documented in READMEs
- [ ] CI/CD tests pass for all examples
- [ ] No hardcoded secrets or credentials
- [ ] Examples demonstrate key use cases:
  - [ ] Quick local development (SQLite file)
  - [ ] Testing/CI (SQLite memory)
  - [ ] Knowledge bases (SQLite + external vector DB)
  - [ ] Performance comparison (PostgreSQL vs SQLite)
  - [ ] Data migration (export/import)
  - [ ] Edge deployment (minimal footprint)
- [ ] Cross-references to documentation pages

## Additional Resources

### Sample Workflows

**`workflows/hello-world.yaml`** (for SQLite quickstart):
```yaml
id: hello-world
name: Hello World Workflow

tasks:
  - id: greet
    prompt: "Say hello to {{ .input.name | default \"World\" }}"
    agent:
      llm: openai

output:
  greeting: "{{ .tasks.greet.output.text }}"
```

**`workflows/rag-workflow.yaml`** (for SQLite + Qdrant):
```yaml
id: rag-workflow
name: RAG Workflow with Knowledge Base

tasks:
  - id: retrieve
    prompt: "Find relevant context for: {{ .input.query }}"
    agent:
      llm: openai
      knowledge:
        - knowledge_base_id: main
          top_k: 5

  - id: answer
    prompt: |
      Based on the following context:
      {{ .tasks.retrieve.output.text }}
      
      Answer: {{ .input.query }}
    agent:
      llm: openai

output:
  answer: "{{ .tasks.answer.output.text }}"
  sources: "{{ .tasks.retrieve.output.sources }}"
```

### Benchmark Script

**`scripts/benchmark.sh`:**
```bash
#!/bin/bash

CONFIG=${1:-compozy.yaml}
WORKFLOWS=${2:-10}

echo "Running benchmark with config: $CONFIG"
echo "Number of workflows: $WORKFLOWS"

# Start server in background
compozy --config="$CONFIG" start &
PID=$!
sleep 5

# Record start time
START=$(date +%s)

# Run workflows
for i in $(seq 1 $WORKFLOWS); do
  compozy workflow run benchmark &
done

# Wait for all workflows
wait

# Record end time
END=$(date +%s)
DURATION=$((END - START))

echo "Benchmark complete"
echo "Total time: ${DURATION}s"
echo "Average: $(( DURATION * 1000 / WORKFLOWS ))ms per workflow"

# Stop server
kill $PID
```

---

**Plan Version:** 1.0  
**Date:** 2025-01-27  
**Status:** Ready for Implementation
