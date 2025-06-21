---
status: pending
---

<task_context>
<domain>engine/workflow/schedule</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>none</dependencies>
</task_context>

# Task 7.0: Implement Override Tracking and Reload Hooks

## Overview

Add support for persisting API override state and triggering reconciliation on YAML reloads. This ensures API modifications persist until the next YAML change and that schedule updates are automatically applied when configurations change.

## Subtasks

- [ ] 7.1 Design and implement override tracking mechanism

    - Create in-memory cache for tracking API overrides
    - Store workflow ID, modification timestamp, and override values
    - Include YAML file modification time for comparison
    - Ensure thread-safe access with sync.RWMutex

- [ ] 7.2 Modify reconciliation to respect active overrides

    - Check override cache before applying YAML state
    - Skip reconciliation for workflows with recent API overrides
    - Clear overrides when YAML is newer than override timestamp
    - Log when overrides are preserved or cleared

- [ ] 7.3 Integrate with workflow configuration reload events

    - Subscribe to workflow configuration changes
    - Trigger reconciliation when YAML files are reloaded
    - Ensure reconciliation runs after successful config validation
    - Handle rapid reload events with debouncing

- [ ] 7.4 Add periodic reconciliation option

    - Implement configurable reconciliation interval (default: 5 minutes)
    - Run as background goroutine with same context handling
    - Ensure only one reconciliation runs at a time
    - Make interval configurable via environment variable

- [ ] 7.5 Write tests for override persistence and reload hooks
    - Test override cache behavior with concurrent access
    - Test reconciliation skips overridden schedules
    - Test YAML reload triggers reconciliation
    - Test periodic reconciliation timer

## Implementation Details

Override cache structure:

```go
type OverrideCache struct {
    mu sync.RWMutex
    overrides map[string]*Override
}

type Override struct {
    WorkflowID   string
    ModifiedAt   time.Time
    YAMLModTime  time.Time
    Values       map[string]interface{} // enabled, etc.
}

func (c *OverrideCache) ShouldSkipReconciliation(workflowID string, yamlModTime time.Time) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()

    override, exists := c.overrides[workflowID]
    if !exists {
        return false
    }

    // Skip if override is newer than YAML
    return override.ModifiedAt.After(yamlModTime)
}
```

## Success Criteria

- API overrides persist across reconciliations until YAML changes
- Configuration reloads automatically trigger schedule updates
- Override state is properly tracked and cleared when appropriate
- System handles concurrent operations safely
- Periodic reconciliation catches any drift

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
- **MUST** run `make lint` and `make test` before completing parent tasks
- **MUST** follow `.cursor/rules/task-review.mdc` workflow for parent tasks

**Enforcement:** Violating these standards results in immediate task rejection.
</critical>
