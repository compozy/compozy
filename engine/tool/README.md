# `tool` – _External Tool Integration Framework_

> **The tool package provides a comprehensive framework for integrating external tools, scripts, and APIs into Compozy workflows, enabling AI agents to extend their capabilities with deterministic operations.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `tool` package is a core component of the Compozy orchestration engine that enables AI agents to interact with external systems, execute scripts, and perform deterministic operations. It provides a standardized interface for defining, validating, and executing tools that extend agent capabilities beyond LLM reasoning.

**Key Features:**
- 🔧 **Multi-runtime Support**: JavaScript/TypeScript, CLI commands, HTTP APIs, and MCP servers
- 📋 **Schema Validation**: JSON Schema-based input/output validation
- ⏱️ **Timeout Management**: Configurable execution timeouts with fallback support
- 🔗 **LLM Integration**: Automatic function calling definitions for AI agents
- 🌍 **Environment Control**: Isolated environment variable management
- 📊 **Template Support**: Dynamic configuration with Go template evaluation

---

## 💡 Motivation

- **Extend AI Capabilities**: Enable AI agents to perform actions beyond text generation
- **Standardize Integration**: Provide consistent interface for external tool integration
- **Ensure Reliability**: Built-in validation, timeout controls, and error handling
- **Enable Composition**: Tools can be combined and reused across different workflows

---

## ⚡ Design Highlights

### Schema-First Architecture
Tools define their input/output contracts using JSON Schema, ensuring type safety and providing clear documentation for AI agents about expected parameters and return values.

### Runtime Flexibility
Support for multiple execution environments:
- **JavaScript/TypeScript**: For custom logic and Node.js ecosystem integration
- **CLI Commands**: For system utilities and existing scripts
- **HTTP APIs**: For web service integration
- **MCP Servers**: For Model Context Protocol compliance

### LLM Function Integration
Automatic generation of LLM function definitions from tool schemas, enabling seamless AI agent integration with proper parameter passing and validation.

### Environment Isolation
Each tool executes in its own environment context with controlled access to environment variables, preventing interference between tools.

---

## 🚀 Getting Started

### Basic Tool Definition

```yaml
# tools/file-reader.yaml
resource: "tool"
id: "file-reader"
description: "Read and parse various file formats"
timeout: "30s"

input:
  type: "object"
  properties:
    path:
      type: "string"
      description: "File path to read"
    format:
      type: "string"
      enum: ["json", "yaml", "csv", "txt"]
      default: "json"
  required: ["path"]

output:
  type: "object"
  properties:
    content:
      type: "string"
    metadata:
      type: "object"

with:
  encoding: "utf-8"
  max_size: "10MB"

env:
  TEMP_DIR: "/tmp/compozy"
```

### Loading and Using Tools

```go
package main

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/tool"
)

func main() {
    // Load tool configuration
    cwd, _ := core.CWDFromPath("/path/to/project")
    config, err := tool.Load(cwd, "tools/file-reader.yaml")
    if err != nil {
        panic(err)
    }

    // Validate input
    input := &core.Input{
        "path": "/path/to/data.json",
        "format": "json",
    }
    
    ctx := context.Background()
    if err := config.ValidateInput(ctx, input); err != nil {
        panic(err)
    }

    // Get LLM function definition
    llmDef := config.GetLLMDefinition()
    fmt.Printf("Tool: %s\n", llmDef.Function.Name)
    fmt.Printf("Description: %s\n", llmDef.Function.Description)
}
```

---

## 📖 Usage

### Library

#### Tool Configuration Management

```go
// Load tool with template evaluation
evaluator := ref.NewEvaluator()
config, err := tool.LoadAndEval(cwd, "tools/api-client.yaml", evaluator)
if err != nil {
    return err
}

// Validate configuration
if err := config.Validate(); err != nil {
    return fmt.Errorf("invalid tool config: %w", err)
}

// Check if tool has schema validation
if config.HasSchema() {
    fmt.Println("Tool has input/output schemas")
}
```

#### Input/Output Validation

```go
// Validate input parameters
input := &core.Input{
    "endpoint": "https://api.example.com/users",
    "method": "GET",
    "headers": map[string]string{
        "Authorization": "Bearer token123",
    },
}

if err := config.ValidateInput(ctx, input); err != nil {
    return fmt.Errorf("invalid input: %w", err)
}

// Validate output after execution
output := &core.Output{
    "status": 200,
    "data": []map[string]any{
        {"id": 1, "name": "John"},
        {"id": 2, "name": "Jane"},
    },
}

if err := config.ValidateOutput(ctx, output); err != nil {
    return fmt.Errorf("invalid output: %w", err)
}
```

#### Timeout Management

```go
// Get effective timeout (tool-specific or global fallback)
globalTimeout := 60 * time.Second
timeout, err := config.GetTimeout(globalTimeout)
if err != nil {
    return fmt.Errorf("invalid timeout: %w", err)
}

// Use timeout in execution context
ctx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()
```

#### Environment Variables

```go
// Access tool environment
env := config.GetEnv()
apiKey := env["API_KEY"]
baseURL := env["BASE_URL"]

// Merge with default input
defaultInput := config.GetInput()
finalInput := mergeInputs(defaultInput, userInput)
```

---

## 🔧 Configuration

### Tool Configuration Schema

```yaml
# Required fields
resource: "tool"              # Must be "tool"
id: "unique-tool-id"          # Unique identifier
description: "Tool purpose"   # Human-readable description

# Optional fields
timeout: "30s"                # Execution timeout (Go duration format)

# Input validation schema (JSON Schema Draft 7)
input:
  type: "object"
  properties:
    param1:
      type: "string"
      description: "Parameter description"
    param2:
      type: "integer"
      minimum: 1
      maximum: 100
  required: ["param1"]

# Output validation schema (JSON Schema Draft 7)
output:
  type: "object"
  properties:
    result:
      type: "string"
    metadata:
      type: "object"

# Default input parameters
with:
  default_param: "default_value"
  retry_count: 3

# Environment variables
env:
  API_KEY: "{{ .secrets.API_KEY }}"
  BASE_URL: "https://api.example.com"
  TIMEOUT: "30s"
```

### Runtime Types

#### JavaScript/TypeScript Tools
```yaml
# Detected automatically by .js/.ts extension
id: "data-processor"
description: "Process JSON data with custom logic"
execute: "./scripts/process-data.js"  # or .ts
```

#### CLI Command Tools
```yaml
id: "file-converter"
description: "Convert files using external utility"
execute: ["pandoc", "-f", "markdown", "-t", "html"]
```

#### HTTP API Tools
```yaml
id: "api-client"
description: "Make HTTP requests"
execute:
  url: "https://api.example.com/endpoint"
  method: "POST"
  headers:
    Content-Type: "application/json"
```

#### MCP Server Tools
```yaml
id: "mcp-tool"
description: "Tool from MCP server"
execute:
  mcp_server: "local-server"
  tool_name: "filesystem_read"
```

---

## 🎨 Examples

### Data Processing Tool

```yaml
# tools/csv-processor.yaml
resource: "tool"
id: "csv-processor"
description: "Process CSV files with filtering and transformation"
timeout: "2m"

input:
  type: "object"
  properties:
    file_path:
      type: "string"
      description: "Path to CSV file"
    filters:
      type: "array"
      items:
        type: "object"
        properties:
          column:
            type: "string"
          operator:
            type: "string"
            enum: ["eq", "ne", "gt", "lt", "contains"]
          value:
            type: "string"
    output_format:
      type: "string"
      enum: ["csv", "json", "xlsx"]
      default: "json"
  required: ["file_path"]

output:
  type: "object"
  properties:
    processed_data:
      type: "array"
    row_count:
      type: "integer"
    columns:
      type: "array"
      items:
        type: "string"

with:
  encoding: "utf-8"
  delimiter: ","
  has_header: true

env:
  TEMP_DIR: "/tmp/compozy"
  MAX_FILE_SIZE: "100MB"
```

### API Integration Tool

```yaml
# tools/slack-notifier.yaml
resource: "tool"
id: "slack-notifier"
description: "Send notifications to Slack channels"
timeout: "10s"

input:
  type: "object"
  properties:
    channel:
      type: "string"
      description: "Slack channel name or ID"
    message:
      type: "string"
      description: "Message to send"
    attachments:
      type: "array"
      items:
        type: "object"
        properties:
          title:
            type: "string"
          text:
            type: "string"
          color:
            type: "string"
            enum: ["good", "warning", "danger"]
  required: ["channel", "message"]

output:
  type: "object"
  properties:
    message_id:
      type: "string"
    timestamp:
      type: "string"
    channel_id:
      type: "string"

with:
  username: "Compozy Bot"
  icon_emoji: ":robot_face:"

env:
  SLACK_TOKEN: "{{ .secrets.SLACK_BOT_TOKEN }}"
  SLACK_WEBHOOK_URL: "{{ .secrets.SLACK_WEBHOOK_URL }}"
```

### Custom JavaScript Tool

```yaml
# tools/text-analyzer.yaml
resource: "tool"
id: "text-analyzer"
description: "Analyze text content for sentiment and keywords"
timeout: "45s"
execute: "./scripts/analyze-text.js"

input:
  type: "object"
  properties:
    text:
      type: "string"
      description: "Text to analyze"
      minLength: 1
      maxLength: 10000
    analysis_type:
      type: "array"
      items:
        type: "string"
        enum: ["sentiment", "keywords", "readability", "language"]
      default: ["sentiment", "keywords"]
  required: ["text"]

output:
  type: "object"
  properties:
    sentiment:
      type: "object"
      properties:
        score:
          type: "number"
        label:
          type: "string"
          enum: ["positive", "neutral", "negative"]
    keywords:
      type: "array"
      items:
        type: "object"
        properties:
          word:
            type: "string"
          relevance:
            type: "number"
    readability:
      type: "object"
      properties:
        score:
          type: "number"
        level:
          type: "string"
    language:
      type: "string"

env:
  NODE_ENV: "production"
  ANALYSIS_MODEL: "advanced"
```

---

## 📚 API Reference

### Core Types

#### `Config`
Main configuration struct for tools.

```go
type Config struct {
    Resource     string         `json:"resource"`
    ID          string         `json:"id"`
    Description string         `json:"description"`
    Timeout     string         `json:"timeout"`
    InputSchema *schema.Schema `json:"input"`
    OutputSchema *schema.Schema `json:"output"`
    With        *core.Input    `json:"with"`
    Env         *core.EnvMap   `json:"env"`
}
```

**Key Methods:**
- `Validate() error` - Validates tool configuration
- `ValidateInput(ctx context.Context, input *core.Input) error` - Validates input parameters
- `ValidateOutput(ctx context.Context, output *core.Output) error` - Validates output data
- `GetTimeout(globalTimeout time.Duration) (time.Duration, error)` - Gets effective timeout
- `GetLLMDefinition() llms.Tool` - Generates LLM function definition
- `HasSchema() bool` - Checks if tool has validation schemas

### Functions

#### `Load(cwd *core.PathCWD, path string) (*Config, error)`
Loads tool configuration from file.

```go
cwd, _ := core.CWDFromPath("/project")
config, err := tool.Load(cwd, "tools/my-tool.yaml")
```

#### `LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error)`
Loads tool configuration with template evaluation.

```go
evaluator := ref.NewEvaluator()
config, err := tool.LoadAndEval(cwd, "tools/my-tool.yaml", evaluator)
```

#### `IsTypeScript(path string) bool`
Checks if file has TypeScript extension.

```go
if tool.IsTypeScript("script.ts") {
    // Handle TypeScript execution
}
```

### Usage Patterns

#### Error Handling
```go
if err := config.Validate(); err != nil {
    switch {
    case errors.Is(err, schema.ErrInvalidSchema):
        // Handle schema validation error
    case errors.Is(err, core.ErrInvalidTimeout):
        // Handle timeout error
    default:
        // Handle general error
    }
}
```

#### Configuration Merging
```go
// Clone configuration
cloned, err := config.Clone()
if err != nil {
    return err
}

// Merge with overrides
override := &Config{
    Timeout: "60s",
    Env: &core.EnvMap{
        "DEBUG": "true",
    },
}
if err := cloned.Merge(override); err != nil {
    return err
}
```

---

## 🧪 Testing

### Unit Tests

```go
func TestConfig_Validate(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "Should validate valid config",
            config: &Config{
                Resource:    "tool",
                ID:          "test-tool",
                Description: "Test tool",
                Timeout:     "30s",
            },
            wantErr: false,
        },
        {
            name: "Should reject invalid timeout",
            config: &Config{
                Resource:    "tool",
                ID:          "test-tool",
                Description: "Test tool",
                Timeout:     "invalid",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

```go
func TestTool_ExecuteWithValidation(t *testing.T) {
    // Setup test tool
    config := &Config{
        ID:          "test-tool",
        Description: "Test tool for integration",
        InputSchema: &schema.Schema{
            Type: "object",
            Properties: map[string]*schema.Schema{
                "message": {
                    Type: "string",
                },
            },
            Required: []string{"message"},
        },
    }

    // Test input validation
    validInput := &core.Input{
        "message": "Hello, world!",
    }
    
    ctx := context.Background()
    err := config.ValidateInput(ctx, validInput)
    assert.NoError(t, err)

    // Test invalid input
    invalidInput := &core.Input{
        "invalid": "field",
    }
    
    err = config.ValidateInput(ctx, invalidInput)
    assert.Error(t, err)
}
```

### Best Practices

1. **Always validate configuration** before using tools
2. **Use schema validation** for type safety
3. **Set appropriate timeouts** based on tool complexity
4. **Test with realistic data** and edge cases
5. **Mock external dependencies** in unit tests
6. **Use table-driven tests** for multiple scenarios

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE)
