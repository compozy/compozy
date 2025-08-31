# Prompt-Only Basic Task Example

Minimal example showing a basic task calling an agent using a direct `prompt` (no actions).

## Run

1. Set your model credentials (example with OpenAI):

```bash
export OPENAI_API_KEY=your_key
```

2. Start the server and run the workflow via API (or UI):

```bash
# From repo root, then in another terminal use api.http or curl
```

3. Example request (see `api.http`):

```http
POST /api/v0/workflows/prompt-only/executions
{
  "input": { "text": "Compozy makes building AI workflows easier." }
}
```
