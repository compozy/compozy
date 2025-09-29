# `agent` – _AI Agent Configuration and Management_

> **Provides configuration structures and validation for AI agents that power workflow automation in the Compozy orchestration engine.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Configuration Format](#configuration-format)
  - [Action System](#action-system)
  - [Memory Integration](#memory-integration)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `agent` package provides the core configuration structures and validation logic for AI agents within the Compozy workflow orchestration engine. Agents are autonomous AI-powered entities that can reason, make decisions, and execute actions based on natural language instructions.

This package handles:

- Agent configuration parsing and validation
- Action definition and management
- Memory reference configuration
- Provider integration (OpenAI, Anthropic, etc.)
- Tool and MCP (Model Context Protocol) integration

---

## 💡 Motivation

- **Structured AI Configuration**: Provide type-safe configuration for AI agents with comprehensive validation
- **Action-Oriented Design**: Enable agents to perform structured actions with defined input/output schemas
- **Memory Management**: Support persistent context across workflow steps and sessions
- **Provider Abstraction**: Abstract away differences between AI providers while maintaining flexibility
- **Extensibility**: Support for tools and MCP servers to extend agent capabilities

---

## ⚡ Design Highlights

- **Type-Safe Configuration**: All agent configurations are validated using JSON Schema with comprehensive error reporting
- **Action System**: Structured actions with input/output validation and native structured outputs (schema-driven)
- **Memory Integration**: Built-in support for memory references with access control
- **Provider Agnostic**: Works with multiple LLM providers through a unified interface
- **Iterative Execution**: Support for multi-iteration agent responses with self-correction
- **Tool Integration**: Seamless integration with external tools and MCP servers

---

## 🚀 Getting Started

```go
import (
    "context"
    "github.com/compozy/compozy/engine/agent"
    "github.com/compozy/compozy/engine/core"
)

// Load an agent configuration
cwd, _ := core.CWDFromPath("/path/to/project")
agentConfig, err := agent.Load(cwd, "agents/code-assistant.yaml")
if err != nil {
    log.Fatal(err)
}

// Validate the configuration
if err := agentConfig.Validate(); err != nil {
    log.Fatal(err)
}

// Use the agent configuration
fmt.Printf("Agent ID: %s\n", agentConfig.ID)
fmt.Printf("Max Iterations: %d\n", agentConfig.GetMaxIterations())
```

---

## 📖 Usage

### Library

#### Basic Agent Configuration

```go
// Create a basic agent configuration
config := &agent.Config{
    ID: "code-assistant",
    Model: agent.Model{Config: core.ProviderConfig{
        Provider: core.ProviderAnthropic,
        Model:    "claude-4-opus",
        Params:   core.PromptParams{Temperature: 0.7, MaxTokens: 4000},
    }},
    Instructions: "You are an expert software engineer...",
    MaxIterations: 10,
}

// Validate the configuration
if err := config.Validate(); err != nil {
    log.Fatal(err)
}
```

### Configuration Format

#### Basic Agent YAML Configuration

```yaml
resource: "agent"
id: "code-assistant"

model:
  provider: "anthropic"
  model: "claude-4-opus"
  params:
    temperature: 0.7
    max_tokens: 4000

instructions: |
  You are an expert software engineer specializing in code review.
  Focus on clarity, performance, and best practices.
  Always explain your reasoning and provide actionable feedback.

max_iterations: 10
```

### Action System

#### Defining Agent Actions

```go
// Define an action with input/output schemas
action := &agent.ActionConfig{
    ID: "review-code",
    Prompt: "Analyze code for quality and improvements",
    InputSchema: &schema.Schema{
        Type: "object",
        Properties: map[string]*schema.Schema{
            "code": {
                Type: "string",
                Description: "The code to review",
            },
            "language": {
                Type: "string",
                Enum: []any{"python", "go", "javascript"},
            },
        },
        Required: []string{"code", "language"},
    },
    OutputSchema: &schema.Schema{
        Type: "object",
        Properties: map[string]*schema.Schema{
            "quality": {
                Type: "string",
                Description: "Overall quality assessment",
            },
            "issues": {
                Type: "array",
                Items: &schema.Schema{
                    Type: "object",
                    Properties: map[string]*schema.Schema{
                        "severity": {
                            Type: "string",
                            Enum: []any{"critical", "high", "medium", "low"},
                        },
                        "description": {
                            Type: "string",
                        },
                    },
                },
            },
        },
    },
}

// Find an action by ID
foundAction, err := agent.FindActionConfig(config.Actions, "review-code")
if err != nil {
    log.Fatal(err)
}
```

### Memory Integration

#### Memory Reference Configuration

```yaml
memory:
  - id: "conversation_history"
    key: "session:{{.workflow.session_id}}"
    mode: "read-write"
  - id: "user_context"
    key: "user:{{.user_id}}"
    mode: "read-only"
```

```go
// Validate memory configuration
if err := config.NormalizeAndValidateMemoryConfig(); err != nil {
    log.Fatal(err)
}
```

---

## 🔧 Configuration

### Agent Configuration Fields

| Field            | Type              | Required | Description                               |
| ---------------- | ----------------- | -------- | ----------------------------------------- |
| `resource`       | string            | No       | Must be "agent" for autoloader            |
| `id`             | string            | Yes      | Unique identifier for the agent           |
| `config`         | ProviderConfig    | Yes      | LLM provider configuration                |
| `instructions`   | string            | Yes      | System instructions for the agent         |
| `actions`        | []ActionConfig    | No       | Structured actions the agent can perform  |
| `with`           | Input             | No       | Default input parameters                  |
| `env`            | EnvMap            | No       | Environment variables                     |
| `tools`          | []tool.Config     | No       | Available tools                           |
| `mcps`           | []mcp.Config      | No       | MCP server configurations                 |
| `max_iterations` | int               | No       | Maximum reasoning iterations (default: 5) |
| `memory`         | []MemoryReference | No       | Memory references                         |

### Action Configuration Fields

| Field    | Type   | Required | Description                  |
| -------- | ------ | -------- | ---------------------------- |
| `id`     | string | Yes      | Unique action identifier     |
| `prompt` | string | Yes      | Action-specific instructions |
| `input`  | Schema | No       | Input validation schema      |
| `output` | Schema | No       | Output validation schema     |
| `with`   | Input  | No       | Default action parameters    |

---

## 🎨 Examples

### Code Review Agent

```yaml
resource: "agent"
id: "code-reviewer"

config:
  provider: "anthropic"
  model: "claude-4-opus"
  params:
    temperature: 0.3
    max_tokens: 4000

instructions: |
  You are an expert code reviewer with deep knowledge of software engineering best practices.
  Provide constructive feedback focusing on:
  - Code quality and maintainability
  - Security vulnerabilities
  - Performance optimizations
  - Best practices adherence

actions:
  - id: "review-pull-request"
    prompt: |
      Review the provided pull request changes and provide detailed feedback.
      Focus on potential issues, improvements, and best practices.
    input:
      type: "object"
      properties:
        diff:
          type: "string"
          description: "The git diff to review"
        language:
          type: "string"
          enum: ["python", "go", "javascript", "typescript"]
        context:
          type: "string"
          description: "Additional context about the changes"
      required: ["diff", "language"]
    output:
      type: "object"
      properties:
        overall_score:
          type: "number"
          minimum: 1
          maximum: 10
        issues:
          type: "array"
          items:
            type: "object"
            properties:
              severity:
                type: "string"
                enum: ["critical", "high", "medium", "low"]
              line_number:
                type: "integer"
              description:
                type: "string"
              suggestion:
                type: "string"
        recommendations:
          type: "array"
          items:
            type: "string"
tools:
  - resource: "tool"
    id: "file-reader"

memory:
  - id: "review_history"
    key: "reviews:{{.workflow.input.pr_id}}"
    mode: "read-write"

max_iterations: 5
```

### Data Analysis Agent

```yaml
resource: "agent"
id: "data-analyst"

config:
  provider: "openai"
  model: "gpt-4-turbo"
  params:
    temperature: 0.1
    max_tokens: 8000

instructions: |
  You are a skilled data analyst specializing in business intelligence.
  Analyze data patterns, generate insights, and create actionable recommendations.
  Always back your conclusions with data and provide clear visualizations when possible.

actions:
  - id: "analyze-dataset"
    prompt: |
      Analyze the provided dataset and generate comprehensive insights.
      Include statistical summaries, trend analysis, and business recommendations.
    input:
      type: "object"
      properties:
        data:
          type: "string"
          description: "CSV or JSON data to analyze"
        analysis_type:
          type: "string"
          enum: ["descriptive", "diagnostic", "predictive", "prescriptive"]
        business_context:
          type: "string"
          description: "Business context for the analysis"
      required: ["data", "analysis_type"]

  - id: "generate-report"
    prompt: |
      Generate a comprehensive business report based on the analysis results.
      Include executive summary, key findings, and actionable recommendations.
    input:
      type: "object"
      properties:
        analysis_results:
          type: "object"
          description: "Results from previous analysis"
        report_format:
          type: "string"
          enum: ["executive", "technical", "summary"]
      required: ["analysis_results"]

tools:
  - resource: "tool"
    id: "data-processor"
  - resource: "tool"
    id: "chart-generator"

mcps:
  - resource: "mcp"
    id: "database-connector"

max_iterations: 8
```

---

## 📚 API Reference

### Core Types

#### `Config`

Main agent configuration structure.

```go
type Config struct {
    Resource      string                `json:"resource,omitempty"`
    ID            string                `json:"id"`
    Config        core.ProviderConfig   `json:"config"`
    Instructions  string                `json:"instructions"`
    Actions       []*ActionConfig       `json:"actions,omitempty"`
    With          *core.Input           `json:"with,omitempty"`
    Env           *core.EnvMap          `json:"env,omitempty"`
    Tools         []tool.Config         `json:"tools,omitempty"`
    MCPs          []mcp.Config          `json:"mcps,omitempty"`
    MaxIterations int                   `json:"max_iterations,omitempty"`
    Memory        []core.MemoryReference `json:"memory,omitempty"`
}
```

#### `ActionConfig`

Configuration for structured agent actions.

```go
type ActionConfig struct {
    ID           string         `json:"id"`
    Prompt       string         `json:"prompt"`
    InputSchema  *schema.Schema `json:"input,omitempty"`
    OutputSchema *schema.Schema `json:"output,omitempty"`
    With         *core.Input    `json:"with,omitempty"`
}
```

### Key Functions

#### `Load(cwd *core.PathCWD, path string) (*Config, error)`

Loads an agent configuration from a file.

#### `FindActionConfig(actions []*ActionConfig, id string) (*ActionConfig, error)`

Finds an action configuration by ID.

#### `(c *Config) Validate() error`

Validates the entire agent configuration.

#### `(c *Config) GetMaxIterations() int`

Returns the maximum iterations with default fallback.

#### `(c *Config) NormalizeAndValidateMemoryConfig() error`

Validates memory reference configuration.

---

## 🧪 Testing

### Running Tests

```bash
# Run all agent package tests
go test -v ./engine/agent

# Run specific test
go test -v ./engine/agent -run TestConfig_Validate

# Run tests with coverage
go test -v ./engine/agent -cover
```

### Test Structure

The package includes comprehensive tests for:

- Configuration validation
- Action system functionality
- Memory reference validation
- Error handling scenarios
- Integration with core types

### Example Test

```go
func TestConfig_Validate(t *testing.T) {
    config := &agent.Config{
        ID: "test-agent",
        Config: core.ProviderConfig{
            Provider: "openai",
            Model:    "gpt-4",
        },
        Instructions: "Test instructions",
    }

    err := config.Validate()
    assert.NoError(t, err)
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
