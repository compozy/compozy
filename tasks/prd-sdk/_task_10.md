## status: pending

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

- [ ] 10.1 Create sdk/schema/builder.go with SchemaBuilder struct
- [ ] 10.2 Implement type constructors (NewObject, NewString, etc.)
- [ ] 10.3 Implement string constraints (WithMinLength, WithMaxLength, WithPattern, WithEnum)
- [ ] 10.4 Implement number constraints (WithMinimum, WithMaximum)
- [ ] 10.5 Implement array constraints (WithMinItems, WithMaxItems)
- [ ] 10.6 Implement object methods (AddProperty, RequireProperty)
- [ ] 10.7 Implement WithDefault() and WithDescription()
- [ ] 10.8 Implement WithRef() for schema references
- [ ] 10.9 Implement ValidateSchema(ctx) for validation
- [ ] 10.10 Implement TestAgainstSample(ctx, sample) for testing
- [ ] 10.11 Create sdk/schema/property.go with PropertyBuilder
- [ ] 10.12 Implement property builder methods
- [ ] 10.13 Implement Build(ctx) for both builders
- [ ] 10.14 Add comprehensive unit tests

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
  - [ ] Test NewObject() creates object schema
  - [ ] Test NewString() creates string schema
  - [ ] Test NewNumber() creates number schema
  - [ ] Test NewInteger() creates integer schema
  - [ ] Test NewBoolean() creates boolean schema
  - [ ] Test NewArray() creates array schema
  - [ ] Test AddProperty() adds object properties
  - [ ] Test RequireProperty() marks properties required
  - [ ] Test WithMinLength() validates string length
  - [ ] Test WithPattern() validates string pattern
  - [ ] Test WithEnum() restricts string values
  - [ ] Test WithMinimum() validates number range
  - [ ] Test WithMinItems() validates array length
  - [ ] Test WithDefault() sets default value
  - [ ] Test WithRef() creates schema reference
  - [ ] Test ValidateSchema() validates schema structure
  - [ ] Test TestAgainstSample() validates sample data
  - [ ] Test nested schemas (object with object properties)
  - [ ] Test array of objects

- Unit tests for Property builder:
  - [ ] Test NewProperty() creates property
  - [ ] Test WithType() sets property type
  - [ ] Test WithDescription() sets description
  - [ ] Test WithDefault() sets default value
  - [ ] Test Required() marks property required
  - [ ] Test Build() with valid property succeeds

## Success Criteria

- Schema builder supports all JSON schema types
- All constraint methods work correctly
- Nested schemas (objects, arrays) work correctly
- Schema validation catches invalid schemas
- Sample testing validates data against schemas
- Build(ctx) requires context.Context for both builders
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
