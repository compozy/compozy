## status: pending

<task_context>
<domain>sdk/examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>sdk/mcp</dependencies>
</task_context>

# Task 48.0: Example: MCP Integration (S)

## Overview

Create comprehensive MCP example demonstrating both remote (URL-based with SSE) and local (command-based with stdio) MCP server configurations.

<critical>
- **ALWAYS READ** @.cursor/rules/go-coding-standards.mdc before start
- **ALWAYS READ** tasks/prd-sdk/05-examples.md (Example 5: MCP Integration)
- **MUST** demonstrate both URL-based and command-based MCP
- **MUST** show transport types, headers, env vars, sessions
</critical>

<requirements>
- Runnable example: sdk/examples/05_mcp_integration.go
- Demonstrates: Remote MCP (SSE), Local MCP (stdio), Docker-based MCP
- Shows: Headers, auth, transport types, env vars, timeouts, sessions
- Agent integration with MCP access
- Clear comments on MCP patterns
</requirements>

## Subtasks

- [ ] 48.1 Create sdk/examples/05_mcp_integration.go
- [ ] 48.2 Build remote MCP with SSE transport (GitHub API example):
  - [ ] URL configuration
  - [ ] SSE transport
  - [ ] Authorization header
  - [ ] Protocol version
  - [ ] Max sessions
- [ ] 48.3 Build local MCP with stdio (filesystem example):
  - [ ] Command configuration
  - [ ] Environment variables
  - [ ] Start timeout
- [ ] 48.4 Build Docker-based MCP (database example):
  - [ ] Docker run command
  - [ ] Environment variables
  - [ ] Start timeout
- [ ] 48.5 Create agent with MCP access
- [ ] 48.6 Build project with all MCP configs
- [ ] 48.7 Add comments explaining MCP patterns
- [ ] 48.8 Update README.md with MCP example
- [ ] 48.9 Test example runs successfully

## Implementation Details

Per 05-examples.md section 5:

**Remote MCP with SSE:**
```go
githubMCP, err := mcp.New("github-api").
    WithURL("https://api.github.com/mcp/v1").
    WithTransport(mcpproxy.TransportSSE).
    WithHeader("Authorization", "Bearer {{.env.GITHUB_TOKEN}}").
    WithProto("2025-03-26").
    WithMaxSessions(10).
    Build(ctx)
```

**Local MCP with stdio:**
```go
filesystemMCP, err := mcp.New("filesystem").
    WithCommand("mcp-server-filesystem").
    WithEnvVar("ROOT_DIR", "/data").
    WithStartTimeout(10 * time.Second).
    Build(ctx)
```

**Docker-based MCP:**
```go
dockerMCP, err := mcp.New("postgres-db").
    WithCommand("docker", "run", "--rm", "-i", "mcp-postgres:latest").
    WithEnvVar("DATABASE_URL", "postgres://user:pass@db/myapp").
    WithStartTimeout(30 * time.Second).
    Build(ctx)
```

**Agent with MCP:**
```go
devAgent, err := agent.New("developer-assistant").
    AddMCP("github-api").
    AddMCP("filesystem").
    Build(ctx)
```

### Relevant Files

- `sdk/examples/05_mcp_integration.go` - Main example
- `sdk/examples/README.md` - Updated instructions

### Dependent Files

- `sdk/mcp/builder.go` - MCP builder
- `sdk/agent/builder.go` - Agent with MCP
- `pkg/mcp-proxy/transport.go` - Transport types

## Deliverables

- [ ] sdk/examples/05_mcp_integration.go (runnable)
- [ ] Updated README.md with MCP example section
- [ ] Comments explaining:
  - Remote vs local MCP patterns
  - Transport types (SSE vs stdio)
  - Authentication with headers
  - Environment variable injection
  - Session management
- [ ] All 3 MCP types demonstrated
- [ ] Verified example runs successfully

## Tests

From _tests.md:

- Example validation:
  - [ ] Code compiles without errors
  - [ ] Remote MCP config with SSE transport
  - [ ] Local MCP config with stdio
  - [ ] Docker MCP config with command args
  - [ ] Headers validated (key-value pairs)
  - [ ] Env vars propagated correctly
  - [ ] Timeouts validated (positive durations)
  - [ ] Agent MCP integration works

## Success Criteria

- Example demonstrates all MCP configuration patterns
- Both remote and local MCPs shown
- Docker-based MCP example included
- Comments explain when to use each pattern
- README updated with MCP setup requirements
- Example runs end-to-end successfully
- Code passes `make lint`
