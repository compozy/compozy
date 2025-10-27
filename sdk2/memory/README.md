# Package memory

## Overview

The memory package provides a clean, type-safe API for configuring memory resources in Compozy. Memory resources enable persistent context management for AI agents, allowing them to retain and share information across multiple interactions.

## Installation

```go
import memory "github.com/compozy/compozy/sdk/v2/memory"
```

## Usage

### Basic Example (Redis Persistence)

```go
package main

import (
	"context"
	"log"

	memory "github.com/compozy/compozy/sdk/v2/memory"
	memorycore "github.com/compozy/compozy/engine/memory/core"
)

func main() {
	ctx := context.Background()

	cfg, err := memory.New(ctx, "conversation-memory", "token_based",
		memory.WithMaxTokens(4000),
		memory.WithPersistence(memorycore.PersistenceConfig{
			Type: memorycore.RedisPersistence,
			TTL:  "24h",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use cfg with Compozy engine...
}
```

### Full Configuration Example

```go
cfg, err := memory.New(ctx, "advanced-memory", "token_based",
	memory.WithDescription("Advanced conversation memory with summarization"),
	memory.WithVersion("1.0.0"),
	memory.WithMaxTokens(4000),
	memory.WithMaxMessages(100),
	memory.WithExpiration("48h"),
	memory.WithTokenAllocation(&memorycore.TokenAllocation{
		ShortTerm: 0.6,
		LongTerm:  0.3,
		System:    0.1,
	}),
	memory.WithFlushing(&memorycore.FlushingStrategyConfig{
		Type:                memorycore.HybridSummaryFlushing,
		SummarizeThreshold:  0.8,
		SummaryTokens:       500,
	}),
	memory.WithPersistence(memorycore.PersistenceConfig{
		Type: memorycore.RedisPersistence,
		TTL:  "24h",
	}),
)
```

### In-Memory Persistence (Development/Testing)

```go
cfg, err := memory.New(ctx, "test-memory", "buffer",
	memory.WithPersistence(memorycore.PersistenceConfig{
		Type: memorycore.InMemoryPersistence,
	}),
)
```

## API Reference

### Constructor

```go
func New(ctx context.Context, id string, memType string, opts ...Option) (*enginememory.Config, error)
```

Creates a new memory configuration with the specified ID and type.

**Parameters:**

- `ctx`: Context (required, cannot be nil)
- `id`: Unique identifier for the memory resource (required, non-empty)
- `memType`: Memory management strategy (required, one of: `"token_based"`, `"message_count_based"`, `"buffer"`)
- `opts`: Variadic functional options for configuration

**Returns:**

- `*enginememory.Config`: Deep-copied memory configuration
- `error`: Validation errors if any

### Memory Types

- **`"token_based"`**: Manages memory based on token count limits (recommended for LLM contexts)
  - Requires at least one limit: `max_tokens`, `max_context_ratio`, or `max_messages`
- **`"message_count_based"`**: Manages memory based on message count limits
- **`"buffer"`**: Simple buffer that stores messages up to a limit

### Options

#### Core Configuration

- `WithResource(resource string)` - Sets resource type (auto-set to "memory")
- `WithID(id string)` - Sets unique identifier
- `WithDescription(description string)` - Sets human-readable description
- `WithVersion(version string)` - Sets version for tracking changes

#### Memory Limits

- `WithType(typeValue memorycore.Type)` - Sets memory management strategy
- `WithMaxTokens(maxTokens int)` - Sets hard limit on token count (must be non-negative)
- `WithMaxMessages(maxMessages int)` - Sets hard limit on message count (must be non-negative)
- `WithMaxContextRatio(maxContextRatio float64)` - Sets maximum portion of LLM context window (0-1)
- `WithExpiration(expiration string)` - Sets data retention period (e.g., "24h", "7d")

#### Advanced Configuration

- `WithTokenAllocation(tokenAllocation *memorycore.TokenAllocation)` - Sets token budget distribution
- `WithFlushing(flushing *memorycore.FlushingStrategyConfig)` - Sets memory management strategy
- `WithPersistence(persistence memorycore.PersistenceConfig)` - **Required** - Sets storage backend
- `WithPrivacyPolicy(privacyPolicy *memorycore.PrivacyPolicyConfig)` - Sets data protection rules
- `WithPrivacyScope(privacyScope PrivacyScope)` - Sets sharing scope across tenants
- `WithLocking(locking *memorycore.LockConfig)` - Sets distributed lock timeouts
- `WithTokenProvider(tokenProvider *memorycore.TokenProviderConfig)` - Sets token counting provider
- `WithDefaultKeyTemplate(defaultKeyTemplate string)` - Sets fallback key template
- `WithCWD(cwd *core.PathCWD)` - Sets current working directory

### Persistence Types

- **`memorycore.RedisPersistence`**: Production-grade persistence with TTL support
  - Requires `TTL` field (e.g., "24h", "7d")
- **`memorycore.InMemoryPersistence`**: Testing/development only (data lost on restart)
  - Does not require `TTL` field

## Migration Guide

### Before (Builder Pattern - Old SDK)

```go
// Old SDK (not available for memory package)
```

### After (Functional Options - New SDK)

```go
cfg, err := memory.New(ctx, "conversation-memory", "token_based",
	memory.WithMaxTokens(4000),
	memory.WithPersistence(memorycore.PersistenceConfig{
		Type: memorycore.RedisPersistence,
		TTL:  "24h",
	}),
)
```

### Key Changes

1. ✅ `ctx` is now the first parameter
2. ✅ `id` and `memType` are required constructor parameters
3. ✅ No `.Build()` call needed - validation happens in constructor
4. ✅ Options use `With` prefix and take typed values
5. ✅ Persistence configuration is required

## Validation Rules

### ID Validation

- Must be non-empty
- Must be valid identifier format

### Type Validation

- Must be one of: `"token_based"`, `"message_count_based"`, `"buffer"`
- Case-insensitive (normalized to lowercase)

### Persistence Validation

- `type` is required
- Must be one of: `"redis"`, `"in_memory"`
- Redis persistence requires `ttl` field
- TTL must be non-negative duration

### Limit Validation

- `max_tokens` must be non-negative (if specified)
- `max_messages` must be non-negative (if specified)
- `max_context_ratio` must be between 0 and 1 (if specified)
- `expiration` must be valid duration format (if specified)
- `token_based` type requires at least one limit configured
- `max_context_ratio` requires `token_provider` configuration

## Error Handling

The constructor returns `*sdkerrors.BuildError` containing all validation errors at once:

```go
cfg, err := memory.New(ctx, "", "invalid-type",
	memory.WithMaxTokens(-1),
)
if err != nil {
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		for _, e := range buildErr.Errors {
			fmt.Println("Validation error:", e)
		}
	}
}
```

## Testing

```bash
# Run tests
gotestsum --format pkgname -- -race -parallel=4 ./sdk2/memory

# Run linter
golangci-lint run --fix --allow-parallel-runners ./sdk2/memory/...
```

## Examples

See example files in `sdk/cmd/` directory for complete usage examples.
