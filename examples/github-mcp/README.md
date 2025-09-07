# GitHub MCP Example

This example shows how to use an MCP (Model Context Protocol) server for GitHub to list pull requests and return the latest PR for a given repository.

## What it does

- Receives a repository identifier (e.g., `owner/repo`).
- Uses the GitHub MCP server’s `list_pull_requests` tool via Compozy’s MCP proxy.
- Returns details of the most recent pull request.

## Prerequisites

- Compozy running in dev mode.
- A running MCP gateway that exposes the GitHub server.
- GitHub token configured on the gateway so the GitHub MCP can access the API.

### Remote MCP (HTTP streaming)

This example is configured to use the remote GitHub MCP server via HTTP streaming.

`workflow.yaml` config uses:

```
mcps:
  - id: github-mcp
    transport: streamable-http
    url: "https://api.githubcopilot.com/mcp"
    headers:
      Authorization: "{{ .env.GITHUB_MCP_AUTH_HEADER }}"
```

Provide the full Authorization header value via `GITHUB_MCP_AUTH_HEADER` in your `.env`.
This avoids double-prefix issues (e.g., "Bearer Bearer ..."):

Examples:

- `Bearer eyJhbGciOi...` (GitHub Copilot OAuth token; REQUIRED for the Copilot MCP endpoint)
- For local MCP servers (stdio), you may use: `token ghp_xxxxxxxxxxxxxxxxxxxxxx` (GitHub PAT)

### Environment variables

Export the variables required by this example:

```bash
export GROQ_API_KEY=...                       # required by the LLM
export MCP_PROXY_URL=http://localhost:6001    # MCP proxy base URL
export GITHUB_MCP_AUTH_HEADER="Bearer eyJ..." # Copilot OAuth token (required for remote Copilot MCP)
```

## Running

```bash
cd examples/github-mcp
../../compozy dev
```

Then, use the `api.http` file in this directory to execute the workflow and fetch the latest pull request. No local Docker container will be started in this mode.

Note: The MCP proxy does not include built-in admin authentication. Protect the `/admin` API using network controls (localhost binding, firewall) or a reverse proxy in your environment.

## Expected behavior

1. The workflow connects to the remote GitHub MCP server over SSE.
2. The agent calls the MCP tool to list pull requests for the provided repository.
3. The agent returns the most recent PR in a structured JSON format.

Note on models and tool-calling

- Ensure your selected model supports tool/function calling via the configured provider.
  If your model ignores tools, the agent may fabricate output. OpenAI models (e.g. `gpt-4.1`)
  generally support tool-calling robustly. Groq models may vary; switch the `models` block in
  `compozy.yaml` if you observe no tool calls being made.

## Notes

- Exact tool names/parameters vary by MCP implementation. The agent inspects tools and selects the appropriate one to list pull requests.
- Ensure `GITHUB_MCP_AUTH_HEADER` contains a valid value for the GitHub MCP server. For the remote Copilot MCP endpoint, you must provide a Copilot OAuth token in the form `Bearer <token>`.

## Dev container requirements

If you run the Compozy MCP Proxy from Docker Compose, it must be able to execute Docker commands:

- The proxy image installs Docker CLI (cluster/mcpproxy.Dockerfile)
- The compose service mounts `/var/run/docker.sock` and runs as root for local dev

Rebuild and restart the proxy after pulling these changes:

```
docker compose -f cluster/docker-compose.yml build compozy-mcp-proxy
docker compose -f cluster/docker-compose.yml up -d compozy-mcp-proxy
```
