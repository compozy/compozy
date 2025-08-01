# `schemagen` – _JSON Schema Generator for Compozy Configuration_

> **A CLI tool that generates JSON schemas from Go structs to provide IDE support and validation for Compozy YAML configurations.**

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

The `schemagen` package is a command-line tool that automatically generates JSON schemas from Go struct definitions in the Compozy engine. It creates comprehensive schemas for all configuration types including agents, workflows, tasks, tools, and more, enabling IDE autocomplete, validation, and documentation for YAML configuration files.

The tool supports both one-time generation and watch mode for continuous development, ensuring schemas stay synchronized with code changes.

---

## 💡 Motivation

- **IDE Support**: Provide rich autocomplete and validation for YAML configuration files
- **Developer Experience**: Catch configuration errors early with schema validation
- **Documentation**: Auto-generate comprehensive documentation for configuration options
- **Cross-references**: Link related configuration types with proper JSON schema references

---

## ⚡ Design Highlights

- **Automatic Schema Generation**: Reflects Go structs to create JSON schemas with proper types and validation
- **Cross-reference Support**: Handles complex relationships between configuration types using `$ref`
- **Watch Mode**: Continuously monitors source files and regenerates schemas on changes
- **Comment Integration**: Includes Go comments in generated schemas for better documentation
- **Validation Tags**: Respects Go validation tags for schema constraints
- **Draft 7 Compliance**: Generates schemas compatible with JSON Schema Draft 7

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- Access to the Compozy engine source code

### Installation

```bash
# Build the tool
go build -o schemagen ./pkg/schemagen

# Or run directly
go run ./pkg/schemagen/main.go
```

### Quick Start

```bash
# Generate schemas to default directory (./schemas)
./schemagen

# Generate schemas to custom directory
./schemagen -out ./config/schemas

# Watch mode for development
./schemagen -watch -out ./schemas
```

---

## 📖 Usage

### Command Line Interface

```bash
Usage: schemagen [options]

Options:
  -out string
        output directory for generated schemas (default "./schemas")
  -watch
        watch config files and regenerate schemas on changes
  -log-level string
        log level (debug, info, warn, error) (default "info")
  -log-json
        output logs in JSON format
  -log-source
        include source code location in logs
```

### Basic Usage

```bash
# Generate schemas once
./schemagen -out ./schemas

# Watch for changes during development
./schemagen -watch -out ./schemas -log-level debug
```

### Integration with IDEs

After generation, configure your IDE to use the schemas:

**VS Code (settings.json):**

```json
{
  "yaml.schemas": {
    "./schemas/workflow.json": [
      "**/workflows/*.yaml",
      "**/workflows/*.yml"
    ],
    "./schemas/agent.json": [
      "**/agents/*.yaml",
      "**/agents/*.yml"
    ],
    "./schemas/task.json": [
      "**/tasks/*.yaml",
      "**/tasks/*.yml"
    ]
  }
}
```

---

## 🔧 Configuration

### Schema Generation Options

The tool generates schemas for these configuration types:

| Schema               | Go Type              | Description                 |
| -------------------- | -------------------- | --------------------------- |
| `agent.json`         | `agent.Config`       | Agent configuration         |
| `action-config.json` | `agent.ActionConfig` | Agent action configuration  |
| `project.json`       | `project.Config`     | Project configuration       |
| `workflow.json`      | `workflow.Config`    | Workflow configuration      |
| `task.json`          | `task.Config`        | Task configuration          |
| `tool.json`          | `tool.Config`        | Tool configuration          |
| `mcp.json`           | `mcp.Config`         | MCP configuration           |
| `memory.json`        | `memory.Config`      | Memory configuration        |
| `cache.json`         | `cache.Config`       | Cache configuration         |
| `monitoring.json`    | `monitoring.Config`  | Monitoring configuration    |
| `config.json`        | `config.Config`      | Application configuration   |
| `compozy.json`       | Combined             | Unified compozy.yaml schema |

### Schema Features

- **Base URI**: `https://schemas.compozy.dev/`
- **Validation**: Respects `validate:"required"` tags
- **Cross-references**: Links between related schemas using `$ref`
- **YAML Compatibility**: Includes `yamlCompatible: true` metadata
- **Draft 7**: Uses JSON Schema Draft 7 specification

---

## 🎨 Examples

### Generated Schema Structure

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://schemas.compozy.dev/agent.json",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "name": {
      "type": "string",
      "description": "Agent name"
    },
    "tools": {
      "type": "array",
      "items": {
        "$ref": "tool.json"
      }
    },
    "mcps": {
      "type": "array",
      "items": {
        "$ref": "mcp.json"
      }
    }
  },
  "required": [
    "name"
  ],
  "yamlCompatible": true
}
```

### Watch Mode Output

```bash
$ ./schemagen -watch -log-level info
INFO[0000] Generating JSON schemas
INFO[0000] Generated schema                              file=/path/to/schemas/agent.json
INFO[0000] Generated schema                              file=/path/to/schemas/workflow.json
INFO[0000] Starting file watcher for config changes. Press Ctrl+C to exit.
INFO[0005] Config file modified                         file=engine/agent/config.go op=Write
INFO[0005] Regenerating schemas due to config changes   file=engine/agent/config.go
INFO[0005] Schemas regenerated successfully
```

### IDE Integration Example

With generated schemas, your IDE will provide:

```yaml
# Auto-completion and validation
agent:
  name: "data-processor" # ✅ Required field validated
  model: "gpt-4" # ✅ Autocomplete available
  tools: # ✅ Array type validated
    - name: "calculator" # ✅ Tool schema referenced
      type: "function"
```

---

## 📚 API Reference

### Core Functions

#### `GenerateParserSchemas`

```go
func GenerateParserSchemas(ctx context.Context, outDir string) error
```

Generates JSON schemas for all configuration types and writes them to the specified directory.

**Parameters:**

- `ctx`: Context for cancellation and logging
- `outDir`: Output directory for generated schema files

**Returns:**

- `error`: Error if schema generation fails

**Example:**

```go
ctx := context.Background()
err := GenerateParserSchemas(ctx, "./schemas")
if err != nil {
    log.Fatal("Schema generation failed:", err)
}
```

#### `watchConfigFiles`

```go
func watchConfigFiles(ctx context.Context, outDir string) error
```

Watches configuration files for changes and regenerates schemas automatically.

**Parameters:**

- `ctx`: Context for cancellation and signal handling
- `outDir`: Output directory for generated schema files

**Features:**

- Recursive directory watching
- File filtering (only `.go` files)
- Debounced regeneration (500ms delay)
- Signal handling for graceful shutdown

---

## 🧪 Testing

### Running Tests

```bash
# Run unit tests
go test ./pkg/schemagen

# Test schema generation
go run ./pkg/schemagen -out ./test-schemas

# Validate generated schemas
jsonschema -i ./test-schemas/agent.json ./example-configs/agent.yaml
```

### Manual Testing

```bash
# Generate schemas and test with sample config
./schemagen -out ./schemas
echo 'agent:
  name: "test-agent"
  model: "gpt-4"' > test-agent.yaml

# Validate with a JSON schema validator
jsonschema -i ./schemas/agent.json ./test-agent.yaml
```

### Development Testing

```bash
# Start watch mode in one terminal
./schemagen -watch -log-level debug

# Make changes to engine configuration files
# Watch for automatic schema regeneration
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
