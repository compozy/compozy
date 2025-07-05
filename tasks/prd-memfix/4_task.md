---
status: pending # Options: pending, in-progress, completed, excluded
---

<task_context>
<domain>engine/memory</domain>
<type>configuration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 4.0: ResourceRegistry Configuration Validation

## Overview

Ensure memory manager initializes correctly by validating ResourceRegistry configuration and autoload setup. Create test memory resource configurations and verify they are properly discovered and loaded by the system.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** check dependent files APIs before write tests to avoid write wrong code
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [ ] 4.1 Create test memory resource configuration files
- [ ] 4.2 Verify autoload configuration includes memory resources
- [ ] 4.3 Validate memory manager initialization in worker startup
- [ ] 4.4 Create tests for ResourceRegistry memory resource loading
- [ ] 4.5 Document memory resource configuration requirements

## Implementation Details

### Memory Resource Configuration

Create test memory resource files to ensure proper discovery:

**File: `test/fixtures/memory/test_memory.yaml`**

```yaml
resource: memory
id: test_memory
description: Test memory for integration testing
version: 0.1.0

key: "test:{{.session_id}}"
type: token_based
max_tokens: 1000
max_messages: 50

persistence:
    type: redis
    ttl: 1h

token_provider:
    provider: openai
    model: gpt-3.5-turbo

locking:
    append_ttl: "30s"
    clear_ttl: "10s"
    flush_ttl: "5m"
```

**File: `test/fixtures/memory/conversation_memory.yaml`**

```yaml
resource: memory
id: conversation_memory
description: Conversation history with privacy protection
version: 0.1.0

key: "conv:{{.user_id}}:{{.session_id}}"
type: token_based
max_tokens: 4000
max_messages: 100

persistence:
    type: redis
    ttl: 24h

privacy_policy:
    redact_patterns:
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b' # Email
        - '\b\d{3}-\d{2}-\d{4}\b' # SSN
    non_persistable_message_types:
        - system
        - tool
    default_redaction_string: "[REDACTED]"

flushing_strategy:
    type: hybrid_summary
    threshold: 0.8
    summarize_threshold: 0.6
    summary_tokens: 500
```

### Autoload Configuration Test

**File: `test/fixtures/compozy-test.yaml`**

```yaml
name: test-project
version: 0.1.0
description: Test project with memory resources

workflows:
    - source: ./workflows/*.yaml

models:
    - provider: openai
      model: gpt-3.5-turbo
      api_key: "{{ .env.OPENAI_API_KEY }}"

runtime:
    type: bun
    entrypoint: "./index.ts"

# AutoLoad configuration for memory resources
autoload:
    enabled: true
    strict: false
    include:
        - "memory/*.yaml"
        - "test/fixtures/memory/*.yaml"
    exclude:
        - "**/*~"
        - "**/*.bak"
        - "**/*.tmp"
```

### ResourceRegistry Loading Tests

**File: `engine/autoload/memory_resource_test.go`**

```go
package autoload

import (
    "context"
    "path/filepath"
    "testing"

    "github.com/compozy/compozy/engine/core"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAutoload_MemoryResources(t *testing.T) {
    t.Run("Should discover and load memory resource files", func(t *testing.T) {
        // Arrange
        config := &Config{
            Enabled: true,
            Include: []string{"test/fixtures/memory/*.yaml"},
            Exclude: DefaultExcludes,
        }

        cwd, err := core.CWDFromPath("../..")
        require.NoError(t, err)

        registry := NewConfigRegistry()
        autoloader := New(config, cwd, registry)

        // Act
        err = autoloader.Run(context.Background())

        // Assert
        assert.NoError(t, err)

        // Verify test_memory resource loaded
        testMemory, err := registry.Get("memory", "test_memory")
        assert.NoError(t, err)
        assert.NotNil(t, testMemory)

        // Verify conversation_memory resource loaded
        convMemory, err := registry.Get("memory", "conversation_memory")
        assert.NoError(t, err)
        assert.NotNil(t, convMemory)
    })

    t.Run("Should validate memory resource structure", func(t *testing.T) {
        // Arrange
        registry := NewConfigRegistry()
        memoryConfig := map[string]any{
            "resource":    "memory",
            "id":          "valid_memory",
            "description": "Valid memory configuration",
            "version":     "0.1.0",
            "key":         "test:{{.id}}",
            "type":        "token_based",
            "max_tokens":  1000,
            "persistence": map[string]any{
                "type": "redis",
                "ttl":  "1h",
            },
        }

        // Act
        err := registry.Register(memoryConfig, "test/valid_memory.yaml")

        // Assert
        assert.NoError(t, err)

        retrieved, err := registry.Get("memory", "valid_memory")
        assert.NoError(t, err)
        assert.Equal(t, "valid_memory", retrieved["id"])
    })
}
```

### Memory Manager Initialization Test

**File: `engine/worker/memory_init_test.go`**

```go
package worker

import (
    "context"
    "testing"

    "github.com/compozy/compozy/engine/autoload"
    "github.com/compozy/compozy/engine/memory"
    "github.com/compozy/compozy/pkg/logger"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWorker_MemoryManagerInitialization(t *testing.T) {
    t.Run("Should initialize memory manager when ResourceRegistry provided", func(t *testing.T) {
        // Arrange
        registry := createTestRegistry(t)
        config := &Config{
            ResourceRegistry: registry,
        }

        log := logger.New()
        templateEngine := createTestTemplateEngine()
        redisCache := createTestRedisCache()
        client := createTestClient()

        // Act
        memoryManager, err := setupMemoryManager(
            config,
            templateEngine,
            redisCache,
            client,
            "test-queue",
            log,
        )

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, memoryManager)
    })

    t.Run("Should return nil when ResourceRegistry not provided", func(t *testing.T) {
        // Arrange
        config := &Config{
            ResourceRegistry: nil, // No registry
        }

        log := logger.New()

        // Act
        memoryManager, err := setupMemoryManager(
            config,
            nil,
            nil,
            nil,
            "test-queue",
            log,
        )

        // Assert
        assert.NoError(t, err)
        assert.Nil(t, memoryManager)
        // Should have logged warning about memory features being disabled
    })
}

func TestMemoryManager_LoadConfiguration(t *testing.T) {
    t.Run("Should create memory instance from loaded configuration", func(t *testing.T) {
        // Arrange
        registry := createTestRegistry(t)

        // Register a test memory configuration
        memoryConfig := map[string]any{
            "resource":   "memory",
            "id":         "test_instance",
            "key":        "test:{{.id}}",
            "type":       "token_based",
            "max_tokens": 1000,
        }
        err := registry.Register(memoryConfig, "test.yaml")
        require.NoError(t, err)

        memoryManager := createTestMemoryManager(registry)

        // Act
        instance, err := memoryManager.GetInstance(
            context.Background(),
            core.MemoryReference{
                ID:  "test_instance",
                Key: "test:123",
            },
            map[string]any{"id": "123"},
        )

        // Assert
        assert.NoError(t, err)
        assert.NotNil(t, instance)
        assert.Equal(t, "test:123", instance.GetID())
    })
}
```

### Documentation

**File: `docs/configuration/memory-resources.md`**

````markdown
# Memory Resource Configuration

Memory resources in Compozy are configured through YAML files that are automatically discovered via the autoload system.

## Configuration Structure

Each memory resource must be defined in a separate YAML file with the following structure:

```yaml
resource: memory # Required: identifies this as a memory resource
id: unique_id # Required: unique identifier for this memory resource
description: Human-readable description
version: 0.1.0

# Memory key template using Go template syntax
key: "prefix:{{.user_id}}:{{.session_id}}"

# Memory type and limits
type: token_based # Options: token_based (default)
max_tokens: 4000 # Maximum tokens before flushing
max_messages: 100 # Maximum messages to retain

# Persistence configuration
persistence:
    type: redis # Currently only redis supported
    ttl: 24h # Time-to-live for memory entries

# Optional: Token counting provider
token_provider:
    provider: openai
    model: gpt-3.5-turbo
    api_key_env: OPENAI_API_KEY

# Optional: Privacy configuration
privacy_policy:
    redact_patterns:
        - '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b' # Email
    non_persistable_message_types:
        - system
    default_redaction_string: "[REDACTED]"

# Optional: Locking configuration
locking:
    append_ttl: "30s"
    clear_ttl: "10s"
    flush_ttl: "5m"

# Optional: Flushing strategy
flushing_strategy:
    type: hybrid_summary
    threshold: 0.8
    summarize_threshold: 0.6
    summary_tokens: 500
```
````

## Autoload Configuration

Memory resources are discovered through the project's autoload configuration:

```yaml
# compozy.yaml
autoload:
    enabled: true
    include:
        - "memory/*.yaml" # Standard location
        - "resources/*.yaml" # Alternative location
```

## Validation Requirements

- `resource` field must be set to "memory"
- `id` field must be unique within the project
- `key` field must be a valid Go template
- Persistence type defaults to "redis" if not specified
- Token limits default to 0 (unlimited) if not specified

## Troubleshooting

### Memory Features Disabled

If you see "Resource registry not provided, memory features will be disabled" in logs:

1. Ensure autoload is enabled in compozy.yaml
2. Verify memory resource files are in included directories
3. Check that memory resource files have correct structure

### Memory Resource Not Found

If memory tasks fail with "memory resource not found":

1. Verify the memory_ref in your task matches the resource id
2. Check that the resource file is being discovered by autoload
3. Ensure no syntax errors in the memory resource YAML

```

### Relevant Files

> Files that this task will create/modify:
- `test/fixtures/memory/*.yaml` - Test memory resource configurations
- `test/fixtures/compozy-test.yaml` - Test project configuration
- `engine/autoload/memory_resource_test.go` - ResourceRegistry tests
- `engine/worker/memory_init_test.go` - Memory manager initialization tests
- `docs/configuration/memory-resources.md` - Configuration documentation

### Dependent Files

> Files that must be checked for configuration compatibility:
- `engine/autoload/registry.go` - ResourceRegistry implementation
- `engine/worker/mod.go` - Worker setup with memory manager
- `engine/memory/manager.go` - Memory manager initialization
- `engine/memory/config.go` - Memory configuration structures

## Success Criteria
- [ ] Test memory resource files are properly structured and valid
- [ ] Autoload configuration successfully discovers memory resources
- [ ] ResourceRegistry correctly loads and stores memory configurations
- [ ] Memory manager initializes without warnings when resources are available
- [ ] Memory instances can be created from loaded configurations
- [ ] Clear documentation explains configuration requirements
- [ ] Tests verify both success and failure scenarios
- [ ] No regression in autoload functionality for other resource types
```
