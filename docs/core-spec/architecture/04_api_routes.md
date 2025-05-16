# API Routes

This document outlines the API routes for the Compozy Workflow Engine.

## System Routes

- `GET /api` - Get API information
- `GET /api/health` - Get health information
- `GET /api/metrics` - Get metrics information
- `GET /api/version` - Get version information
- `GET /openapi.json` - Get OpenAPI schema
- `GET /swagger.ui` - Get Swagger UI

## Workflow Routes

- `GET /api/workflows` - List all workflows
- `GET /api/workflows/:workflow_id/definition` - Get workflow definition
- `GET /api/workflows/:workflow_id/executions` - List all executions for a workflow
- `POST /api/workflows/:workflow_id/execute` - Execute a workflow
- `POST /api/workflows/:workflow_id/execute-async` - Execute a workflow asynchronously

#### Execution Routes

- `GET /api/workflows/executions` - List all executions
- `GET /api/workflows/executions/:workflow_exec_id` - Get information about an execution
- `GET /api/workflows/executions/:workflow_exec_id/status` - Get status of an execution
- `POST /api/workflows/executions/:workflow_exec_id/resume` - Resume an execution
- `POST /api/workflows/executions/:workflow_exec_id/cancel` - Cancel an execution

## Task Routes

- `GET /api/tasks` - List all tasks
- `GET /api/tasks/:task_id/definition` - Get task definition
- `GET /api/tasks/:task_id/executions` - List all executions for a task
- `POST /api/tasks/:task_id/trigger` - Trigger a task
- `POST /api/tasks/:task_id/trigger-async` - Trigger a task asynchronously

#### Execution Routes

- `GET /api/tasks/executions` - List all executions
- `GET /api/tasks/executions/:task_exec_id` - Get information about an execution
- `GET /api/tasks/executions/:task_exec_id/status` - Get status of an execution
- `POST /api/tasks/executions/:task_exec_id/resume` - Resume an execution

## Agent Routes

- `GET /api/agents` - List all agents
- `GET /api/agents/:agent_id/definition` - Get agent definition
- `GET /api/agents/:agent_id/executions` - List all executions for an agent

#### Execution Routes

- `GET /api/agents/executions` - List all executions
- `GET /api/agents/executions/:agent_exec_id` - Get information about an execution
- `GET /api/agents/executions/:agent_exec_id/status` - Get status of an execution

## Tool Routes

- `GET /api/tools` - List all tools
- `GET /api/tools/:tool_id/definition` - Get tool definition
- `GET /api/tools/:tool_id/executions` - List all executions for a tool

#### Execution Routes

- `GET /api/tools/executions` - List all executions
- `GET /api/tools/executions/:tool_exec_id` - Get information about an execution
- `GET /api/tools/executions/:tool_exec_id/status` - Get status of an execution

## Log Routes

- `GET /api/logs` - List all logs
- `GET /api/logs/:log_id` - Get a log by ID
- `GET /api/logs/workflows/:workflow_exec_id` - Get logs for a workflow execution