## status: completed

<task_context>
<domain>sdk/model</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/internal/errors, sdk/internal/validate</dependencies>
</task_context>

# Task 07.0: Model Builder (S)

## Overview

Implement the Model builder for configuring LLM providers and models. Supports OpenAI, Anthropic, Google, Groq, and Ollama with API keys, parameters, and default model selection.

<critical>
- **ALWAYS READ** tasks/prd-sdk/03-sdk-entities.md (Model Configuration section)
- **MUST** validate provider and model names
- **MUST** support temperature, max tokens, and other parameters
- **MUST** allow setting default model flag
</critical>

<requirements>
- Create ModelBuilder with fluent API
- Implement New(provider, model) constructor
- Implement WithAPIKey(key) method
- Implement WithAPIURL(url) method for custom endpoints
- Implement parameter methods (WithTemperature, WithMaxTokens, etc.)
- Implement WithDefault(bool) for default model selection
- Implement Build(ctx) with validation
</requirements>

## Subtasks

- [x] 07.1 Create sdk/model/builder.go with Builder struct
- [x] 07.2 Implement New(provider, model) constructor
- [x] 07.3 Implement WithAPIKey(key) *Builder
- [x] 07.4 Implement WithAPIURL(url) *Builder
- [x] 07.5 Implement WithTemperature(temp) *Builder
- [x] 07.6 Implement WithMaxTokens(max) *Builder
- [x] 07.7 Implement WithTopP(topP) *Builder
- [x] 07.8 Implement WithFrequencyPenalty(penalty) *Builder
- [x] 07.9 Implement WithPresencePenalty(penalty) *Builder
- [x] 07.10 Implement WithDefault(isDefault) *Builder
- [x] 07.11 Implement Build(ctx context.Context) (*core.ProviderConfig, error)
- [x] 07.12 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-sdk/03-sdk-entities.md (Model Configuration)

### Builder Pattern

```go
// sdk/model/builder.go
package model

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/sdk/internal/errors"
    "github.com/compozy/compozy/sdk/internal/validate"
)

type Builder struct {
    config *core.ProviderConfig
    errors []error
}

func New(provider, model string) *Builder
func (b *Builder) WithAPIKey(key string) *Builder
func (b *Builder) WithAPIURL(url string) *Builder
func (b *Builder) WithTemperature(temp float64) *Builder
func (b *Builder) WithMaxTokens(max int) *Builder
func (b *Builder) WithDefault(isDefault bool) *Builder
func (b *Builder) Build(ctx context.Context) (*core.ProviderConfig, error)
```

### Relevant Files

- `sdk/model/builder.go` (NEW)
- `sdk/model/builder_test.go` (NEW)
- `engine/core/provider_config.go` (REFERENCE)

### Dependent Files

- `sdk/internal/errors/build_error.go`
- `sdk/internal/validate/validate.go`

## Deliverables

- ✅ `sdk/model/builder.go` with complete Builder implementation
- ✅ All parameter methods (temperature, max tokens, penalties)
- ✅ Support for all providers (openai, anthropic, google, groq, ollama)
- ✅ Build(ctx) validates provider and model
- ✅ Unit tests with 95%+ coverage

## Tests

Reference: tasks/prd-sdk/_tests.md

- Unit tests for Model builder:
  - [x] Test New() with all supported providers
  - [x] Test WithAPIKey() sets API key
  - [x] Test WithAPIURL() for custom endpoints
  - [x] Test WithTemperature() validates range (0-2)
  - [x] Test WithMaxTokens() validates positive values
  - [x] Test WithTopP() validates range (0-1)
  - [x] Test WithDefault() sets default flag
  - [x] Test Build() with valid config succeeds
  - [x] Test Build() with empty provider fails
  - [x] Test Build() with empty model fails
  - [x] Test Build() with invalid temperature fails
  - [x] Test context propagation

## Success Criteria

- Model builder supports all providers
- Parameter validation works correctly
- Build(ctx) requires context.Context
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
