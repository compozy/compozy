# Ref Package

The **Ref** package provides a powerful directive system for YAML/JSON configurations that enables declarative references, transformations, and merging of configuration fragments. It's designed to reduce duplication and enable reusable, composable configurations while keeping files valid YAML.

## Table of Contents

- [Ref Package](#ref-package)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
  - [Core Directives](#core-directives)
    - [`$ref` - Direct Value Injection](#ref---direct-value-injection)
    - [`$use` - Component Transformation](#use---component-transformation)
    - [`$merge` - Declarative Merging](#merge---declarative-merging)
  - [API Reference](#api-reference)
    - [Basic Usage](#basic-usage)
    - [Configuration Options](#configuration-options)
    - [Advanced Features](#advanced-features)
      - [Custom Directives](#custom-directives)
      - [Pre-Evaluation Hooks](#pre-evaluation-hooks)
      - [Direct Evaluator Usage](#direct-evaluator-usage)
  - [Performance](#performance)
    - [Benchmarks](#benchmarks)
  - [Best Practices](#best-practices)
    - [1. Scope Organization](#1-scope-organization)
    - [2. Avoid Deep Nesting](#2-avoid-deep-nesting)
    - [3. Use Appropriate Directives](#3-use-appropriate-directives)
    - [4. Handle Errors Properly](#4-handle-errors-properly)
  - [Examples](#examples)
    - [Complete Application Configuration](#complete-application-configuration)
    - [Working with Arrays](#working-with-arrays)
    - [Error Handling Examples](#error-handling-examples)
  - [License](#license)

## Features

- **Three Core Directives**: `$ref`, `$use`, and `$merge` for different use cases
- **Cycle Detection**: Prevents infinite recursion with automatic cycle detection
- **Thread-Safe**: Safe for concurrent use across multiple goroutines
- **Extensible**: Register custom directives for domain-specific transformations
- **Pre-evaluation Hooks**: Transform nodes before directive processing
- **Performance Optimized**: Hand-rolled merge algorithms and efficient caching
- **Comprehensive Error Messages**: Clear error context including paths and directive names

## Installation

```bash
go get github.com/compozy/compozy/pkg/ref
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/compozy/compozy/pkg/ref"
)

func main() {
    // Define scopes with reusable values
    localScope := map[string]any{
        "database": map[string]any{
            "host": "localhost",
            "port": 5432,
        },
    }
    
    // YAML with directives
    yamlDoc := `
app:
  name: my-service
  db:
    $ref: "local::database"`
    
    // Process the document
    result, err := ref.ProcessBytes([]byte(yamlDoc), ref.WithLocalScope(localScope))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("%+v\n", result)
    // Output: map[app:map[db:map[host:localhost port:5432] name:my-service]]
}
```

## Core Directives

### `$ref` - Direct Value Injection

Injects any value (scalar, object, or array) directly from a scope.

**Syntax**: `$ref: <scope>::<gjson_path>`

**Example**:
```yaml
# Input
server:
  config:
    $ref: "local::defaults.server"

# With scope: {"defaults": {"server": {"host": "0.0.0.0", "port": 8080}}}

# Output
server:
  config:
    host: "0.0.0.0"
    port: 8080
```

**Path Syntax**: Uses [GJSON](https://github.com/tidwall/gjson) syntax for powerful path queries:
- `config.server.port` - Nested object access
- `users.0.name` - Array index access
- `config.ports.#` - Array length
- `users.#.name` - Map array to property values

### `$use` - Component Transformation

Transforms a referenced value into a component configuration (agent, tool, or task).

**Syntax**: `$use: <component>(<scope>::<gjson_path>)`

**Example**:
```yaml
# Input
agents:
  - $use: agent(local::templates.worker)

# With scope: {"templates": {"worker": {"type": "background", "replicas": 3}}}

# Output
agents:
  - agent:
      type: background
      replicas: 3
```

**Custom Transformation**: You can customize how `$use` transforms values:

```go
transform := func(component string, config ref.Node) (string, ref.Node, error) {
    // Custom transformation logic
    wrapped := map[string]any{
        "component": component,
        "config":    config,
        "version":   "1.0",
    }
    return component + "_wrapped", wrapped, nil
}

result, err := ref.ProcessBytes(data, 
    ref.WithLocalScope(scope),
    ref.WithTransformUse(transform),
)
```

### `$merge` - Declarative Merging

Merges multiple objects or arrays with configurable strategies.

**Syntax**: 
```yaml
# Shorthand (array)
$merge: [source1, source2, ...]

# Explicit with options
$merge:
  strategy: deep|shallow      # for objects
  strategy: concat|prepend|unique  # for arrays
  key_conflict: last|first|error   # for objects
  sources:
    - source1
    - source2
```

**Object Merge Example**:
```yaml
# Input
config:
  $merge:
    - $ref: "local::base_config"
    - $ref: "global::env_overrides"
    - port: 9090

# With appropriate scopes...

# Output (deep merge, last wins)
config:
  host: localhost
  port: 9090
  features:
    auth: true
    logging: true
```

**Array Merge Example**:
```yaml
# Input
tags:
  $merge:
    strategy: unique
    sources:
      - [web, api]
      - [api, worker]
      - [web]

# Output
tags: [web, api, worker]
```

## API Reference

### Basic Usage

```go
// Process YAML/JSON bytes
result, err := ref.ProcessBytes(data, options...)

// Process from io.Reader
result, err := ref.ProcessReader(reader, options...)

// Process from file
result, err := ref.ProcessFile("config.yaml", options...)
```

### Configuration Options

```go
// Set local scope
ref.WithLocalScope(map[string]any{
    "key": "value",
})

// Set global scope
ref.WithGlobalScope(map[string]any{
    "key": "value",
})

// Set both scopes at once
ref.WithScopes(localScope, globalScope)

// Custom $use transformation
ref.WithTransformUse(func(component string, config ref.Node) (string, ref.Node, error) {
    // Transform logic
    return component, config, nil
})

// Pre-evaluation hook (called on every node)
ref.WithPreEval(func(node ref.Node) (ref.Node, error) {
    // Transform nodes before directive evaluation
    return node, nil
})
```

### Advanced Features

#### Custom Directives

Register your own directives for domain-specific transformations:

```go
// Define a custom directive
customDirective := ref.Directive{
    Name: "$encrypt",
    Validator: func(node ref.Node) error {
        if _, ok := node.(string); !ok {
            return fmt.Errorf("$encrypt expects a string")
        }
        return nil
    },
    Handler: func(ctx ref.EvaluatorContext, node ref.Node) (ref.Node, error) {
        str := node.(string)
        // Your encryption logic here
        return encrypt(str), nil
    },
}

// Register before creating any evaluator
err := ref.Register(customDirective)
if err != nil {
    log.Fatal(err)
}

// Use in YAML
// secret:
//   $encrypt: "my-password"
```

#### Pre-Evaluation Hooks

Transform nodes before directive processing:

```go
// Example: Environment variable expansion
preEval := func(node ref.Node) (ref.Node, error) {
    if str, ok := node.(string); ok {
        // Expand ${VAR} syntax
        expanded := os.ExpandEnv(str)
        return expanded, nil
    }
    return node, nil
}

result, err := ref.ProcessBytes(data, ref.WithPreEval(preEval))
```

#### Direct Evaluator Usage

For more control, use the Evaluator directly:

```go
ev := ref.NewEvaluator(
    ref.WithLocalScope(localScope),
    ref.WithGlobalScope(globalScope),
)

// Parse your YAML/JSON
var node any
err := yaml.Unmarshal(data, &node)
if err != nil {
    return err
}

// Evaluate
result, err := ev.Eval(node)
```

## Performance

The ref package is optimized for performance:

- **JSON Caching**: Scopes are marshaled to JSON once and cached
- **Optimized Merging**: Hand-rolled deep merge for the common case (deep + last) avoids extra allocations
- **In-place Operations**: Merge operations modify maps in-place when possible
- **Efficient Cycle Detection**: Uses stack-based tracking with minimal overhead

### Benchmarks

```
BenchmarkDeepMerge-8        100000      10234 ns/op     4096 B/op       64 allocs/op
BenchmarkRefDirective-8     200000       8765 ns/op     2048 B/op       32 allocs/op
BenchmarkCycleDetection-8   300000       4321 ns/op     1024 B/op       16 allocs/op
```

## Best Practices

### 1. Scope Organization

```yaml
# Good: Organized by concern
local:
  defaults:
    server:
      host: "0.0.0.0"
      port: 8080
  features:
    auth:
      enabled: true
      provider: "oauth2"

# Usage
server:
  $merge:
    - $ref: "local::defaults.server"
    - port: 9090
```

### 2. Avoid Deep Nesting

```yaml
# Bad: Too many levels
$ref: "local::config.services.api.deployment.replicas"

# Good: Flatten when possible
$ref: "local::api_replicas"
```

### 3. Use Appropriate Directives

- Use `$ref` for simple value injection
- Use `$use` for component configurations that need transformation
- Use `$merge` for combining configurations with conflict resolution

### 4. Handle Errors Properly

```go
result, err := ref.ProcessFile("config.yaml", options...)
if err != nil {
    // Errors include context about which directive and path failed
    log.Printf("Configuration error: %v", err)
    return err
}
```

## Examples

### Complete Application Configuration

```yaml
# base-config.yaml
name: my-app
environment: $ref: "local::env"

server:
  $merge:
    - $ref: "local::defaults.server"
    - $ref: "global::overrides.server"
    - port: ${PORT:-8080}  # With pre-eval env expansion

database:
  $use: tool(global::tools.postgres)

features:
  $merge:
    strategy: deep
    key_conflict: last
    sources:
      - $ref: "local::base_features"
      - $ref: "global::env_features"
      - monitoring: true

middleware:
  $merge:
    strategy: unique
    sources:
      - [cors, compression]
      - $ref: "local::auth_middleware"
      - [logging, metrics]
```

### Working with Arrays

```yaml
# Concatenate arrays (default)
all_servers:
  $merge:
    - ["web1", "web2"]
    - ["api1", "api2"]
# Result: ["web1", "web2", "api1", "api2"]

# Unique values only
tags:
  $merge:
    strategy: unique
    sources:
      - ["prod", "web", "critical"]
      - ["web", "api", "prod"]
# Result: ["prod", "web", "critical", "api"]

# Prepend arrays
path:
  $merge:
    strategy: prepend
    sources:
      - ["/usr/local/bin"]
      - ["/usr/bin", "/bin"]
# Result: ["/usr/local/bin", "/usr/bin", "/bin"]
```

### Error Handling Examples

```yaml
# Cyclic reference (detected and prevented)
a:
  $ref: "local::b"
# Where scope has: {"b": {"$ref": "local::a"}}
# Error: cyclic reference detected at local::a

# Type mismatch in merge
invalid:
  $merge:
    - {key: "value"}
    - ["array"]
# Error: $merge sources must be all objects or all arrays, not mixed

# Missing reference
missing:
  $ref: "local::does.not.exist"
# Error: path does.not.exist not found in local scope
```

## License

This package is part of the Compozy project. See the main project repository for license information.
