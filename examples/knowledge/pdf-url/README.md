# PDF URL Knowledge Base

## Goal

Ingest a small remote PDF and answer questions about its contents using the knowledge service.

## Prerequisites

- `OPENAI_API_KEY`
- Network access to `w3.org` for the sample PDF

Prepare the environment:

```bash
cp .env.example .env
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
   compozy knowledge query --id pdf_demo --text "What placeholder text does the PDF contain?" --top_k 3 --output table
   ```
4. **Run the workflow**
   ```bash
   compozy run workflows/qa.yaml --input '{"question":"Summarize the PDF"}'
   ```

## Verification

- The query output should reference the dummy placeholder text from the PDF.
- Re-run ingestion and verify that unchanged content is skipped (idempotent run).

## Cleanup

Remove `.env` when finished:

```bash
rm -f .env
```
