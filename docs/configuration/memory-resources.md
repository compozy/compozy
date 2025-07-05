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
