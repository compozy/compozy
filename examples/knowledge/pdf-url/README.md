# PDF URL Knowledge Base

## Goal

Ingest the public NIST incident handling guide PDF and answer questions about its contents using the knowledge service.

## Prerequisites

- `OPENAI_API_KEY`
- `REDIS_URL` pointing to a Redis 8 instance with vector sets enabled (Redis Stack 7.4+ or Redis 8 preview)
- Network access to `nvlpubs.nist.gov` for the NIST PDF

Prepare the environment:

```bash
cp .env.example .env
# start redis stack locally (optional helper)
# Redis 8 preview includes Vector Sets in beta
docker run --rm -p 6379:6379 redis:8.2-alpine
# update .env with:
# REDIS_URL=redis://localhost:6379
```

## Steps

1. **Apply resources**
   ```bash
   compozy knowledge apply --file compozy.yaml
   ```
2. **Ingest the remote PDF**
   ```bash
   compozy knowledge ingest --id pdf_demo --project .
   ```
3. **Query for validation**
   ```bash
   compozy knowledge query --id pdf_demo --text "Which lifecycle phases does NIST recommend for incident handling?" --top_k 3 --output table
   ```
4. **Run the workflow**
   ```bash
   compozy run workflows/qa.yaml --input '{"question":"Summarize the incident handling process described by NIST."}'
   ```

## Verification

- The query output should reference the NIST incident response lifecycle.
- Re-run ingestion and verify that unchanged content is skipped (idempotent run).

## Cleanup

Remove `.env` when finished:

```bash
rm -f .env
```
