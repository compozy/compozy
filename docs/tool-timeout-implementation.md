# Tool Execution Timeout Configuration - Technical Specification

## Overview

This document outlines the implementation plan for making tool execution timeout configurable per tool, rather than using only the global timeout setting. This will allow tools with different performance characteristics to have appropriate timeout values.

## Current State Analysis

### Current Architecture Flow

```
Task Config → Task Activities → Task UC → LLM Tool → Runtime Manager → Worker
```

### Current Timeout Implementation

- **Global Configuration**: Timeout is set globally in `runtime.Config.ToolExecutionTimeout` (default 60s)
- **CLI/Environment**: Configurable via `--tool-execution-timeout` flag and `TOOL_EXECUTION_TIMEOUT` env var
- **Worker Template**: Accepts `timeout_ms` in request payload
- **No Per-Tool Configuration**: All tools use the same global timeout

### Current Files Involved

1. **`engine/runtime/config.go`** - Global timeout configuration
2. **`engine/runtime/runtime.go`** - `ExecuteTool()` method uses global timeout
3. **`engine/runtime/compozy_worker.tpl.ts`** - Worker accepts `timeout_ms` parameter
4. **`engine/llm/tool.go`** - `InternalTool.Call()` uses runtime manager
5. **`engine/task/uc/exec_task.go`** - Calls tool execution
6. **`engine/tool/config.go`** - Tool configuration structure

## Problem Statement

Different tools have different performance characteristics:

- **Fast tools**: Simple calculations, file operations (should timeout quickly)
- **Medium tools**: API calls, data processing (moderate timeout)
- **Heavy tools**: Large dataset analysis, ML inference (need longer timeout)

Currently, all tools must use the same global timeout, which leads to:

- Either too short timeouts causing heavy tools to fail
- Or too long timeouts allowing hung fast tools to waste resources

## Proposed Solution: Tool-Level Timeout Configuration

### Design Principles

1. **Tool-Specific**: Timeout configured where tool is defined
2. **Backward Compatible**: Existing tools continue to work with global timeout
3. **Clear Semantics**: No confusion with task-level timeout (which is for task orchestration)
4. **Fallback Hierarchy**: Tool timeout → Global timeout → Hard-coded default

### Configuration Example

```yaml
# Tool with specific timeout needs
resource: tool
id: heavy_analysis_tool
description: "Analyzes large datasets - needs extended time"
execute: "./scripts/heavy_analysis.ts"
timeout: 10m # 10 minute timeout for this specific tool
input:
    type: object
    properties:
        dataset: { type: string }

---
# Fast tool with quick timeout
resource: tool
id: quick_calc_tool
description: "Simple calculations - should be fast"
execute: "./scripts/quick_calc.ts"
timeout: 30s # 30 second timeout
input:
    type: object
    properties:
        numbers: { type: array }

---
# Tool without timeout (uses global default)
resource: tool
id: standard_tool
description: "Uses global timeout setting"
execute: "./scripts/standard.ts"
# No timeout field - uses global default (60s)
```

## Implementation Plan

### 1. Update Tool Configuration

**File**: `engine/tool/config.go`

```go
type Config struct {
    Resource     string         `json:"resource,omitempty"    yaml:"resource,omitempty"`
    ID           string         `json:"id,omitempty"          yaml:"id,omitempty"`
    Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
    Execute      string         `json:"execute,omitempty"     yaml:"execute,omitempty"`
    Timeout      string         `json:"timeout,omitempty"     yaml:"timeout,omitempty"`  // NEW FIELD
    InputSchema  *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"`
    OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"`
    With         *core.Input    `json:"with,omitempty"        yaml:"with,omitempty"`
    Env          *core.EnvMap   `json:"env,omitempty"         yaml:"env,omitempty"`

    filePath string
    CWD      *core.PathCWD
}

// NEW METHOD: Get tool-specific timeout with fallback
func (t *Config) GetTimeout(globalTimeout time.Duration) time.Duration {
    if t.Timeout == "" {
        return globalTimeout
    }

    timeout, err := time.ParseDuration(t.Timeout)
    if err != nil {
        // Log error and fall back to global timeout
        return globalTimeout
    }

    return timeout
}
```

### 2. Update Tool Execution Chain

**File**: `engine/llm/tool.go`

```go
func (t *InternalTool) Call(ctx context.Context, input *core.Input) (*core.Output, error) {
    inputMap, err := t.validateInput(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("input validation failed: %w", err)
    }

    // NEW: Get tool-specific timeout
    output, err := t.executeToolWithTimeout(ctx, inputMap)
    if err != nil {
        return nil, fmt.Errorf("tool execution failed: %w", err)
    }

    err = t.validateOutput(ctx, output)
    if err != nil {
        return nil, fmt.Errorf("output processing failed: %w", err)
    }

    if output != nil {
        if resultData, ok := (*output)["result"].(map[string]any); ok {
            newOutput := core.Output(resultData)
            return &newOutput, nil
        }
    }
    return output, nil
}

// NEW METHOD: Execute tool with custom timeout
func (t *InternalTool) executeToolWithTimeout(ctx context.Context, input *core.Input) (*core.Output, error) {
    toolExecID := core.MustNewID()
    env := core.EnvMap{}
    if t.env != nil {
        env = *t.env
    }

    // Get global timeout from runtime config
    globalTimeout := t.runtime.GetGlobalTimeout()

    // Get tool-specific timeout with fallback
    toolTimeout := t.config.GetTimeout(globalTimeout)

    return t.runtime.ExecuteToolWithTimeout(ctx, t.config.ID, toolExecID, input, env, toolTimeout)
}
```

### 3. Update Runtime Manager

**File**: `engine/runtime/runtime.go`

```go
// NEW METHOD: Execute tool with custom timeout
func (rm *Manager) ExecuteToolWithTimeout(
    ctx context.Context,
    toolID string,
    toolExecID core.ID,
    input *core.Input,
    env core.EnvMap,
    timeout time.Duration,
) (*core.Output, error) {
    log := logger.FromContext(ctx)
    if err := rm.validateInputs(toolID, toolExecID, input, env); err != nil {
        return nil, err
    }

    workerPath, err := rm.getWorkerPath()
    if err != nil {
        return nil, &ToolExecutionError{
            ToolID:     toolID,
            ToolExecID: toolExecID.String(),
            Operation:  "check_worker",
            Err:        err,
        }
    }

    cmd, pipes, err := rm.setupCommand(ctx, workerPath, env)
    if err != nil {
        return nil, &ToolExecutionError{
            ToolID:     toolID,
            ToolExecID: toolExecID.String(),
            Operation:  "setup_command",
            Err:        err,
        }
    }
    defer pipes.cleanup()

    if err := rm.writeRequestWithTimeout(ctx, pipes.stdin, toolID, toolExecID, input, env, timeout); err != nil {
        return nil, &ToolExecutionError{
            ToolID:     toolID,
            ToolExecID: toolExecID.String(),
            Operation:  "write_request",
            Err:        err,
        }
    }

    response, err := rm.readResponse(ctx, cmd, pipes, toolID, toolExecID)
    if err != nil {
        return nil, err
    }

    log.Debug("Tool execution completed successfully",
        "tool_id", toolID,
        "exec_id", toolExecID.String(),
        "timeout", timeout,
    )
    return response, nil
}

// UPDATED METHOD: Use custom timeout in request
func (rm *Manager) writeRequestWithTimeout(
    ctx context.Context,
    stdin io.WriteCloser,
    toolID string,
    toolExecID core.ID,
    input *core.Input,
    env core.EnvMap,
    timeout time.Duration,
) error {
    // ... existing buffer management code ...

    request := map[string]any{
        "tool_id":      toolID,
        "tool_exec_id": toolExecID.String(),
        "input":        input,
        "env":          env,
        "timeout_ms":   int(timeout.Milliseconds()),  // Use custom timeout
    }

    // ... rest of existing method ...
}

// UPDATED METHOD: Keep backward compatibility
func (rm *Manager) ExecuteTool(
    ctx context.Context,
    toolID string,
    toolExecID core.ID,
    input *core.Input,
    env core.EnvMap,
) (*core.Output, error) {
    // Use global timeout for backward compatibility
    return rm.ExecuteToolWithTimeout(ctx, toolID, toolExecID, input, env, rm.config.ToolExecutionTimeout)
}

// NEW METHOD: Get global timeout
func (rm *Manager) GetGlobalTimeout() time.Duration {
    return rm.config.ToolExecutionTimeout
}
```

### 4. Update Tool Configuration Validation

**File**: `engine/tool/config.go`

```go
func (t *Config) Validate() error {
    // ... existing validation ...

    // NEW: Validate timeout format if provided
    if t.Timeout != "" {
        _, err := time.ParseDuration(t.Timeout)
        if err != nil {
            return fmt.Errorf("invalid timeout format '%s': %w", t.Timeout, err)
        }
    }

    return nil
}
```

## Execution Flow

### New Flow with Tool-Specific Timeout

```
Task Config (tool: heavy_analysis_tool)
  ↓
Task UC: executeTool()
  ↓
LLM Tool: Call()
  ↓
Tool Config: GetTimeout() → returns 10m (tool-specific)
  ↓
Runtime Manager: ExecuteToolWithTimeout(timeout=10m)
  ↓
Worker Request: {timeout_ms: 600000}
  ↓
Deno Worker: executeWithTimeout(toolFn, input, 600000)
```

### Backward Compatibility Flow

```
Existing Task (no tool timeout specified)
  ↓
Tool Config: GetTimeout() → returns 60s (global default)
  ↓
Runtime Manager: ExecuteToolWithTimeout(timeout=60s)
  ↓
Same behavior as before
```

## Testing Strategy

### Unit Tests

1. **Tool Config Tests**

    - Test `GetTimeout()` with valid timeout strings
    - Test `GetTimeout()` with invalid timeout strings (should fall back)
    - Test `GetTimeout()` with empty timeout (should use global)

2. **Tool Execution Tests**

    - Test tool with custom timeout executes correctly
    - Test tool without timeout uses global default
    - Test timeout validation in tool config

3. **Runtime Manager Tests**
    - Test `ExecuteToolWithTimeout()` with various timeout values
    - Test backward compatibility of `ExecuteTool()`
    - Test timeout passing to worker request

### Integration Tests

1. **End-to-End Tool Timeout**

    - Create test tools with different timeout configurations
    - Verify each tool uses its specified timeout
    - Test tools that exceed their timeout fail appropriately

2. **Configuration Loading**
    - Test YAML parsing of tool timeout field
    - Test invalid timeout formats are caught during config loading

### Performance Tests

1. **Timeout Accuracy**
    - Verify tools actually respect their configured timeouts
    - Test that fast tools don't wait for long timeouts
    - Test that slow tools get their full timeout duration

## Backward Compatibility

### Existing Configurations

- Tools without `timeout` field continue to work unchanged
- Global timeout configuration remains as fallback
- CLI flags and environment variables still work for global timeout

### Migration Path

- No breaking changes required
- Teams can gradually add timeouts to tools that need them
- Rollback is simple (remove `timeout` field)

## Error Handling

### Invalid Timeout Formats

- Validation during config loading catches invalid formats
- Runtime falls back to global timeout if parsing fails
- Clear error messages guide users to correct format

### Timeout Examples

```yaml
timeout: 30s      # 30 seconds
timeout: 5m       # 5 minutes
timeout: 1h30m    # 1.5 hours
timeout: 2h       # 2 hours
```

## Benefits

1. **Performance Optimization**: Fast tools fail quickly, slow tools get adequate time
2. **Resource Management**: Prevents runaway processes from consuming resources
3. **User Experience**: Clear, intuitive configuration at the tool level
4. **Flexibility**: Per-tool configuration without affecting other tools
5. **Backward Compatibility**: Existing configurations continue to work

## Risks and Mitigations

### Risk: Tools configured with too short timeouts

**Mitigation**: Documentation with recommended timeout ranges, validation warnings

### Risk: Tools configured with excessively long timeouts

**Mitigation**: Optional maximum timeout limit in global config

### Risk: Confusion between task timeout and tool timeout

**Mitigation**: Clear documentation and different field names/contexts

## Future Enhancements

1. **Agent-Level Timeout**: Could add tool timeout override at agent level
2. **Dynamic Timeout**: Could allow timeout to be calculated based on input size
3. **Timeout Monitoring**: Could add metrics for timeout effectiveness
4. **Timeout Profiles**: Could define named timeout profiles (fast/medium/slow)

## Documentation Requirements

1. **User Guide**: How to configure tool timeouts
2. **Migration Guide**: How to update existing tools
3. **Best Practices**: Recommended timeout values for different tool types
4. **Troubleshooting**: Common timeout-related issues and solutions
