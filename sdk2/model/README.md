# Model SDK Package

Auto-generated functional options for LLM provider configuration.

## Overview

The `model` package provides a clean, type-safe API for configuring LLM providers in Compozy. It uses auto-generated functional options to reduce boilerplate and ensure consistency with the engine layer.

## Usage

### Basic Configuration

```go
import (
    "context"
    "github.com/compozy/compozy/sdk/v2/model"
)

func main() {
    ctx := context.Background()

    // Minimal configuration
    cfg, err := model.New(ctx, "openai", "gpt-4")
    if err != nil {
        panic(err)
    }
}
```

### Full Configuration

```go
import (
    "context"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/sdk/v2/model"
)

func main() {
    ctx := context.Background()

    // Configure parameters
    params := core.PromptParams{}
    params.SetTemperature(0.7)
    params.SetMaxTokens(2000)
    params.SetTopP(0.9)

    cfg, err := model.New(ctx, "openai", "gpt-4-turbo",
        model.WithAPIKey("{{ .env.OPENAI_API_KEY }}"),
        model.WithAPIURL("https://api.openai.com/v1"),
        model.WithParams(params),
        model.WithOrganization("org-123456"),
        model.WithDefault(true),
        model.WithMaxToolIterations(10),
        model.WithContextWindow(128000),
    )
    if err != nil {
        panic(err)
    }
}
```

## Supported Providers

- `openai` - OpenAI (GPT-4, GPT-3.5, etc.)
- `anthropic` - Anthropic (Claude models)
- `google` - Google (Gemini models)
- `groq` - Groq (fast inference)
- `ollama` - Ollama (local models)
- `deepseek` - DeepSeek AI models
- `xai` - xAI (Grok models)
- `cerebras` - Cerebras (fast inference)
- `openrouter` - OpenRouter (multi-model gateway)

## Available Options

### WithProvider

Sets the LLM provider. Automatically normalized to lowercase.

```go
model.New(ctx, "OpenAI", "gpt-4") // Normalized to "openai"
```

### WithModel

Sets the specific model identifier.

```go
model.New(ctx, "openai", "gpt-4-turbo")
```

### WithAPIKey

Sets the authentication key. Use environment variable templates for security.

```go
model.WithAPIKey("{{ .env.OPENAI_API_KEY }}")
```

### WithAPIURL

Sets a custom API endpoint for local hosting or proxies.

```go
model.WithAPIURL("http://localhost:11434") // Ollama
model.WithAPIURL("https://api.openai.com/v1") // OpenAI
```

### WithParams

Configures generation parameters (temperature, max_tokens, etc.).

```go
params := core.PromptParams{}
params.SetTemperature(0.7)
params.SetMaxTokens(2000)
model.WithParams(params)
```

### WithOrganization

Sets the organization ID (primarily for OpenAI).

```go
model.WithOrganization("org-123456789")
```

### WithDefault

Marks this model as the default fallback.

```go
model.WithDefault(true)
```

### WithMaxToolIterations

Limits the number of tool-call iterations.

```go
model.WithMaxToolIterations(10)
```

### WithContextWindow

Overrides the provider's default context window size.

```go
model.WithContextWindow(200000) // Claude 3.5 Sonnet via OpenRouter
```

## Parameter Validation

### Temperature

- **Range:** 0.0 to 2.0
- **Default:** Provider-specific
- Controls randomness and creativity

### MaxTokens

- **Range:** Positive integers
- **Default:** Provider-specific
- Limits response length

### TopP

- **Range:** 0.0 to 1.0
- **Default:** Provider-specific
- Nucleus sampling threshold

### FrequencyPenalty

- **Range:** -2.0 to 2.0
- **Default:** 0.0
- Penalizes token frequency

### PresencePenalty

- **Range:** -2.0 to 2.0
- **Default:** 0.0
- Encourages topic diversity

## Error Handling

The constructor returns a `BuildError` containing all validation errors:

```go
cfg, err := model.New(ctx, "invalid", "")
if err != nil {
    var buildErr *sdkerrors.BuildError
    if errors.As(err, &buildErr) {
        for _, e := range buildErr.Errors {
            fmt.Println(e)
        }
    }
}
```

## Auto-Generation

Options are auto-generated from `engine/core/provider.go`:

```bash
cd sdk2/model
go generate
```

This generates `options_generated.go` with all field options. **Never edit this file manually.**

## Validation Rules

1. **Provider:** Must be one of the supported providers
2. **Model:** Cannot be empty
3. **API URL:** Must be a valid URL (if provided)
4. **Temperature:** 0.0 to 2.0
5. **MaxTokens:** Must be positive
6. **TopP:** 0.0 to 1.0
7. **Frequency/Presence Penalty:** -2.0 to 2.0

## Migration from Builder Pattern

### Before (Old Builder)

```go
cfg, err := model.New("openai", "gpt-4").
    WithTemperature(0.7).
    WithMaxTokens(2000).
    Build(ctx)
```

### After (Functional Options)

```go
params := core.PromptParams{}
params.SetTemperature(0.7)
params.SetMaxTokens(2000)

cfg, err := model.New(ctx, "openai", "gpt-4",
    model.WithParams(params),
)
```

## Key Changes

1. **Context First:** `ctx` is now the first parameter
2. **No Build():** Validation happens immediately
3. **Params Object:** Use `core.PromptParams` with setters
4. **Provider/Model Required:** Both are constructor parameters

## Benefits

- ✅ **70% Less Boilerplate:** Auto-generated options
- ✅ **Zero Maintenance:** `go generate` syncs with engine
- ✅ **Type Safety:** Compile-time validation
- ✅ **Centralized Validation:** All checks in one place
- ✅ **Idiomatic Go:** Standard functional options pattern

## Testing

Run tests with:

```bash
gotestsum --format pkgname -- -race -parallel=4 ./sdk2/model
```

Current coverage: >90% of business logic
