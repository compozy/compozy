# Compozy Workflows

Compozy workflows orchestrate AI agents, tasks, and tools to build sophisticated AI-powered applications through declarative YAML configuration.

## Overview

A workflow in Compozy consists of:

- **Schemas**: Define input/output structures for type safety
- **Tools**: Local executable tools for specific operations
- **Agents**: AI agents with instructions and capabilities
- **MCPs**: External Model Context Protocol servers for additional tools
- **Tasks**: Orchestrated execution units that combine agents and tools

## Basic Workflow Structure

```yaml
id: my-workflow
version: "1.0.0"
description: "A sample workflow"

config:
    input:
        $ref: local::schemas.#(id=="workflow_input")
    env:
        WORKFLOW_VERSION: "1.0.0"

schemas:
    - id: workflow_input
      type: object
      properties:
          query:
              type: string
              description: User query
      required:
          - query

tools:
    - id: my-tool
      description: "A local tool"
      execute: "./tool.ts"
      input:
          $ref: local::schemas.#(id=="workflow_input")

agents:
    - id: my-agent
      config:
          $ref: global::models.#(id=="gpt-4o")
      instructions: "You are a helpful assistant"
      tools:
          - $ref: local::tools.#(id="my-tool")

tasks:
    - id: main-task
      type: basic
      $use: agent(local::agents.#(id=="my-agent"))
      action: default
      final: true
```

## Connecting External MCP Servers

Model Context Protocol (MCP) enables workflows to connect to external servers that provide additional tools and capabilities. Compozy supports HTTP-based MCP transports for connecting to remote servers.

### MCP Configuration

Add MCP servers to your workflow using the `mcps` array:

```yaml
id: mcp-enabled-workflow
version: "1.0.0"
description: "Workflow with external MCP servers"

mcps:
    # Remote MCP server
    - id: remote-mcp-server
      url: https://api.example.com/mcp
      env:
          API_KEY: "{{ .env.MCP_API_KEY }}"
      proto: "2025-03-26"
      start_timeout: 15s

agents:
    - id: mcp-agent
      config:
          $ref: global::models.#(id=="gpt-4o")
      instructions: |
          You are an assistant with access to external tools via MCP.
          Use the available tools to help users with their requests.

tasks:
    - id: mcp-task
      type: basic
      $use: agent(local::agents.#(id=="mcp-agent"))
      action: default
      final: true
```

### MCP Configuration Options

#### Configuration Options

- **`id`**: Unique identifier for the MCP server (required)
- **`url`**: HTTP URL of the MCP server (required)
- **`env`**: Environment variables for authentication and configuration
- **`proto`**: MCP protocol version (default: "2025-03-26")
- **`start_timeout`**: Timeout for server startup (e.g., "30s", "1m")
- **`max_sessions`**: Maximum concurrent sessions (optional)

#### HTTP Transport

Compozy connects to remote MCP servers via HTTP/HTTPS:

```yaml
mcps:
    - id: api-server
      url: https://api.example.com/mcp
      env:
          API_KEY: "{{ .env.API_SECRET }}"
          CLIENT_ID: "compozy-workflow"
```

### Environment Variable Interpolation

MCP configurations support the same environment variable interpolation as other workflow components:

```yaml
mcps:
    - id: secure-server
      url: "{{ .env.MCP_SERVER_URL }}"
      env:
          API_KEY: "{{ .env.MCP_API_KEY }}"
          CLIENT_SECRET: "{{ .secrets.MCP_CLIENT_SECRET }}"
```

### Tool Discovery and Integration

When a workflow executes, Compozy automatically:

1. **Connects** to all configured MCP servers
2. **Discovers** available tools from each server
3. **Integrates** MCP tools with local tools
4. **Provides** the combined tool set to agents

MCP tools appear alongside local tools in agent capabilities and can be called naturally through the LLM interface.

### Error Handling

If an MCP server fails to start or connect:

- The workflow continues with available tools
- Warnings are logged for failed connections
- Local tools and other MCP servers remain functional

### Security Considerations

- **HTTP transport**: All communications are over HTTP/HTTPS
- **Authentication**: Use environment variables for API keys and tokens
- **Environment isolation**: Each MCP server has its own environment scope
- **Timeout protection**: Startup timeouts prevent hanging workflows
- **Secret management**: Use environment interpolation for sensitive data

### Best Practices

1. **Use descriptive IDs**: Name MCP servers clearly (e.g., "weather-api", "file-tools")
2. **Set appropriate timeouts**: Balance startup time vs. workflow responsiveness
3. **Environment separation**: Keep MCP-specific config in environment variables
4. **Error tolerance**: Design workflows to handle MCP server failures gracefully
5. **Resource limits**: Use `max_sessions` to prevent resource exhaustion

### Example: File Processing Workflow

```yaml
id: file-processor
version: "1.0.0"
description: "Process files using MCP tools"

config:
    input:
        type: object
        properties:
            file_path:
                type: string
                description: Path to file to process

mcps:
    - id: file-tools
      url: http://localhost:5000/mcp
      env:
          ALLOWED_PATHS: "{{ .env.SAFE_FILE_PATHS }}"
          API_KEY: "{{ .env.FILE_SERVER_API_KEY }}"

    - id: text-analysis
      url: "{{ .env.TEXT_ANALYSIS_URL }}"
      env:
          API_KEY: "{{ .env.TEXT_API_KEY }}"

agents:
    - id: file-processor-agent
      config:
          $ref: global::models.#(id=="gpt-4o")
      instructions: |
          You are a file processing assistant. You can:
          1. Read and analyze file contents
          2. Extract information and metadata
          3. Perform text analysis and summarization

          Always verify file access before processing.

tasks:
    - id: process-file
      type: basic
      $use: agent(local::agents.#(id=="file-processor-agent"))
      action: default
      with:
          file_path: "{{ .trigger.input.file_path }}"
      final: true
```

This workflow demonstrates how MCP servers enable workflows to access specialized capabilities while maintaining security and modularity.
