# Quickstart: Markdown Glob Knowledge Base

## Goal

Provision a knowledge base from local Markdown files, ingest content with the in-memory vector store, and answer questions with a single workflow run.

## Prerequisites

- `OPENAI_API_KEY` with access to `text-embedding-3-small` and `gpt-4o-mini`
- Compozy CLI installed (`compozy` binary on your PATH)
- Repository dependencies installed (`make deps`)

Copy the sample environment file and set secrets before running any commands:

```bash
cp .env.example .env
```

## Steps

1. **Apply resources**
   ```bash
   compozy knowledge apply --file compozy.yaml
   ```
2. **Ingest Markdown docs**
   ```bash
   compozy knowledge ingest --id quickstart_docs --project .
   ```
3. **Run the workflow**
   ```bash
   compozy run workflows/qa.yaml --input '{"question":"What is our escalation policy?"}'
   ```

You can execute all steps at once with `./run.sh`.

## Verification

- Inspect ingestion logs for `knowledge.ingest.success` and chunk counts.
- Run an ad-hoc query:
  ```bash
  compozy knowledge query --id quickstart_docs --text "Define a knowledge base" --top_k 2 --output json
  ```
- Confirm the workflow response references content from `docs/support-playbook.md`.

## Teardown

No persistent resources are created when using the in-memory vector store. Use `./teardown.sh` to remove temporary files and unset environment variables.
