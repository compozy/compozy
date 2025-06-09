# Compozy API Documentation

This directory contains the OpenAPI/Swagger documentation for the Compozy API.

## Overview

Compozy is a workflow orchestration engine for AI agents, tasks, and tools. The API provides
comprehensive endpoints for managing and monitoring workflow executions.

## Documentation Files

- `swagger.json` - OpenAPI 3.0 specification in JSON format
- `swagger.yaml` - OpenAPI 3.0 specification in YAML format
- `docs.go` - Generated Go documentation file

## API Endpoints

### Core Resources

#### Workflows

- `GET /api/v0/workflows` - List all workflows
- `GET /api/v0/workflows/{workflow_id}` - Get workflow by ID
- `POST /api/v0/workflows/{workflow_id}/executions` - Execute workflow
- `GET /api/v0/workflows/{workflow_id}/executions` - List workflow executions
- `GET /api/v0/workflows/{workflow_id}/executions/children` - List child executions by workflow ID
- `GET /api/v0/workflows/{workflow_id}/executions/children/tasks` - List task executions by workflow
  ID
- `GET /api/v0/workflows/{workflow_id}/executions/children/agents` - List agent executions by
  workflow ID
- `GET /api/v0/workflows/{workflow_id}/executions/children/tools` - List tool executions by workflow
  ID

#### Tasks

- `GET /api/v0/workflows/{workflow_id}/tasks` - List tasks for a workflow
- `GET /api/v0/workflows/{workflow_id}/tasks/{task_id}` - Get task by ID
- `GET /api/v0/workflows/{workflow_id}/tasks/{task_id}/executions` - List task executions
- `GET /api/v0/workflows/{workflow_id}/tasks/{task_id}/executions/children` - List child executions
  by task ID
- `GET /api/v0/workflows/{workflow_id}/tasks/{task_id}/executions/agents` - List agent executions by
  task ID
- `GET /api/v0/workflows/{workflow_id}/tasks/{task_id}/executions/tools` - List tool executions by
  task ID

#### Agents

- `GET /api/v0/agents` - List all agents
- `GET /api/v0/agents/{agent_id}` - Get agent by ID
- `GET /api/v0/agents/{agent_id}/executions` - List agent executions

#### Tools

- `GET /api/v0/tools` - List all tools
- `GET /api/v0/tools/{tool_id}` - Get tool by ID
- `GET /api/v0/tools/{tool_id}/executions` - List tool executions

#### Executions

- `GET /api/v0/executions/workflows` - List all workflow executions
- `GET /api/v0/executions/workflows/{workflow_exec_id}` - Get workflow execution by ID
- `GET /api/v0/executions/workflows/{workflow_exec_id}/executions` - List child executions by
  workflow execution ID
- `GET /api/v0/executions/workflows/{workflow_exec_id}/executions/tasks` - List task executions by
  workflow execution ID
- `GET /api/v0/executions/workflows/{workflow_exec_id}/executions/agents` - List agent executions by
  workflow execution ID
- `GET /api/v0/executions/workflows/{workflow_exec_id}/executions/tools` - List tool executions by
  workflow execution ID
- `GET /api/v0/executions/tasks` - List all task executions
- `GET /api/v0/executions/tasks/{task_exec_id}` - Get task execution by ID
- `GET /api/v0/executions/tasks/{task_exec_id}/executions` - List child executions by task execution
  ID
- `GET /api/v0/executions/tasks/{task_exec_id}/executions/agents` - List agent executions by task
  execution ID
- `GET /api/v0/executions/tasks/{task_exec_id}/executions/tools` - List tool executions by task
  execution ID
- `GET /api/v0/executions/agents` - List all agent executions
- `GET /api/v0/executions/agents/{agent_exec_id}` - Get agent execution by ID
- `GET /api/v0/executions/tools` - List all tool executions
- `GET /api/v0/executions/tools/{tool_exec_id}` - Get tool execution by ID

### System Endpoints

- `GET /health` - Health check endpoint

## Accessing the Documentation

### Swagger UI

When the Compozy server is running, you can access the interactive Swagger UI at:

- `/swagger/index.html` - Main Swagger UI interface
- `/docs/index.html` - Alternative Swagger UI interface

The Swagger UI automatically adapts to the port you're running the server on. For example:

- Default dev server: `http://localhost:3001/swagger/index.html`
- Custom port: `http://localhost:8080/swagger/index.html` (if running with `--port 8080`)
- Production: `https://your-domain.com/swagger/index.html`

You can also access the root endpoint `/` to see all available endpoints with the correct URLs for
your current server configuration.

### Development

To regenerate the documentation after making changes to API annotations:

```bash
make swagger-gen
```

To validate the documentation:

```bash
make swagger-validate
```

To start the server with Swagger UI:

```bash
make swagger-serve
```

## Response Format

All API responses follow a consistent format:

```json
{
    "status": 200,
    "message": "Success message",
    "data": {
        // Response data
    },
    "error": {
        "code": "ERROR_CODE",
        "message": "Error message",
        "details": "Additional error details"
    }
}
```

## Authentication

Currently, the API does not require authentication. This may change in future versions.

## Rate Limiting

No rate limiting is currently implemented.

## Error Codes

Common HTTP status codes used by the API:

- `200` - Success
- `201` - Created
- `202` - Accepted (for async operations)
- `400` - Bad Request
- `404` - Not Found
- `409` - Conflict
- `500` - Internal Server Error

## Examples

The following examples use the default development server port (3001). Replace with your actual
server port if different.

### Execute a Workflow

```bash
# Default dev server (port 3001)
curl -X POST "http://localhost:3001/api/v0/workflows/data-processing/executions" \
    -H "Content-Type: application/json" \
    -d '{
    "input": {
      "data": "example input data",
      "config": {
        "timeout": 300
      }
    }
  }'

# Custom port example
curl -X POST "http://localhost:8080/api/v0/workflows/data-processing/executions" \
    -H "Content-Type: application/json" \
    -d '{
    "input": {
      "data": "example input data"
    }
  }'
```

### Get Workflow Status

```bash
curl "http://localhost:3001/api/v0/workflows/data-processing"
```

### List All Agents

```bash
curl "http://localhost:3001/api/v0/agents"
```

### Get Server Information (Dynamic URLs)

```bash
# This endpoint returns the correct URLs for your current server configuration
curl "http://localhost:3001/"
```

## Development Guidelines

When adding new endpoints:

1. Add comprehensive Swagger annotations to handler functions
2. Include proper parameter validation
3. Document all possible response codes
4. Provide realistic examples
5. Regenerate documentation with `make swagger-gen`

### Swagger Annotation Format

```go
// handlerFunction handles the endpoint
// @Summary Brief description
// @Description Detailed description
// @Tags tag-name
// @Accept json
// @Produce json
// @Param param_name path string true "Parameter description" example("example-value")
// @Success 200 {object} ResponseType "Success description"
// @Failure 400 {object} router.Response{error=router.ErrorInfo} "Error description"
// @Router /endpoint/path [method]
func handlerFunction(c *gin.Context) {
    // Implementation
}
```

## Support

For API support and questions:

- GitHub Issues: https://github.com/compozy/compozy/issues
- Documentation: https://github.com/compozy/compozy/docs
