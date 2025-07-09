# `engine/schema` – _JSON Schema Validation and Configuration Processing_

> **A comprehensive JSON Schema validation library that provides schema compilation, validation, default value application, and composite validation patterns for the Compozy workflow engine.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Schema Operations](#schema-operations)
  - [Parameter Validation](#parameter-validation)
  - [Composite Validation](#composite-validation)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `engine/schema` package provides a robust JSON Schema validation system for the Compozy workflow orchestration engine. It offers schema compilation, validation, default value application, and advanced validation patterns including composite validation strategies.

This package serves as the foundation for validating task parameters, workflow configurations, and component inputs throughout the Compozy ecosystem, ensuring data integrity and providing clear error messages for configuration issues.

---

## 💡 Motivation

- **Type Safety**: Enforce strict validation of workflow configurations and task parameters
- **Developer Experience**: Provide clear, actionable error messages for configuration issues  
- **Default Values**: Automatically apply schema-defined defaults to simplify configuration
- **Composite Validation**: Support complex validation scenarios with multiple validators

---

## ⚡ Design Highlights

- **JSON Schema Compliance**: Built on the `jsonschema` library for standards compliance
- **Default Value Extraction**: Automatically extracts and applies default values from schemas
- **Composite Validation**: Supports combining multiple validation strategies
- **Error-First Design**: Comprehensive error handling with detailed validation messages
- **Context-Aware**: Supports context-based validation for workflow scenarios

---

## 🚀 Getting Started

The schema package is designed to be used within the Compozy workflow engine but can be used independently for JSON Schema validation needs.

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/compozy/compozy/engine/schema"
)

func main() {
    // Create a schema for user data
    userSchema := &schema.Schema{
        "type": "object",
        "properties": map[string]any{
            "name": map[string]any{
                "type": "string",
                "default": "Anonymous",
            },
            "age": map[string]any{
                "type": "number",
                "minimum": 0,
                "maximum": 150,
            },
            "email": map[string]any{
                "type": "string",
                "format": "email",
            },
        },
        "required": []string{"email"},
    }
    
    // Validate user data
    userData := map[string]any{
        "email": "user@example.com",
        "age": 30,
    }
    
    // Apply defaults first
    withDefaults, err := userSchema.ApplyDefaults(userData)
    if err != nil {
        log.Fatal(err)
    }
    
    // Then validate
    result, err := userSchema.Validate(context.Background(), withDefaults)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Validation successful: %v\n", result.Valid)
    fmt.Printf("Data with defaults: %+v\n", withDefaults)
}
```

---

## 📖 Usage

### Library

The schema package provides several core components:

```go
// Core schema type
type Schema map[string]any

// Validation result
type Result = jsonschema.EvaluationResult

// Parameter validator
type ParamsValidator struct { /* ... */ }

// Composite validator
type CompositeValidator struct { /* ... */ }

// Struct validator
type StructValidator struct { /* ... */ }

// CWD validator
type CWDValidator struct { /* ... */ }
```

### Schema Operations

#### Creating and Compiling Schemas

```go
// Create a schema
schema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "name": map[string]any{
            "type": "string",
            "default": "DefaultName",
        },
        "count": map[string]any{
            "type": "number",
            "minimum": 1,
            "default": 10,
        },
    },
    "required": []string{"name"},
}

// Compile to JSON Schema
compiled, err := schema.Compile()
if err != nil {
    log.Fatal(err)
}

// Clone a schema
cloned, err := schema.Clone()
if err != nil {
    log.Fatal(err)
}
```

#### Validation and Defaults

```go
// Apply defaults to input
input := map[string]any{
    "name": "CustomName",
    // count will get default value of 10
}

withDefaults, err := schema.ApplyDefaults(input)
if err != nil {
    log.Fatal(err)
}

// Validate the data
result, err := schema.Validate(context.Background(), withDefaults)
if err != nil {
    log.Fatal(err)
}

if result.Valid {
    fmt.Println("Validation passed")
} else {
    fmt.Printf("Validation failed: %v\n", result.Errors)
}
```

### Parameter Validation

Use `ParamsValidator` for validating parameters against schemas:

```go
// Create parameter validator
validator := schema.NewParamsValidator(
    userData,        // parameters to validate
    userSchema,      // schema to validate against
    "user-config",   // identifier for error messages
)

// Validate parameters
if err := validator.Validate(context.Background()); err != nil {
    fmt.Printf("Parameter validation failed: %v\n", err)
}
```

### Composite Validation

Combine multiple validators for complex validation scenarios:

```go
// Create individual validators
structValidator := schema.NewStructValidator(configStruct)
cwdValidator := schema.NewCWDValidator(workingDir, "project")
paramValidator := schema.NewParamsValidator(params, paramSchema, "params")

// Combine into composite validator
composite := schema.NewCompositeValidator(
    structValidator,
    cwdValidator,
    paramValidator,
)

// Validate all at once
if err := composite.Validate(); err != nil {
    fmt.Printf("Composite validation failed: %v\n", err)
}

// Or add validators dynamically
composite.AddValidator(additionalValidator)
```

---

## 🎨 Examples

### Task Configuration Validation

```go
// Schema for task configuration
taskSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "id": map[string]any{
            "type": "string",
            "pattern": "^[a-zA-Z0-9-_]+$",
        },
        "type": map[string]any{
            "type": "string",
            "enum": []string{"basic", "parallel", "collection"},
            "default": "basic",
        },
        "timeout": map[string]any{
            "type": "string",
            "default": "30s",
        },
        "retry": map[string]any{
            "type": "number",
            "minimum": 0,
            "maximum": 10,
            "default": 3,
        },
        "agent": map[string]any{
            "type": "object",
            "properties": map[string]any{
                "id": map[string]any{"type": "string"},
                "model": map[string]any{"type": "string"},
            },
            "required": []string{"id", "model"},
        },
    },
    "required": []string{"id"},
}

// Validate task configuration
taskConfig := map[string]any{
    "id": "process-data",
    "agent": map[string]any{
        "id": "data-processor",
        "model": "claude-3-haiku-20240307",
    },
    // type, timeout, retry will get default values
}

// Apply defaults and validate
withDefaults, err := taskSchema.ApplyDefaults(taskConfig)
if err != nil {
    log.Fatal(err)
}

_, err = taskSchema.Validate(context.Background(), withDefaults)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Task config with defaults: %+v\n", withDefaults)
```

### Workflow Input Validation

```go
// Create workflow input schema
workflowInputSchema := &schema.Schema{
    "type": "object",
    "properties": map[string]any{
        "user_id": map[string]any{
            "type": "string",
            "pattern": "^[0-9]+$",
        },
        "priority": map[string]any{
            "type": "string",
            "enum": []string{"low", "medium", "high"},
            "default": "medium",
        },
        "data": map[string]any{
            "type": "object",
            "properties": map[string]any{
                "source": map[string]any{
                    "type": "string",
                    "default": "api",
                },
                "format": map[string]any{
                    "type": "string",
                    "enum": []string{"json", "xml", "csv"},
                    "default": "json",
                },
            },
        },
    },
    "required": []string{"user_id"},
}

// Validate workflow input
workflowInput := map[string]any{
    "user_id": "12345",
    "data": map[string]any{
        "format": "xml",
    },
}

// Create validator
validator := schema.NewParamsValidator(
    workflowInput,
    workflowInputSchema,
    "workflow-input",
)

// Validate
if err := validator.Validate(context.Background()); err != nil {
    fmt.Printf("Workflow input validation failed: %v\n", err)
    return
}

fmt.Println("Workflow input validation passed")
```

---

## 📚 API Reference

### Core Types

#### `Schema`
```go
type Schema map[string]any
```

The main schema type that wraps a JSON Schema definition.

**Methods:**
- `String() string` - Returns JSON representation of schema
- `Compile() (*jsonschema.Schema, error)` - Compiles schema for validation
- `Validate(ctx context.Context, value any) (*Result, error)` - Validates value against schema
- `Clone() (*Schema, error)` - Creates deep copy of schema
- `ApplyDefaults(input map[string]any) (map[string]any, error)` - Applies default values

#### `ParamsValidator`
```go
func NewParamsValidator[T any](with T, schema *Schema, id string) *ParamsValidator
```

Creates a validator for parameters against a schema.

**Methods:**
- `Validate(ctx context.Context) error` - Validates parameters

#### `CompositeValidator`
```go
func NewCompositeValidator(validators ...Validator) *CompositeValidator
```

Combines multiple validators into a single validation step.

**Methods:**
- `AddValidator(validator Validator)` - Adds a validator to the composite
- `Validate() error` - Validates using all contained validators

#### `StructValidator`
```go
func NewStructValidator(value any) *StructValidator
```

Validates Go structs using struct tags.

**Methods:**
- `Validate() error` - Validates struct
- `RegisterValidation(tag string, fn validator.Func) error` - Registers custom validation

#### `CWDValidator`
```go
func NewCWDValidator(cwd *core.PathCWD, id string) *CWDValidator
```

Validates current working directory is set.

**Methods:**
- `Validate() error` - Validates CWD is present

### Interfaces

#### `Validator`
```go
type Validator interface {
    Validate() error
}
```

Common interface for all validators.

---

## 🧪 Testing

Run the test suite:

```bash
# Run all tests
go test ./engine/schema

# Run with verbose output
go test -v ./engine/schema

# Run specific test
go test -v ./engine/schema -run TestSchema_Validate

# Run with coverage
go test -cover ./engine/schema
```

### Test Categories

The test suite covers:

- **Schema Validation**: Testing various JSON Schema types and constraints
- **Default Value Application**: Ensuring defaults are properly applied
- **Error Handling**: Validation of error scenarios and messages
- **Composite Validation**: Testing combined validation strategies
- **Parameter Validation**: Context-aware parameter validation

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

MIT License - see [LICENSE](../../LICENSE)
