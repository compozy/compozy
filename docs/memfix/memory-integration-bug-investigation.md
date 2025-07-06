# Memory Integration Bug Investigation

## Problem Statement

Memory saved via REST API cannot be retrieved by agents during workflow execution. The bug manifests as:

- Memory written via REST API with key `"user:api_test_user"` works correctly
- Agents with memory_key template `"user:{{.workflow.input.user_id}}"` fail to retrieve this memory
- Agent always responds with "new conversation" despite memory being present

## Root Cause Analysis

### Initial Hypothesis

The memory key template resolution was failing, causing agents to look up memory with the wrong key.

### Key Discoveries

1. **Double Resolution Issue**
    - Found that `MemoryResolver.GetMemory()` was resolving the template and setting `ResolvedKey`
    - Then `Manager.GetInstance()` was checking if `ResolvedKey` was already set and using it directly
    - This created a double resolution problem where the key was being processed twice

2. **Hash Mismatch**
    - REST API stores memory with SHA-256 hash of `"user:api_test_user"` = `be58fcafcd893d3da441086bc5f8e230dc9110e442e17f75cc3393d92dc238c5`
    - Agent was potentially looking up with hash of template string `"user:{{.workflow.input.user_id}}"` = `a8faba2e8e327f8c1bb40ff6a7cd05abe37c832ef3b1ee3173cca187fa972dc2`
    - These hashes don't match, so memory retrieval fails

3. **Template Engine Availability**
    - Initially suspected `templateEngine` might be nil in some contexts
    - Added fail-fast error handling to catch this case
    - However, tracing showed template engine is properly initialized in the worker

## Code Flow Traced

### REST API Flow (Working)

1. Handler receives request with explicit key: `"user:api_test_user"`
2. `operations.go:622` sets `ResolvedKey: key` directly
3. Memory is stored with SHA-256 hash of the explicit key

### Workflow Agent Flow (Broken)

1. Agent config has `memory_key: "user:{{.workflow.input.user_id}}"`
2. `agent/config.go` creates `MemoryReference` with `Key: "user:{{.workflow.input.user_id}}"`
3. `MemoryResolver.GetMemory()` attempts to resolve template
4. `Manager.GetInstance()` is called with the MemoryReference
5. `Manager.resolveMemoryKey()` checks if `ResolvedKey` is set
6. Key resolution happens in wrong place or twice

## Fixes Applied

### 1. Fail-Fast for Nil Template Engine

Added checks in both `memory_resolver.go` and `config_resolver.go`:

```go
if r.templateEngine == nil {
    if strings.Contains(keyTemplate, "{{") {
        return "", fmt.Errorf("template engine is required to resolve key template: %s", keyTemplate)
    }
    return keyTemplate, nil
}
```

### 2. Remove Double Resolution

Modified `MemoryResolver.GetMemory()` to not resolve the template, letting the Manager handle it:

```go
// Before: Setting ResolvedKey caused double resolution
memRef := core.MemoryReference{
    ID:          memoryID,
    Key:         keyTemplate,
    ResolvedKey: resolvedKey,  // REMOVED THIS
    Mode:        "read-write",
}
```

### 3. Enhanced Debug Logging

Added extensive logging throughout the resolution chain to trace the exact flow and identify where resolution fails.

## Current Status

Despite the fixes:

- Integration test still fails
- Manual testing shows agent still can't retrieve memory
- The core issue persists

## Next Steps

1. **Verify Template Resolution**: Ensure the workflow context structure matches what the template expects
2. **Check Manager's Template Engine**: Verify the Manager has access to a properly configured template engine
3. **Trace Hash Generation**: Add logging to see exactly what string is being hashed in both REST and workflow contexts
4. **Consider Alternative Approaches**:
    - Maybe the MemoryResolver should keep its resolution logic but pass the resolved key differently
    - Or ensure both components use the same resolution approach

## Test Results

Created comprehensive E2E test in `/test/integration/api/memory_workflow_e2e_test.go` that:

- ✅ Clears memory successfully
- ✅ Confirms agent recognizes empty memory
- ✅ Writes memory via REST API
- ✅ Reads memory back via REST API
- ❌ Agent cannot retrieve memory during workflow
- ❌ Memory doesn't persist across workflow executions

The test confirms the bug and will prevent regression once fixed.
