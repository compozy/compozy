## status: pending

<task_context>
<domain>sdk/schema</domain>
<type>implementation</type>
<scope>code_generation</scope>
<complexity>medium</complexity>
<dependencies>sdk2/model</dependencies>
</task_context>

# Task 7.0: Migrate schema Package to Functional Options (Hybrid Approach)

## Overview

**SPECIAL CASE:** Hybrid migration for schema package. Keep the dynamic `PropertyBuilder` pattern for schema construction, but migrate static `Schema` configuration to a simple functional option approach using `WithJsonSchema(map[string]any)`.

**Estimated Time:** 2-3 hours

**Dependency:** Requires Task 1.0 (model) complete

**Simplified Approach:** Instead of generating multiple `With*()` methods for each JSON schema property, we use a single `WithJsonSchema()` option that accepts a `map[string]any` to avoid boilerplate.

<critical>
- **HYBRID APPROACH:** Keep PropertyBuilder, migrate Schema config only
- **DO NOT MIGRATE:** PropertyBuilder and its fluent API (needed for dynamic schemas)
- **MIGRATE ONLY:** Top-level Schema configuration wrapper
</critical>

<requirements>
- Keep existing PropertyBuilder pattern (builder.go stays)
- Create single WithJsonSchema() option accepting map[string]any
- Maintain backwards compatibility for PropertyBuilder API
- No code generation needed - simple functional option approach
- Deep copy and tests for new API only
</requirements>

## Subtasks

- [ ] 7.1 Create sdk2/schema/ directory structure
- [ ] 7.2 Analyze which parts to migrate vs keep
- [ ] 7.3 Create constructor with WithJsonSchema() option
- [ ] 7.4 Implement schema validation
- [ ] 7.5 Tests for new functional options
- [ ] 7.6 Keep PropertyBuilder unchanged (in sdk/schema/)
- [ ] 7.7 Update README with hybrid approach

## Implementation Details

### Keep Unchanged (Dynamic Schema Building)
```go
// Keep this pattern - needed for dynamic schema construction
schema := schema.NewProperty("object").
    AddProperty("name", schema.NewProperty("string")).
    AddProperty("age", schema.NewProperty("integer")).
    Build()
```

### Migrate (Static Schema Configuration)
```go
// Migrate this to functional options with a single WithJsonSchema()
schemaConfig, err := schema.New(ctx, "user-schema",
    schema.WithJsonSchema(map[string]any{
        "type": "object",
        "title": "User Schema",
        "description": "Validates user data",
        "properties": propertySchema, // Built with PropertyBuilder or plain map
        "version": "1.0.0",
    }),
)
```

### Engine Source
- engine/schema/schema.go - Schema type alias (map[string]any)
- Keep PropertyBuilder for dynamic construction
- Add wrapper config for metadata

### Relevant Files

**Reference (for understanding):**
- `sdk/schema/builder.go` - PropertyBuilder pattern (STAYS as reference)
- `sdk/schema/builder_test.go` - Old tests to understand test cases
- `engine/schema/schema.go` - Source type for generation

**To Create in sdk2/schema/:**
- `options.go` - WithJsonSchema() functional option
- `constructor.go` - Schema configuration validation
- `constructor_test.go` - Test suite for new API
- `README.md` - Hybrid approach documentation

**Note:** Do NOT delete or modify anything in `sdk/schema/` - PropertyBuilder stays as reference. We're building a NEW approach in sdk2/schema/ for schema configuration/metadata, while keeping PropertyBuilder pattern for dynamic schema construction.

## Tests

- [ ] PropertyBuilder still works (regression tests)
- [ ] Schema with WithJsonSchema() option
- [ ] Schema with complete JSON schema (type, properties, required, etc.)
- [ ] Schema validation for required fields
- [ ] Integration: PropertyBuilder output → WithJsonSchema()
- [ ] Backwards compatibility maintained

## Success Criteria

- [ ] sdk2/schema/ directory created
- [ ] PropertyBuilder API unchanged and working (in sdk/schema/)
- [ ] WithJsonSchema() option accepting map[string]any
- [ ] Clear documentation of hybrid approach
- [ ] No breaking changes to existing PropertyBuilder users
- [ ] Tests pass: `gotestsum -- ./sdk2/schema`
- [ ] Linter clean: `golangci-lint run ./sdk2/schema/...`
- [ ] README explains when to use each pattern:
  - PropertyBuilder (sdk/schema/): Dynamic schema construction at runtime
  - WithJsonSchema() (sdk2/schema/): Static schema configuration via map[string]any
