---
status: completed
---

<task_context>
<domain>engine/infra/monitoring</domain>
<type>implementation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 6.0: Add Configuration Support

## Overview

Implement configuration support for the monitoring feature, allowing control via environment variables and project YAML configuration with proper precedence handling.

## Subtasks

- [x] 6.1 Update project configuration schema to include monitoring section ✅ COMPLETED
- [x] 6.2 Add `MONITORING_ENABLED` environment variable support ✅ COMPLETED
- [x] 6.3 Implement configuration loading in `compozy.yaml` (monitoring.enabled, monitoring.path) ✅ COMPLETED
- [x] 6.4 Add configuration validation and default values ✅ COMPLETED
- [x] 6.5 Update configuration documentation with monitoring options ✅ COMPLETED
- [x] 6.6 Create tests for configuration loading and precedence (env var vs yaml) ✅ COMPLETED

## Implementation Details

### Configuration Schema

Based on the tech spec (lines 452-464 and 394-398), implement:

```go
// In engine/config/config.go or appropriate location

type MonitoringConfig struct {
    Enabled bool   `yaml:"enabled" env:"MONITORING_ENABLED" default:"false"`
    Path    string `yaml:"path" default:"/metrics"`
}

// In the main project config
type ProjectConfig struct {
    Name       string            `yaml:"name"`
    Version    string            `yaml:"version"`
    Workflows  []WorkflowConfig  `yaml:"workflows"`
    Monitoring *MonitoringConfig `yaml:"monitoring,omitempty"`
    Runtime    RuntimeConfig     `yaml:"runtime"`
    // ... other fields ...
}
```

### Default Configuration

From lines 458-463:

```go
func DefaultMonitoringConfig() *MonitoringConfig {
    return &MonitoringConfig{
        Enabled: false,
        Path:    "/metrics",
    }
}
```

### Environment Variable Support

1. **Environment Variable** (line 390):

    - `MONITORING_ENABLED=true|false`: Global enable/disable
    - Takes precedence over YAML configuration

2. **Precedence Order**:
    - Environment variables (highest priority)
    - YAML configuration
    - Default values (lowest priority)

### Example YAML Configuration

From lines 468-484:

```yaml
name: my-monitored-project
version: 0.1.0

workflows:
    - source: ./workflow.yaml

monitoring:
    enabled: true
    path: /metrics # Optional, defaults to /metrics

runtime:
    permissions:
        - --allow-read
        - --allow-net
        - --allow-env
```

### Configuration Loading Implementation

```go
// Configuration loading with proper precedence
func LoadMonitoringConfig(yamlConfig *MonitoringConfig) *MonitoringConfig {
    // Start with defaults
    config := DefaultMonitoringConfig()

    // Apply YAML config if provided
    if yamlConfig != nil {
        if yamlConfig.Path != "" {
            config.Path = yamlConfig.Path
        }
        // YAML enabled flag only applies if not overridden by env
        config.Enabled = yamlConfig.Enabled
    }

    // Environment variable takes precedence
    if envEnabled := os.Getenv("MONITORING_ENABLED"); envEnabled != "" {
        enabled, err := strconv.ParseBool(envEnabled)
        if err != nil {
            log.Error("Invalid MONITORING_ENABLED value", "value", envEnabled)
        } else {
            config.Enabled = enabled
        }
    }

    return config
}
```

### Configuration Validation

```go
func (c *MonitoringConfig) Validate() error {
    // Validate path starts with /
    if !strings.HasPrefix(c.Path, "/") {
        return fmt.Errorf("monitoring path must start with /")
    }

    // Validate path doesn't conflict with API routes
    if strings.HasPrefix(c.Path, "/api/") {
        return fmt.Errorf("monitoring path cannot be under /api/")
    }

    // Path should not contain query parameters
    if strings.Contains(c.Path, "?") {
        return fmt.Errorf("monitoring path cannot contain query parameters")
    }

    return nil
}
```

### Integration with Config Loading

Update the main configuration loader:

```go
func LoadProjectConfig(path string) (*ProjectConfig, error) {
    // ... existing config loading ...

    // Load monitoring config with proper defaults
    if config.Monitoring == nil {
        config.Monitoring = DefaultMonitoringConfig()
    } else {
        config.Monitoring = LoadMonitoringConfig(config.Monitoring)
    }

    // Validate monitoring config
    if err := config.Monitoring.Validate(); err != nil {
        return nil, fmt.Errorf("invalid monitoring config: %w", err)
    }

    return config, nil
}
```

### Documentation Updates

Add to configuration documentation:

````markdown
## Monitoring Configuration

The monitoring feature can be configured via environment variables or project YAML.

### Environment Variables

- `MONITORING_ENABLED`: Enable/disable monitoring (true/false). Takes precedence over YAML.

### YAML Configuration

Add a `monitoring` section to your `compozy.yaml`:

```yaml
monitoring:
    enabled: true # Enable monitoring (default: false)
    path: /metrics # Metrics endpoint path (default: /metrics)
```
````

````

### Configuration Precedence

1. Environment variables (highest priority)
2. YAML configuration
3. Default values

### Example

```yaml
# Enable monitoring with custom path
monitoring:
    enabled: true
    path: /custom-metrics
````

To disable via environment:

```bash
export MONITORING_ENABLED=false
compozy run
```

```

### Testing Requirements

1. **Default Values Test**:
   - Verify defaults when no config provided
   - Check enabled=false, path="/metrics"

2. **YAML Loading Test**:
   - Test valid YAML configuration
   - Test partial configuration (only enabled or only path)

3. **Environment Override Test**:
   - Set MONITORING_ENABLED=true, verify it overrides YAML
   - Test invalid boolean values

4. **Precedence Test**:
   - YAML says enabled=true, env says false
   - Verify env wins

5. **Validation Tests**:
   - Invalid path formats
   - Path conflicts with API routes

## Success Criteria

- Configuration supports both environment variables and YAML
- Environment variable `MONITORING_ENABLED` properly controls global state
- Project-level `monitoring` section in compozy.yaml works correctly
- Default configuration values are sensible and well-documented
- Configuration validation prevents invalid values
- All configuration tests pass with comprehensive coverage
- Documentation clearly explains all configuration options

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
  - Architecture patterns: `.cursor/rules/architecture.mdc`
  - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
  - Testing requirements: `.cursor/rules/testing-standards.mdc`
  - API standards: `.cursor/rules/api-standards.mdc`
  - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test-all` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
```
