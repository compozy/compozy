## status: pending

<task_context>
<domain>v2/model</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>v2/internal/errors, v2/internal/validate</dependencies>
</task_context>

# Task 07.0: Model Builder (S)

## Overview

Implement the Model builder for configuring LLM providers and models. Supports OpenAI, Anthropic, Google, Groq, and Ollama with API keys, parameters, and default model selection.

<critical>
- **ALWAYS READ** tasks/prd-modules/03-sdk-entities.md (Model Configuration section)
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

- [ ] 07.1 Create v2/model/builder.go with Builder struct
- [ ] 07.2 Implement New(provider, model) constructor
- [ ] 07.3 Implement WithAPIKey(key) *Builder
- [ ] 07.4 Implement WithAPIURL(url) *Builder
- [ ] 07.5 Implement WithTemperature(temp) *Builder
- [ ] 07.6 Implement WithMaxTokens(max) *Builder
- [ ] 07.7 Implement WithTopP(topP) *Builder
- [ ] 07.8 Implement WithFrequencyPenalty(penalty) *Builder
- [ ] 07.9 Implement WithPresencePenalty(penalty) *Builder
- [ ] 07.10 Implement WithDefault(isDefault) *Builder
- [ ] 07.11 Implement Build(ctx context.Context) (*core.ProviderConfig, error)
- [ ] 07.12 Add comprehensive unit tests

## Implementation Details

Reference: tasks/prd-modules/03-sdk-entities.md (Model Configuration)

### Builder Pattern

```go
// v2/model/builder.go
package model

import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/v2/internal/errors"
    "github.com/compozy/compozy/v2/internal/validate"
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

- `v2/model/builder.go` (NEW)
- `v2/model/builder_test.go` (NEW)
- `engine/core/provider_config.go` (REFERENCE)

### Dependent Files

- `v2/internal/errors/build_error.go`
- `v2/internal/validate/validate.go`

## Deliverables

- ✅ `v2/model/builder.go` with complete Builder implementation
- ✅ All parameter methods (temperature, max tokens, penalties)
- ✅ Support for all providers (openai, anthropic, google, groq, ollama)
- ✅ Build(ctx) validates provider and model
- ✅ Unit tests with 95%+ coverage

## Tests

Reference: tasks/prd-modules/_tests.md

- Unit tests for Model builder:
  - [ ] Test New() with all supported providers
  - [ ] Test WithAPIKey() sets API key
  - [ ] Test WithAPIURL() for custom endpoints
  - [ ] Test WithTemperature() validates range (0-2)
  - [ ] Test WithMaxTokens() validates positive values
  - [ ] Test WithTopP() validates range (0-1)
  - [ ] Test WithDefault() sets default flag
  - [ ] Test Build() with valid config succeeds
  - [ ] Test Build() with empty provider fails
  - [ ] Test Build() with empty model fails
  - [ ] Test Build() with invalid temperature fails
  - [ ] Test context propagation

## Success Criteria

- Model builder supports all providers
- Parameter validation works correctly
- Build(ctx) requires context.Context
- Tests achieve 95%+ coverage
- Error messages are clear and actionable
