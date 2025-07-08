# Ref Package

The **Ref** package provides a powerful directive system for YAML/JSON configurations that enables
declarative references, transformations, and merging of configuration fragments. It's designed to
reduce duplication and enable reusable, composable configurations while keeping files valid YAML.

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
  - [Inline Merge](#inline-merge)
    - [Syntax](#syntax)
    - [Default Behavior](#default-behavior)
    - [Merge Options](#merge-options)
    - [Examples](#examples)
      - [Basic Inline Merge](#basic-inline-merge)
      - [Explicit Merge Options](#explicit-merge-options)
      - [Key Conflict Handling](#key-conflict-handling)
      - [Inline Merge with $use](#inline-merge-with-use)
      - [Complex Nested Scenarios](#complex-nested-scenarios)
  - [API Reference](#api-reference)
    - [Basic Usage](#basic-usage)
    - [Configuration Options](#configuration-options)
    - [Advanced Features](#advanced-features)
      - [Custom Directives](#custom-directives)
      - [Pre-Evaluation Hooks](#pre-evaluation-hooks)
      - [Direct Evaluator Usage](#direct-evaluator-usage)
      - [Caching](#caching)
  - [Performance](#performance)
    - [Benchmarks](#benchmarks)
  - [Best Practices](#best-practices)
    - [1. Scope Organization](#1-scope-organization)
    - [2. Avoid Deep Nesting](#2-avoid-deep-nesting)
    - [3. Use Appropriate Directives](#3-use-appropriate-directives)
    - [4. Handle Errors Properly](#4-handle-errors-properly)
  - [Examples](#examples-1)
    - [Complete Application Configuration](#complete-application-configuration)
    - [Working with Arrays](#working-with-arrays)
    - [Error Handling Examples](#error-handling-examples)
  - [License](#license)

## Features

- **Three Core Directives**: `$ref`, `$use`, and `$merge` for different use cases
- **Inline Merge**: Automatic merging of directive results with sibling keys using
  `!merge:<options>` syntax
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

Injects any value (scalar, object, or array) directly from a scope. Supports **inline merge** when
sibling keys are present.

**Syntax**: `$ref: <scope>::<gjson_path>[!merge:<options>]`

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

**Inline Merge Example**:

```yaml
# Input
server:
    $ref: "local::defaults.server"
    port: 9090
    ssl: true

# Output (automatic merge)
server:
    host: "0.0.0.0"
    port: 9090 # sibling overrides
    ssl: true # sibling adds
```

**Path Syntax**: Uses [GJSON](https://github.com/tidwall/gjson) syntax for powerful path queries:

- `config.server.port` - Nested object access
- `users.0.name` - Array index access
- `config.ports.#` - Array length
- `users.#.name` - Map array to property values

### `$use` - Component Transformation

Transforms a referenced value into a component configuration (agent, tool, or task). Supports
**inline merge** when sibling keys are present.

**Syntax**: `$use: <component>(<scope>::<gjson_path>)[!merge:<options>]`

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

**Inline Merge Example**:

```yaml
# Input
deployment:
    $use: "agent(local::worker.config)"
    resources:
        cpu: "500m"
    metadata:
        labels:
            env: "prod"

# Output (automatic merge)
deployment:
    agent:
        type: "background"
        replicas: 3
    resources:
        cpu: "500m"
    metadata:
        labels:
            env: "prod"
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

Merges multiple objects or arrays with configurable strategies. **Note**: Unlike `$ref` and `$use`,
the `$merge` directive does not support sibling keys.

**Syntax**:

```yaml
# Shorthand (array)
$merge: [source1, source2, ...]

# Explicit with options
$merge:
    strategy: deep|shallow|replace # for objects
    strategy: concat|prepend|unique|append|union # for arrays
    key_conflict: replace|first|error # for objects (default: replace)
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

## Inline Merge

The `$ref` and `$use` directives support **inline merge**, which automatically merges the directive
result with sibling keys in the same object. This enables powerful composition patterns without
requiring explicit `$merge` directives.

### Syntax

**For `$ref` directive:**

```yaml
$ref: "<scope>::<path>[!merge:<options>]"
```

**For `$use` directive:**

```yaml
$use: "<component>(<scope>::<path>)[!merge:<options>]"
```

### Default Behavior

When a `$ref` or `$use` directive has sibling keys, inline merge is **automatically enabled** with
these defaults:

- **Strategy**: `deep` (for objects)
- **Key Conflict**: `replace` (siblings override directive result)
- **No Inline Merge**: `$merge` directive does not support sibling keys

### Merge Options

The `!merge:<options>` syntax accepts the following options:

| Option                 | Values                                           | Default   | Description                 |
| ---------------------- | ------------------------------------------------ | --------- | --------------------------- |
| **Strategy** (Objects) | `deep`, `shallow`, `replace`                     | `deep`    | How to merge nested objects |
| **Strategy** (Arrays)  | `concat`, `prepend`, `unique`, `append`, `union` | `concat`  | How to merge arrays         |
| **Key Conflict**       | `replace`, `first`, `error`                      | `replace` | How to handle key conflicts |

### Examples

#### Basic Inline Merge

```yaml
# Input
server:
    $ref: "local::defaults.server"
    port: 9090
    ssl: true

# With scope: {"defaults": {"server": {"host": "localhost", "port": 8080}}}

# Output (automatic deep merge)
server:
    host: "localhost"
    port: 9090 # sibling overrides
    ssl: true # sibling adds new key
```

#### Explicit Merge Options

```yaml
# Deep merge with explicit options
config:
    $ref: "local::base.config!merge:<deep,replace>"
    timeout: 30
    features:
        logging: false

# Shallow merge (replaces nested objects entirely)
config:
    $ref: "local::base.config!merge:<shallow>"
    database:
        host: "prod-db" # completely replaces base.config.database

# Replace strategy (ignores siblings)
config:
    $ref: "local::base.config!merge:<replace>"
    timeout: 30 # this will be ignored
```

#### Key Conflict Handling

```yaml
# First wins (directive result takes precedence)
server:
    $ref: "local::defaults.server!merge:<deep,first>"
    host: "override" # ignored, directive result wins

# Error on conflicts
server:
    $ref: "local::defaults.server!merge:<deep,error>"
    host: "conflict" # ERROR: key conflict detected
```

#### Inline Merge with $use

```yaml
# $use with sibling keys
deployment:
    $use: "agent(local::worker.config)"
    metadata:
        labels:
            env: "prod"
    resources:
        cpu: "500m"

# Output
deployment:
    agent:
        type: "background"
        replicas: 3
    metadata:
        labels:
            env: "prod"
    resources:
        cpu: "500m"
```

#### Complex Nested Scenarios

```yaml
# Inline merge inside $merge directive
config:
  $merge:
    - name: "my-app"
    - database:
        $ref: "local::db.defaults!merge:<deep>"
        ssl: true
        pool:
          timeout: 30
    - $ref: "local::overrides"
# The $ref with inline merge is evaluated first, then becomes a source for $merge
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

// Enable caching with default configuration
ref.WithCacheEnabled()

// Enable caching with custom configuration
ref.WithCache(ref.CacheConfig{
    MaxCost:     50 << 20,  // 50 MB max memory
    NumCounters: 1e6,       // 1 million counters
    BufferItems: 64,        // Buffer size
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
// - **Example**: Environment variable expansion
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

#### Caching

The ref package supports high-performance caching using
[Ristretto](https://github.com/dgraph-io/ristretto) to cache path resolution results. This
significantly improves performance when the same paths are referenced multiple times.

**Important:** For best performance, reuse the same evaluator across multiple operations:

```go
// ❌ BAD: Creating new evaluator with cache for each operation (slower!)
for _, doc := range documents {
    result, err := ref.ProcessBytes(doc,
        ref.WithLocalScope(scope),
        ref.WithCacheEnabled(), // Cache is created and destroyed each time
    )
}

// ✅ GOOD: Reuse evaluator with cache across operations
ev := ref.NewEvaluator(
    ref.WithLocalScope(scope),
    ref.WithCacheEnabled(),
)
for _, doc := range documents {
    result, err := ref.ProcessBytesWithEvaluator(doc, ev)
}
```

**Single Document Processing:**

```go
// For one-off processing, use the standard API
result, err := ref.ProcessFile("config.yaml",
    ref.WithLocalScope(localScope),
    // No cache needed for single operations
)
```

**Multiple Document Processing:**

```go
// Create evaluator once with cache
ev := ref.NewEvaluator(
    ref.WithLocalScope(localScope),
    ref.WithGlobalScope(globalScope),
    ref.WithCacheEnabled(),
)

// Process multiple documents efficiently
for _, configFile := range configFiles {
    result, err := ref.ProcessFileWithEvaluator(configFile, ev)
    if err != nil {
        log.Printf("Error processing %s: %v", configFile, err)
        continue
    }
    // Use result...
}
```

**Custom Cache Configuration:**

```go
cacheConfig := ref.CacheConfig{
    MaxCost:     50 << 20,  // 50 MB maximum cache size
    NumCounters: 1e6,       // Number of frequency counters
    BufferItems: 64,        // Number of keys per Get buffer
}

ev := ref.NewEvaluator(
    ref.WithLocalScope(localScope),
    ref.WithCache(cacheConfig),
)
```

**Cache Benefits:**

- Eliminates redundant GJSON path evaluations
- Significantly improves performance for configurations with repeated references
- Thread-safe and optimized for concurrent access
- Automatic memory management with cost-based eviction

**When to Use Caching:**

- Processing multiple documents with shared scopes
- Long-running services that process configurations repeatedly
- Configurations with many repeated references to the same paths
- Server applications handling configuration per request

**When NOT to Use Caching:**

- One-off document processing
- Small configurations with few references
- When memory usage is a critical concern

## Performance

The ref package is optimized for performance:

- **JSON Caching**: Scopes are marshaled to JSON once and cached
- **Optimized Merging**: Hand-rolled deep merge for the common case (deep + last) avoids extra
  allocations
- **In-place Operations**: Merge operations modify maps in-place when possible
- **Efficient Cycle Detection**: Uses stack-based tracking with minimal overhead
- **Path Resolution Caching**: Optional Ristretto-based caching for dramatic performance
  improvements

### Benchmarks

```
BenchmarkResolvePath_NoCache-16             204399    5207 ns/op     288 B/op    7 allocs/op
BenchmarkResolvePath_WithCache-16         14513816      78 ns/op      87 B/op    2 allocs/op

BenchmarkEval_ComplexDocument/NoCache-16    165568    7227 ns/op   15290 B/op  126 allocs/op
BenchmarkEval_ComplexDocument/WithCache-16   185941    6540 ns/op   13458 B/op   90 allocs/op

BenchmarkMerge_DeepObjects-16              1506735     779 ns/op     336 B/op    2 allocs/op
BenchmarkMerge_LargeArrays/Concat-16       1534473     791 ns/op   14104 B/op    4 allocs/op
```

**Key Findings:**

- **66x faster** path resolution with caching enabled
- **10% faster** complex document evaluation with 30% fewer allocations
- Deep merge operations are highly optimized with minimal allocations
- Cache is thread-safe with excellent concurrent performance

**Note:** Cache initialization has overhead, so it's most beneficial for:

- Long-running processes that evaluate many configurations
- Configurations with repeated references to the same paths
- Server applications that process configurations on each request

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

    # Using inline merge for server configuration
    server:
        $ref: "local::defaults.server"
        port: ${PORT:-8080} # Override port with env expansion
        ssl: true # Add SSL configuration

    # Using $merge for complex scenarios
    database:
        $merge:
            - $ref: "local::defaults.database"
            - $ref: "global::overrides.database"
            - maxConnections: 100

    # Using $use with inline merge
    worker:
        $use: "agent(local::agents.worker)"
        resources:
            cpu: "500m"
            memory: "1Gi"
        scaling:
            enabled: true

    features:
        $merge:
            strategy: deep
            key_conflict: replace
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
    - { key: "value" }
    - ["array"]
# Error: $merge sources must be all objects or all arrays, not mixed

# Missing reference
missing:
  $ref: "local::does.not.exist"
# Error: path does.not.exist not found in local scope
```

## License

This package is part of the Compozy project. See the main project repository for license
information.
