---
status: completed
---

<task_context>
<domain>engine/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>template_engine</dependencies>
</task_context>

# Task 16.0: Complete Template Engine Integration

## Overview

Implement template evaluation in `resolveMemoryKey` function by integrating with the existing `pkg/tplengine`. This addresses TODO comment at line 27 and enables dynamic memory keys with variable substitution.

## Subtasks

- [x] 16.1 Implement template evaluation using existing `pkg/tplengine`
- [x] 16.2 Add fallback to sanitization when template evaluation fails
- [x] 16.3 Add unit tests for template resolution with various contexts
- [x] 16.4 Add edge case handling (invalid templates, missing variables)

## Implementation Details

Update the `resolveMemoryKey` function in `engine/memory/config_resolver.go`:

```go
func (mm *Manager) resolveMemoryKey(
    ctx context.Context,
    keyTemplate string,
    workflowContextData map[string]any,
) (string, string) {
    // Use existing pkg/tplengine - no new dependencies needed
    result, err := mm.tplEngine.ProcessString(keyTemplate, workflowContextData)
    if err != nil {
        // Fall back to sanitizing the template as-is with warning
        mm.log.Warn("Failed to evaluate key template",
            "template", keyTemplate,
            "error", err)
        sanitizedKey := mm.sanitizeKey(keyTemplate)
        projectIDVal := extractProjectID(workflowContextData)
        return sanitizedKey, projectIDVal
    }

    // Sanitize the resolved key and extract project ID
    resolvedKey := result.Text
    sanitizedKey := mm.sanitizeKey(resolvedKey)
    projectIDVal := extractProjectID(workflowContextData)

    return sanitizedKey, projectIDVal
}

// Helper function to extract project ID from workflow context
func extractProjectID(workflowContextData map[string]any) string {
    if projectID, ok := workflowContextData["project.id"]; ok {
        if projectIDStr, ok := projectID.(string); ok {
            return projectIDStr
        }
    }
    return ""
}
```

**Key Implementation Notes:**

- Uses existing `pkg/tplengine.ProcessString` method
- Graceful fallback to sanitization on template errors
- Maintains existing sanitization and project ID extraction
- Follows established logging patterns

## Success Criteria

- ✅ Template evaluation works with complex template expressions
- ✅ Graceful fallback when templates are invalid or variables missing
- ✅ Unit tests cover various template scenarios and error cases
- ✅ Integration tests validate with real workflow context data
- ✅ No performance degradation in memory key resolution
- ✅ Follows established error handling and logging patterns

<critical>
**MANDATORY REQUIREMENTS:**

- **MUST** use existing `pkg/tplengine` - no new template implementations
- **MUST** provide graceful fallback for template errors
- **MUST** maintain backward compatibility with existing key patterns
- **MUST** include comprehensive test coverage for template scenarios
- **MUST** run `make lint` and `make test` before completion
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for completion
  </critical>
