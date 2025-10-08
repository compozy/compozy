# Pgvector Knowledge Base

## Goal

Demonstrate a production-like knowledge base backed by pgvector, including ingestion and query validation against a PostgreSQL container.

## Prerequisites

- `OPENAI_API_KEY`
- `PGVECTOR_DSN` (connection string without inline credentials)
- `PGVECTOR_PASSWORD` (used to populate `PGPASSWORD` at runtime)
- Docker running locally
- Compozy CLI installed

Prepare the environment:

```bash
cp .env.example .env
make start-docker
```

The default `make start-docker` target boots PostgreSQL with the pgvector extension on port `5433`.

## Steps

1. **Apply resources**
   ```bash
   compozy knowledge apply --file compozy.yaml
   ```
2. **Run ingestion**
   ```bash
   compozy knowledge ingest --id company_docs --project . --batch-size 48
   ```
3. **Verify retrieval**
   ```bash
   compozy knowledge query --id company_docs --text "What is our uptime target?" --top_k 3 --min_score 0.2 --output table
   ```
4. **Execute the workflow**
   ```bash
   compozy run workflows/qa.yaml --input '{"question":"How quickly do we publish retrospectives?"}'
   ```

Use `./scripts/run.sh` to execute the full sequence.

## Verification

- Check `knowledge_ingest_duration_seconds` in your metrics endpoint.
- Inspect pgvector rows:
  ```bash
  PGPASSWORD="$PGVECTOR_PASSWORD" psql "$PGVECTOR_DSN" -c 'SELECT COUNT(*) FROM knowledge_chunks;'
  ```
- Confirm the workflow response cites the handbook content committed in `docs/`.

## Teardown

```bash
./scripts/teardown.sh
make stop-docker
```

The teardown script drops the `knowledge_chunks` table to keep the database clean between runs.
