# Compozy MCP Proxy

> A lightweight, secure **Model-Context-Protocol (MCP) gateway** that exposes multiple remote (or local) MCP servers through a single, unified HTTP interface with powerful administration, observability and security features.

---

## ✨ Key Features

- **Multi-Transport Support** – proxy MCP servers that speak **`stdio`**, **`sse`** or **`streamable-http`**.
- **Pluggable Storage** – in-memory (default) or **Redis** persistence for definitions & status.
- **Dynamic Registration** – hot-add, update or remove servers at runtime via the **Admin API**.
- **Built-in Security**
  - Admin token authentication & IP allow-lists.
  - Per-client and global auth tokens inherited by every request.
  - Trusted-proxy support for safe `X-Forwarded-For` handling.
- **Observability**
  - `/healthz` and `/admin/metrics` endpoints.
  - Per-client connection statistics & status tracking.
- **Automatic Reconnect & Health-Checks** with configurable intervals/back-off.
- **Tool, Prompt & Resource Discovery** – exposes downstream MCP capabilities through the proxy.

---

## 📦 Package Layout (high level)

```
pkg/mcp-proxy
├── admin_handlers.go      # Admin CRUD & tooling endpoints
├── proxy_handlers.go      # Core proxying logic (SSE / streamable HTTP)
├── server.go              # HTTP server bootstrap & routing
├── client_manager.go      # Manages multiple MCP client connections
├── client_mcp.go          # Thin wrapper around mark3labs/mcp-go clients
├── storage.go             # Redis & in-memory storage back-ends
├── types.go               # Shared type definitions & validation helpers
└── ... (tests & helpers)
```

---

## 🚀 Quick-Start (In-Memory Storage)

```bash
# 1. Run the proxy (binds to http://127.0.0.1:8080)
go run ./cmd/proxy/main.go # you may create your own main or use examples

# 2. Register an MCP server (stdio example)
curl -X POST http://127.0.0.1:8080/admin/mcps \
  -H 'Authorization: Bearer CHANGE_ME_ADMIN_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{
        "name": "echo-mcp",
        "transport": "stdio",
        "command": "echo",
        "args": ["hello world"],
        "logEnabled": true
      }'

# 3. Call the proxied server (SSE example)
curl http://127.0.0.1:8080/echo-mcp/sse
```

> **Note** – The default admin token is **`CHANGE_ME_ADMIN_TOKEN`** and **must** be replaced in production.

---

## ⚙️ Configuration Reference (`server.Config`)

| Field              | Type            | Default                     | Description                                          |
| ------------------ | --------------- | --------------------------- | ---------------------------------------------------- |
| `Port`             | `string`        | `"8080"`                    | TCP port to bind                                     |
| `Host`             | `string`        | `"127.0.0.1"`               | Listen address                                       |
| `BaseURL`          | `string`        | `"http://127.0.0.1:8080"`   | Base URL used when generating SSE paths              |
| `ShutdownTimeout`  | `time.Duration` | `10s`                       | Graceful shutdown deadline                           |
| `AdminTokens`      | `[]string`      | `["CHANGE_ME_ADMIN_TOKEN"]` | Allowed bearer tokens for Admin API                  |
| `AdminAllowIPs`    | `[]string`      | `["127.0.0.1", "::1"]`      | CIDR / IP allow-list for Admin API                   |
| `TrustedProxies`   | `[]string`      | `[]`                        | IP/CIDR list used to trust `X-Forwarded-For` headers |
| `GlobalAuthTokens` | `[]string`      | `[]`                        | Tokens injected into every proxied request           |

Configure these via code or inject via env-aware config loader of your choice.

---

## 📑 MCP Definition Schema (`MCPDefinition`)

```jsonc
{
  "name": "chat-llm", // unique id
  "description": "OpenAI ChatGPT",
  "transport": "sse", // stdio | sse | streamable-http

  // stdio-specific
  "command": "python",
  "args": ["server.py"],
  "env": { "PYTHONPATH": "." },

  // http-based specific
  "url": "https://llm.example.com/mcp",
  "headers": { "X-API-Key": "..." },
  "timeout": "30s",

  // security
  "authTokens": ["client-token"],
  "requireAuth": true,
  "allowedIPs": ["0.0.0.0/0"],

  // behaviour
  "autoReconnect": true,
  "maxReconnects": 5,
  "reconnectDelay": "5s",
  "healthCheckEnabled": true,
  "healthCheckInterval": "30s",

  // tool filtering (optional)
  "toolFilter": {
    "mode": "allow", // allow|block
    "list": ["search-tool"],
  },
}
```

> Call **`POST /admin/mcps`** with the JSON above to register.

---

## 🛡️ Security Model

1. **Admin API** (`/admin/**`)
   - Protected by **Bearer token** (`AdminTokens`).
   - Optional IP allow-list (`AdminAllowIPs`).
2. **Proxy Endpoints** (`/{name}/sse`, `/{name}/stream`)
   - Auth is **passed-through** to downstream MCP.
   - Per-client tokens (`authTokens`) **+** `GlobalAuthTokens` are automatically accepted.
3. **Trusted Proxies**
   - Configure `TrustedProxies` to safely read user IPs from `X-Forwarded-For`.

---

## 🌐 HTTP API Overview

### System Endpoints

| Method | Path             | Purpose               |
| ------ | ---------------- | --------------------- |
| `GET`  | `/healthz`       | Liveness probe        |
| `GET`  | `/admin/metrics` | JSON metrics snapshot |

### Admin API (requires token)

| Method   | Path                 | Description                               |
| -------- | -------------------- | ----------------------------------------- |
| `POST`   | `/admin/mcps`        | Register new MCP definition               |
| `PUT`    | `/admin/mcps/{name}` | Update definition & hot-reload connection |
| `DELETE` | `/admin/mcps/{name}` | Remove MCP & disconnect                   |
| `GET`    | `/admin/mcps`        | List all definitions (+ status)           |
| `GET`    | `/admin/mcps/{name}` | Get single definition                     |
| `GET`    | `/admin/tools`       | Aggregated list of all tools across MCPs  |

### Proxy Endpoints (public)

| Transport           | Path Pattern               | Notes                       |
| ------------------- | -------------------------- | --------------------------- |
| **SSE**             | `/{name}/sse[/{extra}]`    | Bi-directional event stream |
| **Streamable HTTP** | `/{name}/stream[/{extra}]` | Chunked HTTP streaming      |

> Additional path segments are forwarded to the downstream MCP verbatim.

---

## 📊 Metrics

The **ClientManager** exposes aggregate statistics:

```json
{
  "total_clients": 4,
  "connected": 3,
  "connecting": 0,
  "errored": 1,
  "total_requests": 1234,
  "total_errors": 8,
  "max_connections": 100
}
```

Retrieve via `GET /admin/metrics` (admin token required).

---

## 🗄️ Storage Back-Ends

### In-Memory (default)

Great for local testing – nothing to configure.

### Redis

```go
redisCfg := &mcpproxy.RedisConfig{
  Addr: "redis:6379",
  Password: "s3cr3t",
}
store, _ := mcpproxy.NewRedisStorage(redisCfg)
```

All definitions are stored under the prefix `mcp_proxy:*`.

---

## 🧩 Integration with Compozy Engine

The proxy is **first-class infrastructure** inside the Compozy runtime – several engine layers automatically discover and use it when present:

1. **`engine/mcp`** – Provides a strongly-typed Go client (`mcp.NewProxyClient`) that wraps the HTTP Admin & System APIs. Higher-level services build on top of this.
2. **`engine/mcp/service.go`** – The `RegisterService` keeps the proxy in sync with workflow YAML/JSON by hot-(de)registering MCP definitions at runtime.
3. **`engine/llm`** – When an Agent/Action declares an MCP in its config **and** `use_proxy: true`, the LLM service resolves tool calls **via the proxy**. It transparently calls `ListTools` and converts the returned schema into dynamic `llms.Tool`s.
4. **`engine/worker`** – During worker startup the `Worker` performs `HealthCheck` on the proxy to ensure connectivity before executing tasks.

### Minimal Engine Usage Example

```go
package main

import (
    "context"
    "time"

    "github.com/compozy/compozy/engine/mcp"
)

func main() {
    ctx := context.Background()

    // 1) Create a lightweight client (no retry backoff in this example)
    proxy := mcp.NewProxyClient("http://127.0.0.1:8080", "CHANGE_ME_ADMIN_TOKEN", 30*time.Second)

    // 2) Ping proxy – abort startup if unhealthy
    if err := proxy.Health(ctx); err != nil {
        panic(err)
    }

    // 3) (Optional) Register an MCP programmatically
    def := &mcp.Definition{
        Name:      "echo-mcp",
        Transport: "stdio",
        Command:   []string{"echo", "hello engine"},
    }
    if err := proxy.Register(ctx, def); err != nil {
        panic(err)
    }

    // 4) Discover all available tools (aggregated across MCPs)
    tools, err := proxy.ListTools(ctx)
    if err != nil {
        panic(err)
    }
    for _, t := range tools {
        println("Discovered tool:", t.Name, "from", t.MCPName)
    }
}
```

> **Tip** – Use `mcp.NewProxyClient(...).WithRetry(...)` if you need automatic exponential back-off.

### Auto-Registration via RegisterService

If you prefer declarative YAML instead of code, hand the definitions to the **`RegisterService`** and let it manage lifecycle:

```go
svc := mcp.NewWithTimeout("http://127.0.0.1:8080", "CHANGE_ME_ADMIN_TOKEN", 15*time.Second)
if err := svc.Ensure(ctx, &mcp.Config{
    ID:        "chat-llm",
    URL:       "https://llm.example.com/mcp",
    Transport: "sse",
    UseProxy:  true, // Critical – routes traffic via the proxy
}); err != nil {
    log.Error("failed to register", "error", err)
}
```

The service automatically performs health-checks and deregisters MCPs during graceful shutdown (`svc.Shutdown`).

### Workflow YAML Snippet

When designing a workflow you can declare external MCP servers under the `mcps:` section and simply set **`use_proxy: true`**. The engine will route all traffic through the proxy and automatically rewrite URLs when invoking tools.

```yaml
id: chat-qa-workflow
version: "0.1.0"

author:
  name: Jane Doe

actions: [] # omitted for brevity

mcps:
  # Down-stream chat LLM exposed via the proxy
  - id: chat-llm
    # Point to the *proxy* URL – not the raw LLM server
    url: http://127.0.0.1:8080/chat-llm/sse
    transport: sse # stdio | sse | streamable-http
    use_proxy: true # <- magic flag
    env:
      X-API-Key: "{{ .env.LLM_API_KEY }}"
    start_timeout: 10s
    max_sessions: 4

  # You can still mix direct (non-proxied) MCPs if desired
  - id: batch-mcp
    url: http://batch.local:4000/mcp
    transport: streamable-http
    env:
      TOKEN: "{{ .env.BATCH_TOKEN }}"
```

#### How it Works

1. At **worker startup** the `RegisterService` iterates over `mcps:` and sees `use_proxy: true`.
2. It strips the proxy URL prefix and registers the definition with the proxy's Admin API.
3. At runtime the LLM/Tool layer queries `ListTools` from the proxy instead of the raw server – benefiting from pooled connections, global auth & metrics.

### Agent YAML Snippet (Optional)

If you author an **agent** that directly lists MCPs it follows the same pattern:

```yaml
id: research-agent
config:
  provider: openai
  model: gpt-4o-mini

instructions: |
  You are a research assistant.

mcps:
  - id: search-tool
    url: http://127.0.0.1:8080/search-mcp/stream
    transport: streamable-http
    use_proxy: true

actions:
  - id: web-search
    prompt: "Find information about: {{ .input.query }}"
    tool: $mcp(search-tool#search_web)
    input:
      type: object
      properties:
        query:
          type: string
      required: [query]
```

With `use_proxy: true` the agent's tool calls are transparently routed via the proxy, inheriting all global auth tokens/config set on the proxy instance.

---

## 🛠️ Development & Testing

```bash
# Run linters, tests and examples
make fmt lint test

# Start proxy with default config
go run ./cmd/proxy/main.go

# Run unit tests with race detector
GOMAXPROCS=4 go test -race ./...
```

> Extensive unit tests cover concurrency, security, admin workflows & storage back-ends.

---

## 📚 Further Reading

- **MCP Specification** – <https://github.com/mark3labs/mcp>
- **Compozy Project Docs** – see `/docs` for higher-level architecture
- **pkg/mcp-proxy/types.go** – full schema & validation logic

---

## © License

Distributed under the MIT License. See `LICENSE` for more information.
