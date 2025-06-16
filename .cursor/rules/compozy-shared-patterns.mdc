---
description: "Shared patterns for MCP integrations, references, and templates in Compozy"
globs:
  - "**/compozy.yaml"
  - "**/workflow.yaml"
  - "**/workflows/*.yaml"
  - "**/agents/*.yaml"
  - "**/tools/*.yaml"
alwaysApply: false
---
# Compozy Shared Patterns

<configuration_overview type="shared_patterns">
Shared patterns include MCP integrations, reference syntax, and template expressions used across all Compozy configuration files.
</configuration_overview>

## MCP Integration Patterns

<mcp_patterns type="filesystem_mcp">
```yaml
mcps:
  - id: file_server
    transport: stdio
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-filesystem"
      - "./"
    proto: "2025-03-26"
    start_timeout: 15s
```
</mcp_patterns>

<mcp_patterns type="http_mcp">
```yaml
mcps:
  - id: search-mcp
    url: http://localhost:3000
    transport: sse
    env:
      API_KEY: "{{ .env.SEARCH_API_KEY }}"
```
</mcp_patterns>

## Reference Patterns

<reference_patterns type="local_references">
```yaml
# Local references within the same file
$ref: local::schemas.#(id=="city_input")
$ref: local::tools.#(id=="weather_tool")
$ref: local::agents.#(id=="tourist_guide")
$ref: local::tasks.#(id=="save_results")

# Global references to project-level resources
$ref: global::models.#(provider=="groq")

# Resource references to external files
$ref: resource::agent::#(id=='file_reader')
```
</reference_patterns>

<reference_patterns type="use_patterns">
```yaml
# Using tools
$use: tool(local::tools.#(id=="weather_tool"))

# Using agents
$use: agent(local::agents.#(id=="tourist_guide"))
$use: agent(resource::agent::#(id=='file_reader'))
```
</reference_patterns>

## Template Expression Patterns

<template_patterns type="common_expressions">
```yaml
# Workflow input access
"{{ .workflow.input.city }}"

# Task output access
"{{ .tasks.weather.output.temperature }}"
"{{ .tasks.activities.output | toJson }}"

# Collection item access
"{{ .item }}"
"{{ .index }}"

# Conditional expressions
'{{ .tasks.clothing.output.clothing | empty | ternary "no_clothes" "has_clothes" }}'

# Current timestamp
"{{ now }}"

# Array length
"{{ len .output }}"
```
</template_patterns>
