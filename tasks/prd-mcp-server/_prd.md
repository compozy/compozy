# PRD: MCP Server - Expose Compozy Actions via Model Context Protocol

**Status:** Draft
**Version:** 1.0.0
**Created:** 2025-10-31
**Owner:** Product Team
**Contributors:** Engineering, Architecture

---

## Executive Summary

This PRD defines the implementation of a **Model Context Protocol (MCP) Server** that automatically exposes Compozy's REST API actions as MCP tools, enabling users to interact with workflows, agents, and tasks through AI applications that support the MCP standard (Claude Desktop, VSCode Copilot, etc.).

### Key Benefits
- **Unified Access Pattern**: Use workflows/agents/tasks via both REST API and MCP protocol
- **AI-Native Integration**: Direct integration with MCP-compatible AI applications
- **Zero Configuration**: Automatic tool generation from existing REST API endpoints
- **Consistent Behavior**: Same business logic for REST and MCP access paths

---

## 1. Problem Statement

### Current State
Compozy currently provides a comprehensive REST API at `/api/v0/*` that enables users to:
- Create and execute workflows
- Define and run agents
- Manage tasks, tools, models, schemas
- Stream execution results via SSE
- Control execution flow (pause, resume, cancel)

However, users working with AI applications (Claude Desktop, VSCode Copilot, Windsurf, etc.) cannot directly access Compozy capabilities through the **Model Context Protocol (MCP)**, which is becoming the standard for AI tool integration.

### User Pain Points
1. **Manual Integration Overhead**: Users must write custom scripts or API wrappers to integrate Compozy with MCP-compatible AI applications
2. **Fragmented Experience**: Cannot use natural language (via AI) to trigger Compozy workflows or query execution status
3. **Missing AI-Native Patterns**: No first-class support for the MCP ecosystem that modern AI applications use
4. **Duplicate Implementation**: Teams building MCP integrations duplicate the same API-to-MCP mapping logic

### Opportunity
By providing an **automatic MCP server** that mirrors the REST API as MCP tools, we enable:
- **Direct AI Integration**: Claude Desktop users can say "Run my data-processing workflow" and have it execute via MCP
- **Developer Productivity**: Teams building AI-powered automation can use Compozy actions as tools in their AI applications
- **Future-Proof Architecture**: Position Compozy as a first-class AI orchestration platform with native MCP support

---

## 2. Goals & Non-Goals

### Goals
1. **Automatic MCP Server**: Expose all REST API actions as MCP tools without manual configuration
2. **Parity with REST API**: MCP tools should provide equivalent functionality to REST endpoints
3. **Zero-Configuration Start**: MCP server starts automatically when Compozy server runs with default path `/mcp`
4. **Standards Compliance**: Implement MCP protocol specification (2025-03-26) correctly
5. **Seamless Integration**: Work with existing Compozy authentication, monitoring, and configuration systems

### Non-Goals
1. **Custom MCP Tools**: This PRD does not cover user-defined MCP tools beyond REST API actions
2. **Bidirectional Sync**: Not implementing MCP → REST API conversion (only REST → MCP)
3. **MCP Client Features**: Not implementing MCP client functionality (Compozy already has this via `engine/mcp` and `pkg/mcp-proxy`)
4. **UI for MCP Management**: Admin UI for MCP server management is out of scope

---

## 3. Success Metrics

### Key Performance Indicators (KPIs)

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| **MCP Server Startup Time** | < 500ms | Server initialization telemetry |
| **Tool Discovery Performance** | < 100ms for ListTools | MCP protocol response time |
| **Tool Execution Latency** | < 50ms overhead vs REST | Comparative benchmarks |
| **API Coverage** | 100% of REST endpoints | Automated endpoint scanning |
| **Adoption Rate** | 20% of users try MCP within 3 months | Usage telemetry |

### Success Criteria
- [ ] MCP server starts automatically on `/mcp` path
- [ ] All REST API endpoints exposed as MCP tools with correct schemas
- [ ] MCP tools can execute workflows, agents, and tasks successfully
- [ ] Documentation includes MCP integration examples for Claude Desktop, VSCode
- [ ] Zero P0/P1 bugs in production after 1 month

---

## 4. User Stories & Use Cases

### Primary User Personas

#### Persona 1: AI Power User (Sarah)
**Background**: Data scientist using Claude Desktop for data pipeline automation
**Need**: Execute Compozy workflows directly from Claude Desktop via natural language

**User Story**:
```
As Sarah, I want to tell Claude Desktop "Run my daily ETL workflow"
So that I can trigger Compozy workflows without switching to API clients or web UI
```

**Acceptance Criteria**:
- Sarah configures Claude Desktop with Compozy MCP server URL (`http://localhost:3000/mcp`)
- Claude Desktop discovers all available Compozy workflows as MCP tools
- Sarah can execute workflows by natural language commands in Claude Desktop
- Execution status and results are returned through MCP protocol

---

#### Persona 2: DevOps Engineer (Mike)
**Background**: Building internal automation with Windsurf IDE and Compozy
**Need**: Integrate Compozy agent execution into AI-assisted coding workflows

**User Story**:
```
As Mike, I want to invoke Compozy agents from Windsurf IDE
So that I can build AI-powered automation pipelines with my existing Compozy resources
```

**Acceptance Criteria**:
- Mike adds Compozy MCP server to Windsurf configuration
- Windsurf discovers Compozy agents as available tools
- Mike can chain agent execution with other MCP tools in Windsurf
- Agent outputs are properly formatted for AI consumption

---

#### Persona 3: Application Developer (Emma)
**Background**: Building SaaS product that needs to orchestrate AI workflows
**Need**: Programmatic MCP access to Compozy for embedding in applications

**User Story**:
```
As Emma, I want to connect to Compozy via MCP from my application
So that I can provide users with AI-powered workflow orchestration features
```

**Acceptance Criteria**:
- Emma can connect MCP clients programmatically to Compozy MCP server
- All Compozy resources (workflows, agents, tasks) are accessible via MCP
- Authentication works seamlessly with API keys via MCP protocol
- Execution monitoring works through MCP streaming capabilities

---

### Use Case 1: Workflow Execution via Claude Desktop

**Scenario**: User executes a data processing workflow through Claude Desktop

**Flow**:
1. User opens Claude Desktop and says: "Run the customer-data-processing workflow"
2. Claude Desktop queries Compozy MCP server for available tools
3. Compozy MCP server returns list of workflows as MCP tools
4. Claude invokes `compozy.workflow.execute` tool with workflow_id
5. Compozy executes workflow via existing workflow engine
6. Execution status and results returned through MCP protocol
7. Claude presents results to user in natural language

**Expected Result**: Workflow executes successfully, user sees execution ID and can query status

---

### Use Case 2: Agent Invocation from VSCode Copilot

**Scenario**: Developer uses VSCode Copilot to run Compozy agent for code review

**Flow**:
1. Developer configures VSCode with Compozy MCP server
2. Developer asks Copilot: "Review this code with my security-review agent"
3. VSCode Copilot discovers `compozy.agent.execute` tool via MCP
4. Copilot invokes agent with code context as input
5. Compozy agent analyzes code and returns findings
6. Results displayed in VSCode with inline annotations

**Expected Result**: Agent executes, returns structured output, developer sees findings

---

### Use Case 3: Task Monitoring via MCP

**Scenario**: User monitors running task execution status through MCP

**Flow**:
1. User starts long-running workflow via MCP
2. User queries `compozy.execution.status` tool with execution ID
3. Compozy returns current execution state, progress, and logs
4. User can pause, resume, or cancel execution via MCP tools
5. User receives completion notification when execution finishes

**Expected Result**: Full execution lifecycle manageable through MCP protocol

---

## 5. Requirements

### 5.1 Functional Requirements

#### FR-1: MCP Server Initialization
**Priority**: P0 (Must Have)

- **FR-1.1**: MCP server MUST start automatically when Compozy server starts in standalone or distributed mode
- **FR-1.2**: MCP server MUST be accessible at `/mcp` path (e.g., `http://localhost:3000/mcp`)
- **FR-1.3**: MCP server MUST implement SSE (Server-Sent Events) transport as primary protocol
- **FR-1.4**: MCP server MUST implement protocol version `2025-03-26` (current MCP standard)
- **FR-1.5**: MCP server startup MUST complete within 500ms
- **FR-1.6**: MCP server MUST gracefully shutdown with server lifecycle

---

#### FR-2: Tool Discovery & Registration
**Priority**: P0 (Must Have)

- **FR-2.1**: MCP server MUST automatically expose ALL REST API endpoints as MCP tools
- **FR-2.2**: Tool names MUST follow format: `compozy.{resource}.{action}` (e.g., `compozy.workflow.execute`, `compozy.agent.list`)
- **FR-2.3**: Tool schemas MUST be auto-generated from OpenAPI specification (Swagger)
- **FR-2.4**: Tool descriptions MUST be human-readable and AI-friendly
- **FR-2.5**: Tool input schemas MUST match REST API JSON schemas exactly
- **FR-2.6**: Tool discovery (`tools/list`) MUST complete in < 100ms

**Tool Naming Examples**:
```
compozy.workflow.create          → POST /api/v0/workflows
compozy.workflow.execute         → POST /api/v0/workflows/{id}/executions
compozy.workflow.get             → GET /api/v0/workflows/{id}
compozy.workflow.list            → GET /api/v0/workflows
compozy.workflow.delete          → DELETE /api/v0/workflows/{id}

compozy.agent.create             → POST /api/v0/agents
compozy.agent.execute            → POST /api/v0/agents/{id}/executions
compozy.agent.list               → GET /api/v0/agents

compozy.execution.status         → GET /api/v0/executions/{id}
compozy.execution.pause          → POST /api/v0/executions/{id}/pause
compozy.execution.resume         → POST /api/v0/executions/{id}/resume
compozy.execution.cancel         → POST /api/v0/executions/{id}/cancel
compozy.execution.stream         → GET /api/v0/executions/{id}/stream (SSE)

compozy.task.create              → POST /api/v0/workflows/{wf_id}/tasks
compozy.task.update              → PUT /api/v0/workflows/{wf_id}/tasks/{id}

compozy.model.list               → GET /api/v0/models
compozy.schema.list              → GET /api/v0/schemas
compozy.tool.list                → GET /api/v0/tools
```

---

#### FR-3: Tool Execution
**Priority**: P0 (Must Have)

- **FR-3.1**: Tool execution MUST route to corresponding REST API endpoint handler
- **FR-3.2**: Tool execution MUST preserve ALL REST API business logic (no duplication)
- **FR-3.3**: Tool execution MUST support all REST API input formats (JSON body, path params, query params)
- **FR-3.4**: Tool execution MUST return results in MCP-compliant format
- **FR-3.5**: Tool execution MUST handle errors and map to MCP error codes
- **FR-3.6**: Tool execution overhead MUST be < 50ms compared to direct REST API call

**Execution Flow**:
```
MCP Client Request
  ↓
/mcp (MCP Server Endpoint)
  ↓
MCP Protocol Handler (parse MCP request)
  ↓
Tool Router (map tool name → REST endpoint)
  ↓
REST API Handler (existing business logic)
  ↓
Response Transformer (REST → MCP format)
  ↓
MCP Protocol Response
  ↓
MCP Client
```

---

#### FR-4: Authentication & Authorization
**Priority**: P0 (Must Have)

- **FR-4.1**: MCP server MUST support same authentication as REST API (API keys)
- **FR-4.2**: MCP server MUST enforce same authorization rules as REST API
- **FR-4.3**: Authentication MUST work via MCP protocol headers or query parameters
- **FR-4.4**: Unauthorized requests MUST return MCP-compliant error responses
- **FR-4.5**: Admin-only endpoints MUST be restricted in MCP tools (same as REST)

---

#### FR-5: Streaming & Asynchronous Operations
**Priority**: P1 (Should Have)

- **FR-5.1**: MCP server SHOULD support streaming responses for long-running operations
- **FR-5.2**: Execution streaming SHOULD mirror existing SSE implementation at `/api/v0/executions/{id}/stream`
- **FR-5.3**: MCP clients SHOULD be able to subscribe to execution updates via MCP streaming
- **FR-5.4**: Streaming SHOULD support same event types as REST API (status, output, error, complete)

---

#### FR-6: Configuration & Customization
**Priority**: P1 (Should Have)

- **FR-6.1**: MCP server path SHOULD be configurable via environment variable `MCP_SERVER_PATH` (default: `/mcp`)
- **FR-6.2**: MCP server SHOULD be disableable via configuration flag `server.mcp.enabled: false`
- **FR-6.3**: Tool naming conventions SHOULD be customizable via config (e.g., prefix override)
- **FR-6.4**: Tool filtering SHOULD allow excluding specific endpoints from MCP exposure

---

### 5.2 Non-Functional Requirements

#### NFR-1: Performance
- MCP server MUST handle 100 concurrent tool discovery requests without degradation
- Tool execution MUST add < 50ms latency vs direct REST API call
- MCP server MUST support at least 1000 requests/minute per instance
- Memory overhead for MCP server MUST be < 50MB at startup

#### NFR-2: Reliability
- MCP server MUST have 99.9% uptime (same as REST API)
- MCP server failures MUST NOT crash main Compozy server
- Tool execution errors MUST be isolated and not affect other operations
- MCP server MUST gracefully handle malformed MCP requests

#### NFR-3: Observability
- MCP server MUST expose Prometheus metrics (request count, latency, errors)
- MCP server MUST log all tool invocations with structured logging
- MCP server health MUST be checkable via `/mcp/healthz` endpoint
- MCP server metrics MUST be integrated into existing monitoring dashboards

#### NFR-4: Compatibility
- MCP server MUST work with Claude Desktop, VSCode Copilot, Windsurf, and other MCP clients
- MCP server MUST be backwards compatible with existing REST API
- MCP server MUST follow MCP protocol specification version 2025-03-26
- MCP server MUST support HTTP/1.1 and HTTP/2

#### NFR-5: Security
- MCP server MUST enforce rate limiting (same as REST API)
- MCP server MUST sanitize all inputs to prevent injection attacks
- MCP server MUST NOT expose internal implementation details in errors
- MCP server MUST support CORS for browser-based MCP clients

---

## 6. Technical Architecture

### 6.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Compozy Server (Port 3000)                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────────────────────────────────────────────┐    │
│  │           REST API (/api/v0/*)                         │    │
│  │  ┌─────────────────────────────────────────────────┐  │    │
│  │  │  Workflow Routes                                 │  │    │
│  │  │  Agent Routes                                    │  │    │
│  │  │  Task Routes                                     │  │    │
│  │  │  Execution Routes                                │  │    │
│  │  │  Model/Schema/Tool Routes                        │  │    │
│  │  └─────────────────────────────────────────────────┘  │    │
│  └───────────────────────────────────────────────────────┘    │
│                          ↑                                      │
│                          │ (shared handlers)                   │
│                          │                                      │
│  ┌───────────────────────────────────────────────────────┐    │
│  │         MCP Server (/mcp) ← NEW                       │    │
│  │  ┌─────────────────────────────────────────────────┐  │    │
│  │  │  MCP Protocol Handler (SSE)                      │  │    │
│  │  │  ├─ tools/list                                   │  │    │
│  │  │  ├─ tools/call                                   │  │    │
│  │  │  ├─ resources/list (optional)                    │  │    │
│  │  │  └─ prompts/list (optional)                      │  │    │
│  │  │                                                   │  │    │
│  │  │  Tool Registry (auto-generated from OpenAPI)     │  │    │
│  │  │  ├─ compozy.workflow.*                           │  │    │
│  │  │  ├─ compozy.agent.*                              │  │    │
│  │  │  ├─ compozy.task.*                               │  │    │
│  │  │  ├─ compozy.execution.*                          │  │    │
│  │  │  └─ compozy.{resource}.*                         │  │    │
│  │  │                                                   │  │    │
│  │  │  Tool Executor (routes to REST handlers)         │  │    │
│  │  │  └─ Request Transformer (MCP → REST)             │  │    │
│  │  │  └─ Response Transformer (REST → MCP)            │  │    │
│  │  └─────────────────────────────────────────────────┘  │    │
│  └───────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

External Clients:
┌─────────────────┐      ┌─────────────────┐      ┌──────────────┐
│ Claude Desktop  │      │ VSCode Copilot  │      │  Windsurf    │
│                 │      │                 │      │              │
│ MCP Client      │      │ MCP Client      │      │ MCP Client   │
└────────┬────────┘      └────────┬────────┘      └──────┬───────┘
         │                        │                       │
         └────────────────────────┴───────────────────────┘
                                  │
                      HTTP/SSE to /mcp
```

---

### 6.2 Component Design

#### Component 1: MCP Server (`engine/infra/server/mcp_server.go`)

**Responsibilities**:
- Initialize MCP server on `/mcp` path
- Handle MCP protocol messages (initialize, tools/list, tools/call, etc.)
- Coordinate with Tool Registry and Tool Executor
- Manage MCP client connections and sessions

**Key Methods**:
```go
type MCPServer struct {
    router       *gin.Engine
    toolRegistry *MCPToolRegistry
    toolExecutor *MCPToolExecutor
    config       *config.Config
}

func NewMCPServer(router *gin.Engine, cfg *config.Config) *MCPServer
func (s *MCPServer) RegisterRoutes()
func (s *MCPServer) handleInitialize(c *gin.Context)
func (s *MCPServer) handleToolsList(c *gin.Context)
func (s *MCPServer) handleToolsCall(c *gin.Context)
func (s *MCPServer) handleHealthCheck(c *gin.Context)
```

---

#### Component 2: Tool Registry (`engine/infra/server/mcp_tool_registry.go`)

**Responsibilities**:
- Auto-generate MCP tools from OpenAPI specification
- Maintain mapping of tool names to REST endpoints
- Provide tool metadata (name, description, input schema)
- Handle tool discovery requests

**Key Methods**:
```go
type MCPToolRegistry struct {
    tools map[string]*MCPToolDefinition
    swagger *OpenAPISpec
}

type MCPToolDefinition struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
    RestMethod  string
    RestPath    string
    RestHandler gin.HandlerFunc
}

func NewMCPToolRegistry(swagger *OpenAPISpec) *MCPToolRegistry
func (r *MCPToolRegistry) LoadFromOpenAPI() error
func (r *MCPToolRegistry) ListTools() []MCPToolDefinition
func (r *MCPToolRegistry) GetTool(name string) (*MCPToolDefinition, error)
func (r *MCPToolRegistry) MapToolNameToEndpoint(name string) (string, string)
```

**Tool Generation Logic**:
```go
// Example: Generate tool from OpenAPI endpoint
// POST /api/v0/workflows/{id}/executions → compozy.workflow.execute

OpenAPI Endpoint:
  Path: /api/v0/workflows/{id}/executions
  Method: POST
  Summary: "Execute a workflow"
  RequestBody: {...}

↓ Transform to

MCP Tool:
  Name: compozy.workflow.execute
  Description: "Execute a workflow by ID"
  InputSchema: {
    type: "object",
    properties: {
      workflow_id: {type: "string", description: "Workflow ID"},
      input: {type: "object", description: "Workflow input parameters"}
    },
    required: ["workflow_id"]
  }
```

---

#### Component 3: Tool Executor (`engine/infra/server/mcp_tool_executor.go`)

**Responsibilities**:
- Execute tools by routing to REST API handlers
- Transform MCP requests to REST API format
- Transform REST API responses to MCP format
- Handle execution errors and map to MCP error codes

**Key Methods**:
```go
type MCPToolExecutor struct {
    registry *MCPToolRegistry
    router   *gin.Engine
}

func NewMCPToolExecutor(registry *MCPToolRegistry, router *gin.Engine) *MCPToolExecutor
func (e *MCPToolExecutor) Execute(ctx *gin.Context, toolName string, args map[string]interface{}) (interface{}, error)
func (e *MCPToolExecutor) transformMCPRequestToREST(toolDef *MCPToolDefinition, args map[string]interface{}) (*http.Request, error)
func (e *MCPToolExecutor) transformRESTResponseToMCP(resp *http.Response) (interface{}, error)
func (e *MCPToolExecutor) handleError(err error) *MCPError
```

**Execution Flow**:
```go
// MCP Client calls: compozy.workflow.execute(workflow_id="wf-123", input={...})

1. MCPServer.handleToolsCall() receives MCP request
2. ToolExecutor.Execute("compozy.workflow.execute", args)
3. ToolRegistry.GetTool("compozy.workflow.execute") → returns tool definition
4. ToolExecutor.transformMCPRequestToREST() → creates internal HTTP request
   - Method: POST
   - Path: /api/v0/workflows/wf-123/executions
   - Body: args["input"]
5. Execute REST handler via router.ServeHTTP()
6. ToolExecutor.transformRESTResponseToMCP() → converts response to MCP format
7. Return MCP response to client
```

---

#### Component 4: MCP Protocol Handler (`engine/infra/server/mcp_protocol.go`)

**Responsibilities**:
- Implement MCP protocol specification (2025-03-26)
- Handle SSE transport for streaming responses
- Manage MCP client sessions and state
- Serialize/deserialize MCP messages

**Key Methods**:
```go
type MCPProtocolHandler struct {
    version string // "2025-03-26"
}

type MCPRequest struct {
    JSONRPC string                 `json:"jsonrpc"`
    Method  string                 `json:"method"`
    Params  map[string]interface{} `json:"params"`
    ID      interface{}            `json:"id"`
}

type MCPResponse struct {
    JSONRPC string      `json:"jsonrpc"`
    Result  interface{} `json:"result,omitempty"`
    Error   *MCPError   `json:"error,omitempty"`
    ID      interface{} `json:"id"`
}

func NewMCPProtocolHandler() *MCPProtocolHandler
func (h *MCPProtocolHandler) ParseRequest(body []byte) (*MCPRequest, error)
func (h *MCPProtocolHandler) SerializeResponse(resp *MCPResponse) ([]byte, error)
func (h *MCPProtocolHandler) HandleSSE(c *gin.Context)
```

---

### 6.3 Integration Points

#### Integration 1: Server Initialization
```go
// In engine/infra/server/server.go

func (s *Server) setupMCPServer(ctx context.Context) error {
    cfg := config.FromContext(ctx)
    if !cfg.Server.MCP.Enabled {
        return nil
    }

    // Create MCP server
    mcpServer := NewMCPServer(s.router, cfg)

    // Register MCP routes under /mcp
    mcpServer.RegisterRoutes()

    s.mcpServer = mcpServer
    return nil
}
```

#### Integration 2: Route Registration
```go
// In engine/infra/server/mcp_server.go

func (s *MCPServer) RegisterRoutes() {
    mcpGroup := s.router.Group("/mcp")
    {
        mcpGroup.POST("/", s.handleMCPRequest)           // Main MCP endpoint
        mcpGroup.GET("/healthz", s.handleHealthCheck)    // Health check
        mcpGroup.GET("/sse", s.handleSSEStream)          // SSE streaming
    }
}
```

#### Integration 3: Metrics & Monitoring
```go
// Prometheus metrics for MCP server

var (
    mcpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "compozy_mcp_requests_total",
            Help: "Total number of MCP requests",
        },
        []string{"method", "status"},
    )

    mcpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "compozy_mcp_request_duration_seconds",
            Help: "MCP request duration in seconds",
        },
        []string{"method"},
    )

    mcpToolCallsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "compozy_mcp_tool_calls_total",
            Help: "Total number of MCP tool calls",
        },
        []string{"tool_name", "status"},
    )
)
```

---

### 6.4 Configuration Schema

```yaml
# config.yaml (additions)

server:
  mcp:
    enabled: true                    # Enable/disable MCP server
    path: "/mcp"                     # MCP server path (default: /mcp)
    transport: "sse"                 # Transport type (sse, http, stdio)
    protocol_version: "2025-03-26"   # MCP protocol version
    tool_prefix: "compozy"           # Tool name prefix (default: compozy)

    # Tool filtering
    exclude_endpoints:               # REST endpoints to exclude from MCP
      - "/api/v0/admin/*"            # Exclude admin endpoints
      - "/api/v0/internal/*"         # Exclude internal endpoints

    # Performance tuning
    max_concurrent_requests: 100     # Max concurrent MCP requests
    request_timeout: 30s             # Request timeout

    # Streaming configuration
    sse_heartbeat_interval: 15s      # SSE heartbeat interval
    sse_retry_interval: 5s           # SSE retry interval
```

**Environment Variables**:
```bash
COMPOZY_SERVER_MCP_ENABLED=true
COMPOZY_SERVER_MCP_PATH=/mcp
COMPOZY_SERVER_MCP_PROTOCOL_VERSION=2025-03-26
```

---

### 6.5 Error Handling

**MCP Error Codes** (following JSON-RPC 2.0):
```go
const (
    MCPErrorParseError     = -32700 // Invalid JSON
    MCPErrorInvalidRequest = -32600 // Invalid MCP request
    MCPErrorMethodNotFound = -32601 // Tool not found
    MCPErrorInvalidParams  = -32602 // Invalid tool parameters
    MCPErrorInternalError  = -32603 // Server error

    // Custom error codes
    MCPErrorAuthFailed     = -32000 // Authentication failed
    MCPErrorNotAuthorized  = -32001 // Not authorized
    MCPErrorRateLimited    = -32002 // Rate limit exceeded
    MCPErrorToolFailed     = -32003 // Tool execution failed
)
```

**Error Response Format**:
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32601,
    "message": "Tool not found",
    "data": {
      "tool_name": "compozy.invalid.tool",
      "available_tools": ["compozy.workflow.execute", "..."]
    }
  },
  "id": 1
}
```

---

## 7. API Specification

### 7.1 MCP Protocol Endpoints

#### Endpoint: Initialize Session
```http
POST /mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "capabilities": {
      "tools": {}
    },
    "clientInfo": {
      "name": "Claude Desktop",
      "version": "1.0.0"
    }
  },
  "id": 1
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "protocolVersion": "2025-03-26",
    "capabilities": {
      "tools": {
        "listChanged": false
      },
      "resources": {},
      "prompts": {}
    },
    "serverInfo": {
      "name": "Compozy MCP Server",
      "version": "0.1.0"
    }
  },
  "id": 1
}
```

---

#### Endpoint: List Tools
```http
POST /mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 2
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "compozy.workflow.execute",
        "description": "Execute a workflow by ID",
        "inputSchema": {
          "type": "object",
          "properties": {
            "workflow_id": {
              "type": "string",
              "description": "Unique workflow identifier"
            },
            "input": {
              "type": "object",
              "description": "Workflow input parameters"
            },
            "sync": {
              "type": "boolean",
              "description": "Wait for completion (default: false)",
              "default": false
            }
          },
          "required": ["workflow_id"]
        }
      },
      {
        "name": "compozy.agent.execute",
        "description": "Execute an agent by ID",
        "inputSchema": {
          "type": "object",
          "properties": {
            "agent_id": {
              "type": "string",
              "description": "Unique agent identifier"
            },
            "prompt": {
              "type": "string",
              "description": "Agent prompt"
            }
          },
          "required": ["agent_id", "prompt"]
        }
      },
      {
        "name": "compozy.execution.status",
        "description": "Get execution status by ID",
        "inputSchema": {
          "type": "object",
          "properties": {
            "execution_id": {
              "type": "string",
              "description": "Execution ID"
            }
          },
          "required": ["execution_id"]
        }
      }
    ]
  },
  "id": 2
}
```

---

#### Endpoint: Call Tool
```http
POST /mcp
Content-Type: application/json

{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "compozy.workflow.execute",
    "arguments": {
      "workflow_id": "wf-customer-data-processing",
      "input": {
        "date_range": "2025-10-01:2025-10-31",
        "output_format": "csv"
      },
      "sync": false
    }
  },
  "id": 3
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Workflow execution started successfully"
      },
      {
        "type": "resource",
        "resource": {
          "uri": "compozy://executions/exec-abc123",
          "name": "Execution exec-abc123",
          "mimeType": "application/json"
        }
      }
    ],
    "isError": false
  },
  "id": 3
}
```

**Error Response**:
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid workflow_id",
    "data": {
      "field": "workflow_id",
      "reason": "Workflow not found",
      "provided_value": "wf-invalid"
    }
  },
  "id": 3
}
```

---

### 7.2 Tool Catalog (Auto-Generated)

#### Workflow Tools
```yaml
- name: compozy.workflow.create
  rest: POST /api/v0/workflows
  description: Create a new workflow definition
  input:
    - id: string (required)
    - name: string
    - tasks: array
    - mcps: array

- name: compozy.workflow.execute
  rest: POST /api/v0/workflows/{id}/executions
  description: Execute a workflow by ID
  input:
    - workflow_id: string (required)
    - input: object
    - sync: boolean (default: false)

- name: compozy.workflow.get
  rest: GET /api/v0/workflows/{id}
  description: Get workflow definition by ID
  input:
    - workflow_id: string (required)

- name: compozy.workflow.list
  rest: GET /api/v0/workflows
  description: List all workflows
  input:
    - limit: integer (default: 50)
    - cursor: string

- name: compozy.workflow.update
  rest: PUT /api/v0/workflows/{id}
  description: Update workflow definition
  input:
    - workflow_id: string (required)
    - name: string
    - tasks: array
    - mcps: array

- name: compozy.workflow.delete
  rest: DELETE /api/v0/workflows/{id}
  description: Delete workflow by ID
  input:
    - workflow_id: string (required)
```

#### Agent Tools
```yaml
- name: compozy.agent.create
  rest: POST /api/v0/agents
  description: Create a new agent definition
  input:
    - id: string (required)
    - name: string
    - model: string
    - workflows: array

- name: compozy.agent.execute
  rest: POST /api/v0/agents/{id}/executions
  description: Execute an agent by ID
  input:
    - agent_id: string (required)
    - prompt: string (required)
    - context: object

- name: compozy.agent.get
  rest: GET /api/v0/agents/{id}
  description: Get agent definition by ID
  input:
    - agent_id: string (required)

- name: compozy.agent.list
  rest: GET /api/v0/agents
  description: List all agents
  input:
    - limit: integer (default: 50)
    - cursor: string
```

#### Execution Tools
```yaml
- name: compozy.execution.status
  rest: GET /api/v0/executions/{id}
  description: Get execution status and results
  input:
    - execution_id: string (required)

- name: compozy.execution.pause
  rest: POST /api/v0/executions/{id}/pause
  description: Pause running execution
  input:
    - execution_id: string (required)

- name: compozy.execution.resume
  rest: POST /api/v0/executions/{id}/resume
  description: Resume paused execution
  input:
    - execution_id: string (required)

- name: compozy.execution.cancel
  rest: POST /api/v0/executions/{id}/cancel
  description: Cancel running execution
  input:
    - execution_id: string (required)

- name: compozy.execution.stream
  rest: GET /api/v0/executions/{id}/stream (SSE)
  description: Stream execution events in real-time
  input:
    - execution_id: string (required)
```

---

## 8. Dependencies & Prerequisites

### Internal Dependencies
- **REST API Handlers**: Reuse existing workflow/agent/task handlers (`engine/*/router/*`)
- **OpenAPI Specification**: Use existing Swagger docs for tool generation (`docs/swagger.yaml`)
- **Authentication Middleware**: Reuse existing auth middleware (`engine/infra/server/middleware/auth`)
- **Logging & Monitoring**: Use existing logger and Prometheus metrics
- **Configuration System**: Use existing config management (`pkg/config`)

### External Dependencies
- **Go 1.25+**: For language features
- **Gin Framework**: For HTTP routing (already used)
- **JSON-RPC 2.0**: MCP protocol base (no new library needed, implement spec)
- **SSE Library**: For Server-Sent Events streaming (consider `github.com/r3labs/sse/v2`)

### Infrastructure Requirements
- **No new infrastructure**: MCP server runs embedded in main Compozy server
- **Network**: MCP endpoint accessible on same port as REST API (e.g., 3000)
- **Storage**: No additional storage needed (uses existing database for workflows/agents)

---

## 9. Implementation Plan

### Phase 1: Foundation (Week 1-2)
**Goal**: Set up MCP protocol handler and basic tool discovery

**Tasks**:
- [ ] Implement MCP protocol handler (`mcp_protocol.go`)
  - [ ] Parse MCP JSON-RPC requests
  - [ ] Serialize MCP JSON-RPC responses
  - [ ] Handle `initialize` method
  - [ ] Handle `tools/list` method
  - [ ] Implement SSE transport for streaming
- [ ] Create Tool Registry (`mcp_tool_registry.go`)
  - [ ] Load OpenAPI specification (Swagger)
  - [ ] Generate tool definitions from OpenAPI paths
  - [ ] Map tool names to REST endpoints
  - [ ] Implement tool discovery logic
- [ ] Set up MCP server routes (`mcp_server.go`)
  - [ ] Register `/mcp` endpoint group
  - [ ] Integrate with main server initialization
  - [ ] Add health check endpoint
- [ ] Write unit tests for protocol handler and tool registry

**Deliverables**:
- MCP server responds to `initialize` and `tools/list` requests
- Tool registry auto-generates tools from OpenAPI spec
- Unit tests for core components

---

### Phase 2: Tool Execution (Week 3-4)
**Goal**: Implement tool execution and request/response transformation

**Tasks**:
- [ ] Implement Tool Executor (`mcp_tool_executor.go`)
  - [ ] Handle `tools/call` method
  - [ ] Transform MCP requests to REST API format
  - [ ] Route to existing REST API handlers
  - [ ] Transform REST responses to MCP format
  - [ ] Map REST errors to MCP error codes
- [ ] Integrate authentication/authorization
  - [ ] Support API key authentication via MCP headers
  - [ ] Enforce same authorization rules as REST API
  - [ ] Handle auth failures with MCP error responses
- [ ] Add execution monitoring
  - [ ] Prometheus metrics for tool calls
  - [ ] Structured logging for all tool invocations
  - [ ] Error tracking and alerting
- [ ] Write integration tests for tool execution

**Deliverables**:
- MCP tools execute successfully and return results
- Authentication works correctly
- Metrics and logging operational

---

### Phase 3: Streaming & Advanced Features (Week 5-6)
**Goal**: Add streaming support and optimize performance

**Tasks**:
- [ ] Implement SSE streaming for long-running operations
  - [ ] Stream execution status updates
  - [ ] Support multiple concurrent streams
  - [ ] Handle stream disconnections gracefully
- [ ] Add configuration options
  - [ ] Config file schema for MCP server
  - [ ] Environment variable support
  - [ ] Tool filtering and customization
- [ ] Performance optimization
  - [ ] Reduce tool execution latency
  - [ ] Optimize tool discovery caching
  - [ ] Benchmark against performance requirements
- [ ] Error handling improvements
  - [ ] Comprehensive error mapping
  - [ ] User-friendly error messages
  - [ ] Retry logic for transient failures

**Deliverables**:
- Streaming works for long-running executions
- Configuration fully implemented
- Performance targets met (< 50ms overhead)

---

### Phase 4: Testing & Documentation (Week 7-8)
**Goal**: Comprehensive testing and user-facing documentation

**Tasks**:
- [ ] Write comprehensive test suite
  - [ ] Unit tests (80%+ coverage)
  - [ ] Integration tests (all tools)
  - [ ] End-to-end tests (Claude Desktop, VSCode)
  - [ ] Load tests (1000 req/min)
- [ ] Create documentation
  - [ ] MCP server user guide
  - [ ] Integration guides (Claude Desktop, VSCode, Windsurf)
  - [ ] API reference for MCP tools
  - [ ] Configuration reference
  - [ ] Troubleshooting guide
- [ ] Add examples
  - [ ] Sample MCP client configurations
  - [ ] Example workflows for common use cases
  - [ ] Demo videos for AI application integration
- [ ] Security audit
  - [ ] Review authentication/authorization
  - [ ] Input validation and sanitization
  - [ ] Rate limiting configuration
  - [ ] Security best practices documentation

**Deliverables**:
- Test suite passing with 80%+ coverage
- Complete documentation published
- Security audit completed

---

### Phase 5: Beta Release & Feedback (Week 9-10)
**Goal**: Beta release to select users and iterate based on feedback

**Tasks**:
- [ ] Deploy to staging environment
- [ ] Invite beta users (5-10 early adopters)
- [ ] Collect feedback on:
  - [ ] Tool usability and naming
  - [ ] Integration ease with AI applications
  - [ ] Performance and reliability
  - [ ] Documentation clarity
- [ ] Address critical feedback
- [ ] Monitor production metrics
- [ ] Prepare for GA release

**Deliverables**:
- Beta release deployed
- Feedback collected and prioritized
- Critical issues resolved

---

### Phase 6: General Availability (Week 11-12)
**Goal**: Production release and announcement

**Tasks**:
- [ ] Final QA testing
- [ ] Production deployment
- [ ] Announcement and marketing
  - [ ] Blog post on MCP integration
  - [ ] Social media announcements
  - [ ] Email to existing users
- [ ] Monitor production metrics
  - [ ] Track adoption rate
  - [ ] Monitor performance KPIs
  - [ ] Watch for errors/issues
- [ ] Support readiness
  - [ ] FAQ based on beta feedback
  - [ ] Support team training
  - [ ] Monitoring and alerting setup

**Deliverables**:
- MCP server in production
- Announcement published
- Monitoring operational

---

## 10. Testing Strategy

### 10.1 Unit Tests

**Coverage Target**: 80%+

**Test Areas**:
- MCP protocol parsing and serialization
- Tool registry generation from OpenAPI
- Tool name mapping logic
- Request/response transformers
- Error handling and mapping

**Example Test**:
```go
func TestMCPProtocolHandler_ParseRequest(t *testing.T) {
    handler := NewMCPProtocolHandler()

    requestJSON := `{
        "jsonrpc": "2.0",
        "method": "tools/list",
        "params": {},
        "id": 1
    }`

    req, err := handler.ParseRequest([]byte(requestJSON))
    assert.NoError(t, err)
    assert.Equal(t, "tools/list", req.Method)
    assert.Equal(t, 1, req.ID)
}
```

---

### 10.2 Integration Tests

**Test Areas**:
- End-to-end tool execution (MCP request → REST handler → MCP response)
- Authentication flow
- Error scenarios
- Streaming responses

**Example Test**:
```go
func TestMCPServer_ExecuteWorkflow(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Close()

    // Create test workflow
    workflow := createTestWorkflow(t, server)

    // Execute via MCP
    mcpRequest := MCPRequest{
        JSONRPC: "2.0",
        Method:  "tools/call",
        Params: map[string]interface{}{
            "name": "compozy.workflow.execute",
            "arguments": map[string]interface{}{
                "workflow_id": workflow.ID,
                "input": map[string]interface{}{
                    "test_input": "value",
                },
            },
        },
        ID: 1,
    }

    // Send MCP request
    resp := sendMCPRequest(t, server, mcpRequest)

    // Verify response
    assert.NotNil(t, resp.Result)
    assert.Nil(t, resp.Error)

    // Verify execution started
    execID := extractExecutionID(t, resp)
    assert.NotEmpty(t, execID)
}
```

---

### 10.3 End-to-End Tests

**Test with Real MCP Clients**:
- Claude Desktop integration test
- VSCode Copilot integration test
- Programmatic MCP client test

**Test Scenarios**:
1. User installs Compozy MCP server in Claude Desktop
2. User asks Claude to "list available workflows"
3. User asks Claude to "execute the data-processing workflow"
4. User monitors execution status through Claude

---

### 10.4 Performance Tests

**Load Testing**:
- Simulate 100 concurrent tool discovery requests
- Simulate 1000 tool calls per minute
- Measure latency overhead vs direct REST API

**Benchmarks**:
```bash
# Tool discovery benchmark
go test -bench=BenchmarkToolDiscovery -benchmem

# Tool execution benchmark
go test -bench=BenchmarkToolExecution -benchmem
```

**Performance Targets**:
- Tool discovery: < 100ms (p99)
- Tool execution overhead: < 50ms (p99)
- Throughput: > 1000 requests/minute per instance

---

### 10.5 Security Tests

**Test Areas**:
- Authentication bypass attempts
- Authorization violations
- Input injection attacks (SQL, command injection)
- Rate limiting enforcement
- CORS configuration

**Security Test Cases**:
- Attempt tool execution without API key → expect 401
- Attempt admin-only tool without admin role → expect 403
- Send malformed MCP requests → expect proper error handling
- Exceed rate limits → expect 429

---

## 11. Documentation Plan

### 11.1 User Documentation

#### MCP Server Quick Start Guide
**Audience**: Developers integrating Compozy with AI applications
**Content**:
- What is MCP and why use it
- How to enable MCP server in Compozy
- Basic configuration options
- First tool call example
- Troubleshooting common issues

#### Integration Guides

**Claude Desktop Integration**
- Prerequisites and setup
- Configuring Claude Desktop to connect to Compozy MCP server
- Example workflows: executing workflows, monitoring status
- Screenshot walkthrough

**VSCode Copilot Integration**
- Installing MCP extension for VSCode
- Configuring Compozy MCP server connection
- Using Compozy tools in Copilot chat
- Example use cases

**Windsurf Integration**
- Windsurf MCP configuration
- Chaining Compozy tools with other MCP tools
- Advanced automation patterns

---

### 11.2 API Documentation

#### MCP Tools Reference
**Content**:
- Complete list of all MCP tools
- Tool name, description, input schema for each
- Example requests and responses
- Error codes and troubleshooting

**Example Entry**:
```markdown
## compozy.workflow.execute

**Description**: Execute a workflow by ID

**Input Schema**:
```json
{
  "workflow_id": "string (required)",
  "input": "object (optional)",
  "sync": "boolean (optional, default: false)"
}
```

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "compozy.workflow.execute",
    "arguments": {
      "workflow_id": "wf-data-processing",
      "input": {"date": "2025-10-31"}
    }
  },
  "id": 1
}
```

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Execution started: exec-abc123"
      }
    ]
  },
  "id": 1
}
```
```

---

### 11.3 Configuration Reference

**Complete configuration options**:
```yaml
server:
  mcp:
    enabled: true
    path: "/mcp"
    transport: "sse"
    protocol_version: "2025-03-26"
    tool_prefix: "compozy"
    exclude_endpoints: []
    max_concurrent_requests: 100
    request_timeout: 30s
    sse_heartbeat_interval: 15s
    sse_retry_interval: 5s
```

**Environment variable mapping**:
```
COMPOZY_SERVER_MCP_ENABLED        → server.mcp.enabled
COMPOZY_SERVER_MCP_PATH           → server.mcp.path
COMPOZY_SERVER_MCP_PROTOCOL       → server.mcp.protocol_version
```

---

### 11.4 Examples & Tutorials

**Example 1: Daily Report Automation**
```
Goal: Use Claude Desktop to generate daily reports via Compozy workflows

Steps:
1. Configure Claude Desktop with Compozy MCP server
2. User says: "Generate my daily sales report"
3. Claude discovers compozy.workflow.execute tool
4. Claude calls tool with workflow_id="daily-sales-report"
5. Report generated and presented to user
```

**Example 2: Code Review Agent**
```
Goal: Use VSCode Copilot to run code review agent on current file

Steps:
1. Configure VSCode with Compozy MCP server
2. User selects code and asks: "Review this code for security issues"
3. Copilot discovers compozy.agent.execute tool
4. Copilot invokes security-review agent with code context
5. Agent findings displayed inline in VSCode
```

---

## 12. Risks & Mitigations

### Risk 1: Performance Degradation
**Risk**: MCP server adds significant latency to REST API requests
**Impact**: P0 - Core functionality affected
**Probability**: Medium

**Mitigation**:
- Design MCP server to reuse REST handlers (no duplication)
- Implement request pooling and caching
- Benchmark continuously during development
- Add performance tests to CI/CD pipeline
- **Success Criterion**: Maintain < 50ms overhead target

---

### Risk 2: Tool Schema Mismatch
**Risk**: Auto-generated MCP tools don't match REST API contracts
**Impact**: P1 - Poor user experience, confusing errors
**Probability**: Medium

**Mitigation**:
- Use OpenAPI as single source of truth
- Add automated schema validation tests
- Implement strict schema generation rules
- Version tool schemas with protocol version
- **Success Criterion**: 100% schema accuracy verified by tests

---

### Risk 3: Security Vulnerabilities
**Risk**: MCP server introduces new attack vectors
**Impact**: P0 - Security breach potential
**Probability**: Low

**Mitigation**:
- Reuse existing authentication/authorization
- Implement input validation and sanitization
- Conduct security audit before GA release
- Add rate limiting and abuse protection
- Monitor for unusual MCP request patterns
- **Success Criterion**: Pass security audit, zero critical vulnerabilities

---

### Risk 4: Limited MCP Client Adoption
**Risk**: Users don't use MCP server (low adoption)
**Impact**: P2 - Feature underutilized
**Probability**: Low

**Mitigation**:
- Create compelling integration guides for popular AI tools
- Provide demo videos and examples
- Reach out to beta users for feedback
- Market MCP integration as key differentiator
- **Success Criterion**: Achieve 20% user adoption within 3 months

---

### Risk 5: OpenAPI Spec Incomplete/Inaccurate
**Risk**: Swagger docs don't cover all endpoints or are outdated
**Impact**: P1 - Missing tools or incorrect tool schemas
**Probability**: Medium

**Mitigation**:
- Audit OpenAPI spec for completeness before starting
- Update OpenAPI spec as part of development
- Add automated OpenAPI validation to CI/CD
- Document manual tool overrides if needed
- **Success Criterion**: OpenAPI spec 100% accurate for all v0 endpoints

---

## 13. Open Questions

### Question 1: Tool Naming Convention
**Question**: Should tool names be `compozy.workflow.execute` or `execute_workflow`?
**Options**:
- A) `compozy.{resource}.{action}` (hierarchical, namespace-safe)
- B) `{action}_{resource}` (flat, shorter)

**Recommendation**: **Option A** (hierarchical)
**Rationale**: Prevents naming conflicts, clearer organization, matches REST API structure

**Decision**: TBD (Product Team)

---

### Question 2: Synchronous vs Asynchronous Execution
**Question**: Should all tools execute asynchronously by default?
**Options**:
- A) Async by default (return execution ID immediately)
- B) Sync by default (wait for completion)
- C) Configurable via `sync` parameter

**Recommendation**: **Option C** (configurable)
**Rationale**: Matches REST API behavior, gives users flexibility

**Decision**: TBD (Product Team)

---

### Question 3: Streaming Support Priority
**Question**: Is streaming support critical for MVP or can it be added later?
**Options**:
- A) MVP (Phase 1-3)
- B) Post-MVP (Phase 4+)

**Recommendation**: **Option B** (Post-MVP)
**Rationale**: Async execution + status polling covers most use cases, streaming can be added after GA

**Decision**: TBD (Product Team)

---

### Question 4: Admin Tool Exposure
**Question**: Should admin-only endpoints (e.g., `/api/v0/admin/*`) be exposed as MCP tools?
**Options**:
- A) Yes, with admin role check
- B) No, exclude from MCP entirely
- C) Configurable via exclude_endpoints

**Recommendation**: **Option C** (configurable)
**Rationale**: Gives operators flexibility, defaults to secure (excluded)

**Decision**: TBD (Engineering Team)

---

## 14. Approval & Sign-off

### Stakeholder Approval

| Role | Name | Status | Date |
|------|------|--------|------|
| Product Manager | TBD | ⏳ Pending | - |
| Engineering Lead | TBD | ⏳ Pending | - |
| Architecture Team | TBD | ⏳ Pending | - |
| Security Team | TBD | ⏳ Pending | - |
| DevOps Team | TBD | ⏳ Pending | - |

### Sign-off Criteria
- [ ] All functional requirements reviewed and approved
- [ ] Technical architecture validated by engineering
- [ ] Security review completed
- [ ] Performance targets agreed upon
- [ ] Success metrics defined and accepted
- [ ] Implementation timeline approved

---

## 15. Appendix

### A. MCP Protocol Specification Reference
- **Official Spec**: https://spec.modelcontextprotocol.io/specification/2025-03-26/
- **JSON-RPC 2.0**: https://www.jsonrpc.org/specification
- **MCP GitHub**: https://github.com/modelcontextprotocol

### B. Competitor Analysis
- **LangChain MCP Integration**: Offers MCP tools via LangChain ecosystem
- **Semantic Kernel**: Microsoft's AI orchestration with MCP support
- **AgentOps**: Monitoring platform with MCP integration

**Compozy Differentiator**: Native MCP server with automatic REST → MCP mapping, zero configuration

### C. Related PRDs
- REST API v0 Specification (existing)
- Workflow Execution Engine (existing)
- Agent Framework (existing)
- Future: MCP Custom Tools (user-defined MCP tools beyond REST API)

### D. Glossary

| Term | Definition |
|------|------------|
| **MCP** | Model Context Protocol - standard for AI tool integration |
| **SSE** | Server-Sent Events - HTTP streaming protocol |
| **JSON-RPC** | Remote procedure call protocol using JSON |
| **Tool** | MCP-exposed function that AI can invoke |
| **Resource** | MCP concept for data sources (optional for this PRD) |
| **Prompt** | MCP concept for reusable prompts (optional for this PRD) |

---

## Document Revision History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2025-10-31 | Product Team | Initial PRD creation |

---

**End of PRD**
