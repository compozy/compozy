# Knowledge Query CLI

## Goal

Learn how to iterate on retrieval parameters using `compozy knowledge query` with a small runbook knowledge base.

## Prerequisites

- `OPENAI_API_KEY`
- Compozy CLI installed

Prepare environment variables:

```bash
cp .env.example .env
```

## Steps

1. Apply definitions:
   ```bash
   compozy knowledge apply --file compozy.yaml
   ```
2. Ingest the runbook:
   ```bash
   compozy knowledge ingest --id runbook_kb --project .
   ```
3. Experiment with query parameters:
   ```bash
   compozy knowledge query --id runbook_kb --text "How long can downtime last?" --top_k 3 --min_score 0.2 --output json
   ```
4. Tighten the filter:
   ```bash
   compozy knowledge query --id runbook_kb --text "Who should we escalate to?" --filters category=runbook --top_k 2 --output table
   ```

## Verification

- Returned matches should include the troubleshooting steps from `docs/runbook.md`.
- Adjust `--min_score` to see how fewer matches change the response.

## Cleanup

Remove the `.env` file when finished:

```bash
rm -f .env
```
