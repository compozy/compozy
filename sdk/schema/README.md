# Schema Package - Hybrid Approach

This package provides a **hybrid approach** to schema configuration in the Compozy SDK:

1. **PropertyBuilder Pattern** (from `sdk/schema/`) - For dynamic schema construction
2. **Functional Options** (this package) - For static schema configuration with metadata

## When to Use Each Pattern

### Use PropertyBuilder (`sdk/schema/`) for Dynamic Schema Construction

When you need to build schemas programmatically at runtime based on dynamic conditions:

```go
import sdkschema "github.com/compozy/compozy/sdk/schema"

// Build schema dynamically
schema := sdkschema.NewObject().
    AddProperty("name", sdkschema.NewString().WithMinLength(1)).
    AddProperty("age", sdkschema.NewInteger().WithMinimum(0)).
    RequireProperty("name").
    Build(ctx)
```

**Use cases:**

- Runtime-dependent schemas
- Conditional property addition
- Dynamic validation rules
- Schema generation from external sources

### Use Functional Options (`sdk/schema/`) for Static Schema Configuration

When you have a complete JSON schema definition and just need to wrap it with metadata:

```go
import "github.com/compozy/compozy/sdk/v2/schema"

// Static schema with metadata
schemaConfig, err := schema.New(ctx, "user-schema",
    schema.WithJSONSchema(map[string]any{
        "type": "object",
        "title": "User Schema",
        "description": "Validates user data",
        "properties": map[string]any{
            "name": map[string]any{"type": "string"},
            "age": map[string]any{"type": "integer"},
        },
        "required": []string{"name"},
    }),
)
```

**Use cases:**

- Fixed schema definitions
- Configuration metadata
- Schema versioning
- Simple wrapper around complete JSON schemas

## Installation

```go
import "github.com/compozy/compozy/sdk/v2/schema"
```

## API Reference

### Constructor

```go
func New(ctx context.Context, id string, opts ...Option) (*engschema.Schema, error)
```

Creates a new schema configuration with the provided ID and options. Returns a deep-copied schema ready for use.

**Parameters:**

- `ctx`: Context for logging and cancellation (required, non-nil)
- `id`: Schema identifier (required, non-empty)
- `opts`: Functional options for configuration

**Returns:**

- `*engschema.Schema`: Deep-copied schema configuration
- `error`: Validation errors if any

### Options

#### WithJSONSchema

```go
func WithJSONSchema(jsonSchema map[string]any) Option
```

Sets the complete JSON schema definition. Accepts a `map[string]any` representing the full JSON schema object.

**Note:** The "id" field is automatically preserved from the constructor and will not be overwritten.

**Supported fields:**

- `type`: Schema type (object, string, number, integer, boolean, array, null)
- `title`: Human-readable title
- `description`: Schema description
- `properties`: Object properties (can be from Builder or plain map)
- `required`: Array of required property names
- `version`: Schema version (custom field)
- Any other valid JSON Schema fields

## Usage Examples

### Basic Example

```go
schema, err := schema.New(ctx, "simple-schema",
    schema.WithJSONSchema(map[string]any{
        "type": "string",
        "minLength": 1,
        "maxLength": 100,
    }),
)
if err != nil {
    log.Fatal(err)
}
```

### Full Configuration Example

```go
schema, err := schema.New(ctx, "user-schema",
    schema.WithJSONSchema(map[string]any{
        "type": "object",
        "title": "User Schema",
        "description": "Validates user data",
        "version": "1.0.0",
        "properties": map[string]any{
            "name": map[string]any{
                "type": "string",
                "minLength": 1,
                "maxLength": 100,
            },
            "email": map[string]any{
                "type": "string",
                "pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
            },
            "age": map[string]any{
                "type": "integer",
                "minimum": 0,
                "maximum": 150,
            },
        },
        "required": []string{"name", "email"},
    }),
)
```

### Using PropertyBuilder with Functional Options

You can combine both approaches - use PropertyBuilder to construct the schema, then wrap it:

```go
import (
    sdkschema "github.com/compozy/compozy/sdk/schema"
    "github.com/compozy/compozy/sdk/v2/schema"
)

// Build dynamic properties with PropertyBuilder
properties, err := sdkschema.NewObject().
    AddProperty("name", sdkschema.NewString()).
    AddProperty("age", sdkschema.NewInteger()).
    Build(ctx)
if err != nil {
    return err
}

// Wrap with metadata using functional options
schemaConfig, err := schema.New(ctx, "dynamic-schema",
    schema.WithJSONSchema(map[string]any{
        "type": "object",
        "title": "Dynamic Schema",
        "properties": *properties,
    }),
)
```

## Migration Guide

### Before (Old SDK - Builder Pattern Only)

```go
import "github.com/compozy/compozy/sdk/schema"

schema := schema.NewObject().
    AddProperty("name", schema.NewString()).
    AddProperty("age", schema.NewInteger()).
    Build(ctx)
```

### After (New SDK - Hybrid Approach)

**Option 1: Keep using PropertyBuilder for dynamic schemas**

```go
import sdkschema "github.com/compozy/compozy/sdk/schema"

// Still works! Use for dynamic construction
schema := sdkschema.NewObject().
    AddProperty("name", sdkschema.NewString()).
    AddProperty("age", sdkschema.NewInteger()).
    Build(ctx)
```

**Option 2: Use functional options for static schemas**

```go
import "github.com/compozy/compozy/sdk/v2/schema"

// New approach for static schemas
schemaConfig, err := schema.New(ctx, "user-schema",
    schema.WithJSONSchema(map[string]any{
        "type": "object",
        "properties": map[string]any{
            "name": map[string]any{"type": "string"},
            "age": map[string]any{"type": "integer"},
        },
    }),
)
```

### Key Changes

1. **PropertyBuilder API remains unchanged** - No breaking changes for dynamic schema construction
2. **New functional options API** - Simpler approach for static schema configuration
3. **ID is required** - Schema configurations now require an explicit ID
4. **Context-first** - Context is the first parameter (Go idiom)
5. **Metadata support** - Easy to add version, title, description, etc.

## Error Handling

All validation errors are collected and returned together:

```go
schema, err := schema.New(ctx, "",  // Invalid: empty ID
    schema.WithJSONSchema(map[string]any{
        "type": "invalid-type",  // Invalid: unsupported type
    }),
)
if err != nil {
    // Error will contain:
    // - "id is invalid: id cannot be empty"
    // - "invalid schema type: invalid-type"
    fmt.Println(err)
}
```

## Validation

The constructor performs validation on:

- **ID**: Must be non-empty and valid format
- **Schema type**: Must be one of: object, string, number, integer, boolean, array, null
- **Properties**: Must be a `map[string]any` if present
- **Required**: Must be a string array if present

## Deep Copy

All schemas returned by `New()` are deep-copied to prevent accidental mutation:

```go
schema1, _ := schema.New(ctx, "test", schema.WithJSONSchema(props))
// Modifying schema1 won't affect future schemas created with same props
```

## Testing

Run tests:

```bash
gotestsum --format pkgname -- -race -parallel=4 ./sdk/schema
```

Run linting:

```bash
golangci-lint run --fix --allow-parallel-runners ./sdk/schema/...
```

## Architecture Decision

**Why the hybrid approach?**

1. **PropertyBuilder is still valuable** - Dynamic schema construction at runtime cannot be easily replaced with functional options
2. **Functional options for static config** - When you have a complete schema definition, functional options are simpler and more idiomatic
3. **Best of both worlds** - Use the right tool for the job:
   - PropertyBuilder: Runtime flexibility
   - Functional options: Static configuration simplicity

**Design principles:**

- **No breaking changes**: PropertyBuilder API remains unchanged
- **Minimal API**: Single `WithJSONSchema()` option instead of dozens of field-specific options
- **Idiomatic Go**: Follows functional options pattern used throughout Go ecosystem
- **Type safety**: Full type checking at compile time

## Examples

See `sdk/cmd/` directory for complete examples using both patterns.

## Support

For issues or questions, please open an issue on GitHub.
