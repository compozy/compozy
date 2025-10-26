## status: completed

<task_context>
<domain>sdk/schema</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>sdk/internal/errors, sdk/internal/validate</dependencies>
</task_context>

# Task 10.0: Schema + Property Builders (M)

## Overview

Implement Schema and Property builders for defining JSON schemas with validation. Supports all JSON types (object, string, number, integer, boolean, array) with constraints and validation helpers.

<critical>
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Schema Validation section)
- **MUST** support all JSON schema types
- **MUST** implement validation constraints (min/max, pattern, enum, etc.)
- **MUST** support nested schemas (objects, arrays)
- **MUST** provide schema validation and sample testing
</critical>

<requirements>
- Create SchemaBuilder with type constructors
- Implement NewObject(), NewString(), NewNumber(), NewInteger(), NewBoolean(), NewArray()
- Implement constraint methods (WithMinLength, WithMaxLength, WithPattern, etc.)
- Implement object property management (AddProperty, RequireProperty)
- Create PropertyBuilder for object properties
- Implement ValidateSchema() for schema validation
- Implement TestAgainstSample() for testing schemas
- Implement Build(ctx) for both builders
</requirements>

## Subtasks

- [x] 10.1 Create sdk/schema/builder.go with SchemaBuilder struct
- [x] 10.2 Implement type constructors (NewObject, NewString, etc.)
- [x] 10.3 Implement string constraints (WithMinLength, WithMaxLength, WithPattern, WithEnum)
- [x] 10.4 Implement number constraints (WithMinimum, WithMaximum)
- [x] 10.5 Implement array constraints (WithMinItems, WithMaxItems)
- [x] 10.6 Implement object methods (AddProperty, RequireProperty)
- [x] 10.7 Implement WithDefault() and WithDescription()
- [x] 10.8 Implement WithRef() for schema references
- [x] 10.9 Implement ValidateSchema(ctx) for validation
- [x] 10.10 Implement TestAgainstSample(ctx, sample) for testing
- [x] 10.11 Create sdk/schema/property.go with PropertyBuilder
- [x] 10.12 Implement property builder methods
- [x] 10.13 Implement Build(ctx) for both builders
- [x] 10.14 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-sdk/03-sdk-entities.md (Schema Validation)

### Builder Patterns

```go
// sdk/schema/builder.go
package schema

import (
    "context"
    "github.com/compozy/compozy/engine/schema"
    "github.com/compozy/compozy/sdk/internal/errors"
)

type Builder struct {
    schema schema.Schema
    errors []error
}

// Type constructors
func NewObject() *Builder
func NewString() *Builder
func NewNumber() *Builder
func NewInteger() *Builder
func NewBoolean() *Builder
func NewArray(itemType *Builder) *Builder

// Object properties
func (b *Builder) AddProperty(name string, prop *Builder) *Builder
func (b *Builder) RequireProperty(name string) *Builder

// String constraints
func (b *Builder) WithMinLength(min int) *Builder
func (b *Builder) WithMaxLength(max int) *Builder
func (b *Builder) WithPattern(pattern string) *Builder
func (b *Builder) WithEnum(values ...string) *Builder

// Number constraints
func (b *Builder) WithMinimum(min float64) *Builder
func (b *Builder) WithMaximum(max float64) *Builder

// Array constraints
func (b *Builder) WithMinItems(min int) *Builder
func (b *Builder) WithMaxItems(max int) *Builder

// Common
func (b *Builder) WithDefault(value interface{}) *Builder
func (b *Builder) WithDescription(desc string) *Builder
func (b *Builder) WithRef(schemaID string) *Builder

// Validation
func (b *Builder) ValidateSchema(ctx context.Context) error
func (b *Builder) TestAgainstSample(ctx context.Context, sample interface{}) error

func (b *Builder) Build(ctx context.Context) (*schema.Schema, error)

// sdk/schema/property.go
type PropertyBuilder struct {
    property schema.Property
    errors   []error
}

func NewProperty(name string) *PropertyBuilder
func (b *PropertyBuilder) WithType(typ string) *PropertyBuilder
func (b *PropertyBuilder) WithDescription(desc string) *PropertyBuilder
func (b *PropertyBuilder) WithDefault(value interface{}) *PropertyBuilder
func (b *PropertyBuilder) Required() *PropertyBuilder
func (b *PropertyBuilder) Build(ctx context.Context) (*schema.Property, error)
```

### Relevant Files

- `sdk/schema/builder.go` (NEW)
- `sdk/schema/property.go` (NEW)
- `sdk/schema/builder_test.go` (NEW)
- `sdk/schema/property_test.go` (NEW)
- `engine/schema/schema.go` (REFERENCE)

### Dependent Files

- `sdk/internal/errors/build_error.go`

## Deliverables

- ✅ `sdk/schema/builder.go` with complete SchemaBuilder
- ✅ `sdk/schema/property.go` with complete PropertyBuilder
- ✅ All JSON schema types supported (object, string, number, etc.)
- ✅ All constraint methods implemented
- ✅ Schema validation and sample testing
- ✅ Unit tests with 95%+ coverage for both builders

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for Schema builder:
  - [x] Test NewObject() creates object schema
  - [x] Test NewString() creates string schema
  - [x] Test NewNumber() creates number schema
  - [x] Test NewInteger() creates integer schema
  - [x] Test NewBoolean() creates boolean schema
  - [x] Test NewArray() creates array schema
  - [x] Test AddProperty() adds object properties
  - [x] Test RequireProperty() marks properties required
  - [x] Test WithMinLength() validates string length
  - [x] Test WithPattern() validates string pattern
  - [x] Test WithEnum() restricts string values
  - [x] Test WithMinimum() validates number range
  - [x] Test WithMinItems() validates array length
  - [x] Test WithDefault() sets default value
  - [x] Test WithRef() creates schema reference
  - [x] Test ValidateSchema() validates schema structure
  - [x] Test TestAgainstSample() validates sample data
  - [x] Test nested schemas (object with object properties)
  - [x] Test array of objects

- Unit tests for Property builder:
  - [x] Test NewProperty() creates property
  - [x] Test WithType() sets property type
  - [x] Test WithDescription() sets description
  - [x] Test WithDefault() sets default value
  - [x] Test Required() marks property required
  - [x] Test Build() with valid property succeeds

## Success Criteria

- Schema builder supports all JSON schema types
- All constraint methods work correctly
- Nested schemas (objects, arrays) work correctly
- Schema validation catches invalid schemas
- Sample testing validates data against schemas
- Build(ctx) requires context.Context for both builders
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
