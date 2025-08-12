# 🔎 Deep Analysis Complete: Weather Workflow Template Evaluation Regression

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 📊 Summary

├─ Findings: 1 critical regression identified
├─ Critical: 1
├─ High: 0
├─ Medium: 0
└─ Low: 0

## 🧩 Finding #1: Premature Template Evaluation in Collection Normalizer

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📍 **Location**: `/Users/pedronauck/Dev/compozy/compozy/engine/task2/collection/normalizer.go:115`

⚠️ **Severity**: Critical

📂 **Category**: Architecture/Template Processing

### Root Cause:

The collection normalizer's `shouldSkipField()` function excludes specific fields from template processing but fails to exclude the `"tasks"` field. When `config.AsMap()` serializes a task configuration, it includes nested task arrays. The template engine then recursively processes ALL tasks in the serialized config instead of just the current task being normalized.

### Impact:

**Weather workflow fails** with error: `template: inline:1:9: executing "inline" at <.tasks.clothing.output.save_data.clothing>: map has no entry for key "clothing"`

The template engine prematurely evaluates `clothing_validation.items: "{{ .tasks.clothing.output.save_data.clothing }}"` during `activity_analysis` subtask execution, before the `clothing` task has run.

### Evidence:

- Error path: `"workflow -> config -> tasks -> array index 4 -> items"` maps to `taskConfig.Tasks[4].items`
- `Tasks[4]` is the `clothing_validation` task containing the problematic template reference
- Template engine's `ParseMapWithFilter` recursively processes the entire nested config structure
- Current `shouldSkipField()` excludes `"task"` (singular) but not `"tasks"` (plural array)

### Solution Strategy:

**Minimal One-Line Fix**: Add `"tasks"` to the exclusion list in `shouldSkipField()` function.

```go
// CURRENT (BROKEN):
func (n *Normalizer) shouldSkipField(k string) bool {
    return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
        k == "collection" || k == "items" || k == "filter" || k == "task"
}

// FIXED:
func (n *Normalizer) shouldSkipField(k string) bool {
    return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
        k == "collection" || k == "items" || k == "filter" || k == "task" || k == "tasks"
        //                                                                    ^^^^^^^^^^^^
}
```

### Related Areas:

- `/Users/pedronauck/Dev/compozy/compozy/engine/task/config.go` - Config struct with `Tasks []Config` field
- `/Users/pedronauck/Dev/compozy/compozy/pkg/tplengine/engine.go` - `ParseMapWithFilter` recursive processing
- `/Users/pedronauck/Dev/compozy/compozy/examples/weather/workflow.yaml` - Contains failing template

## 🔗 Dependency/Flow Map

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Critical Template Processing Flow:**

1. `exec_subtask.go` calls `normalizer.Normalize(taskConfig, normContext)`
2. `collection/normalizer.go` calls `config.AsMap()` to serialize task config
3. `task/config.go` serializes entire Config struct including `Tasks []Config` field
4. `collection/normalizer.go` calls `templateEngine.ParseMapWithFilter(configMap, context, shouldSkipField)`
5. `pkg/tplengine/engine.go` recursively processes all nested structures
6. **ISSUE**: `shouldSkipField("tasks")` returns `false`, so nested tasks array is processed
7. Template engine reaches `clothing_validation.items` and tries to resolve `{{ .tasks.clothing.output }}`
8. **FAILURE**: `clothing` task hasn't executed yet, so key doesn't exist

## 🌐 Broader Context Considerations (REQUIRED)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- **Reviewed Areas**: Config serialization, template engine processing, collection normalization, task inheritance, git diff analysis of context building changes, execution flow, subtask activities
- **Impacted Areas Matrix**:
  - Collection tasks → High impact → Critical risk → Immediate priority
  - Weather workflow → High impact → Critical risk → Immediate priority
  - Template processing scope → High impact → Critical risk → Immediate priority
- **Unknowns/Gaps**: None - exact root cause and fix identified
- **Assumptions**: Fix will not impact other normalizers since they use different shouldSkipField implementations

## 📐 Standards Compliance

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- **Rules satisfied**: @architecture.mdc (clean boundaries), @go-coding-standards.mdc (minimal change principle), @backwards-compatibility.mdc (preserves existing functionality)
- **Constraints considered**: Template processing isolation, normalization scope boundaries, task execution ordering
- **Deviations**: None - the fix aligns with existing architectural patterns and only prevents premature evaluation

## ✅ Verified Sound Areas

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- Template engine `ParseMapWithFilter` logic works correctly when proper fields are skipped
- Config serialization via `AsMap()` produces correct structure
- Context building in `shared/context.go` provides appropriate template variables
- Execution flow in `exec_subtask.go` follows correct normalization sequence
- Code-reviewer workflow demonstrates template processing works with explicit context

## 🎯 Fix Priority Order

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. **Immediate (Critical)**: Add `"tasks"` to `shouldSkipField()` function in `engine/task2/collection/normalizer.go`

**Exact Implementation:**

```go
// File: engine/task2/collection/normalizer.go
// Line: ~115

func (n *Normalizer) shouldSkipField(k string) bool {
    // Skip fields that need special handling
    return k == "agent" || k == "tool" || k == "outputs" || k == "output" ||
        k == "collection" || k == "items" || k == "filter" || k == "task" || k == "tasks"
}
```

**Impact Assessment:**

- ✅ Weather workflow: FIXED - `clothing_validation.items` won't be evaluated prematurely
- ✅ Code-reviewer workflow: CONTINUES WORKING - no changes to working functionality
- ✅ All workflows: SAFE - only prevents premature evaluation of nested task templates

This is a surgical, minimal fix that addresses the exact regression without side effects or architectural changes.

Returning control to the main agent. No changes performed.
