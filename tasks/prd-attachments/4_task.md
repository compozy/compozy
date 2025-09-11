---
status: in-progress # Options: pending, in-progress, completed, excluded
parallelizable: true # Whether this task can run in parallel when preconditions are met
blocked_by: ["1.0"] # List of task IDs that must be completed first
---

<task_context>
<domain>pkg/config</domain>
<type>configuration</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>cli|schema_registry</dependencies>
<unblocks>"3.0", "5.0"</unblocks>
</task_context>

# Task 4.0: Global Configuration & Schema Integration

## Overview

Implement comprehensive global configuration for attachments following the established project patterns in `pkg/config`. This includes schema registration, typed structs, environment/CLI mappings, defaults, and embedding attachment configuration into Task, Agent, and Action configurations.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Follow `@.cursor/rules/global-config.mdc` patterns strictly
- Register all `attachments.*` fields in schema registry with proper defaults, CLI flags, env vars
- Add typed structs with full tagging (`koanf`, `json`, `yaml`, `mapstructure`, `env`, `validate`)
- Implement CLI visibility and diagnostics integration
- Add global limits: size, timeouts, redirects, MIME allowlists, temp quotas
- Embed `attachment.Config` into Task, Agent, and Action configurations
- Ensure `compozy config show` displays attachments settings correctly
</requirements>

## Subtasks

- [x] 4.1 Register attachment fields in `pkg/config/definition/schema.go` following global-config.mdc pattern
- [x] 4.2 Add typed structs in `pkg/config/config.go` with full tags (`koanf`, `json`, `yaml`, `mapstructure`, `env`, `validate`)
- [x] 4.3 Map from registry in appropriate `build<Section>Config(...)` functions
- [x] 4.4 Add CLI visibility in `cli/helpers/flag_categories.go` and diagnostics in `cli/cmd/config/config.go`
- [x] 4.5 Implement global limits: `max_download_size_bytes`, `download_timeout`, `max_redirects`, `allowed_mime_types.*`, `temp_dir_quota_bytes`
- [x] 4.6 Embed `attachment.Config` into Task, Agent, and Action configurations
- [x] 4.7 Validate integration: `compozy config show` displays attachments settings; validation works; env/CLI mapping functional

## Sequencing

- Blocked by: 1.0 (Domain Model & Core Interfaces)
- Unblocks: 3.0 (Normalization & Template Integration), 5.0 (Execution Wiring & Orchestrator Integration)
- Parallelizable: Yes (can run parallel with 2.0 after 1.0 is complete)

## Implementation Details

### Global Configuration Fields

Following Section 14 of technical specification, implement these global properties:

```
attachments.max_download_size_bytes (int, default: 10_000_000)
attachments.download_timeout (duration, default: 30s)
attachments.max_redirects (int, default: 3)
attachments.allowed_mime_types.image (string slice, default: ["image/*"])
attachments.allowed_mime_types.audio (string slice, default: ["audio/*"])
attachments.allowed_mime_types.video (string slice, default: ["video/*"])
attachments.allowed_mime_types.pdf (string slice, default: ["application/pdf"])
attachments.temp_dir_quota_bytes (int, optional)
```

### Schema Registration Pattern

In `pkg/config/definition/schema.go`:

```go
registry.Register(&FieldDef{
    Path:    "attachments.max_download_size_bytes",
    Default: 10_000_000,
    CLIFlag: "attachments-max-download-size",
    EnvVar:  "ATTACHMENTS_MAX_DOWNLOAD_SIZE_BYTES",
    Type:    reflect.TypeOf(0),
    Help:    "Maximum download size in bytes for attachment resolution",
})
```

### Configuration Integration

Embed `attachment.Config` into existing configurations:

- `engine/task/config.go` - `BaseConfig` struct
- `engine/agent/config.go` - `Config` struct
- `engine/agent/action_config.go` - `ActionConfig` struct

### CLI Integration

Add to `cli/helpers/flag_categories.go` under appropriate category:

```go
{
    Name: "Attachments & Media",
    Flags: []string{
        "attachments-max-download-size",
        "attachments-download-timeout",
        "attachments-max-redirects",
    },
}
```

### Relevant Files

- `pkg/config/definition/schema.go` - Schema registration
- `pkg/config/config.go` - Typed structs and builders
- `pkg/config/provider.go` - Defaults mapping
- `cli/helpers/flag_categories.go` - CLI categorization
- `cli/cmd/config/config.go` - Diagnostics integration
- `engine/task/config.go` - Task configuration embedding
- `engine/agent/config.go` - Agent configuration embedding
- `engine/agent/action_config.go` - Action configuration embedding

### Dependent Files

- `engine/attachment/config.go` - For `attachment.Config` type

## Success Criteria

- All `attachments.*` fields appear in `compozy config show -f table|json|yaml`
- `compozy config diagnostics --verbose` correctly lists attachment field sources
- CLI help groups attachment flags under correct category
- Environment variable mapping works for all attachment settings
- Global configuration validation catches invalid values (negative timeouts, etc.)
- `attachment.Config` successfully embeds into Task, Agent, and Action configs
- Schema validation works for attachment-related configurations
- All linter checks pass (`make lint`)
- All tests pass (`make test`)
- Configuration tests cover attachment-specific scenarios
