# 🔎 Deep Analysis Complete: Weather Workflow Template Evaluation Regression (Phase 2)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 📊 RepoPrompt Analysis Summary

├─ Critical Issue Identified: Premature Template Evaluation
├─ Root Cause: Template engine logic flaw in pkg/tplengine/engine.go
├─ Impact: Weather workflow fails, blocking production deployment
└─ Solution: Surgical fix in template decision logic

## 🧩 Core Finding: Template Engine Logic Flaw

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📍 **Location**: `/Users/pedronauck/Dev/compozy/compozy/pkg/tplengine/engine.go` - Template evaluation decision logic

⚠️ **Severity**: Critical

📂 **Category**: Runtime/Logic

### Technical Root Cause:

During config normalization, `tplengine.TemplateEngine.ParseMapWithFilter` walks every field value. When it encounters a string containing `.tasks.`, the current logic decides whether to execute the template immediately or defer it:

```go
if HasTemplate(v) && containsRuntimeReferences(v) {
    if data != nil {
        if tasksVal, ok := data["tasks"]; ok && tasksVal != nil {
            // <-- CURRENT FLAWED LOGIC: ALWAYS falls through to full parsing
        } else {
            return v, nil          // keep placeholder
        }
    } else {
        return v, nil
    }
}
```

**The Flaw**: `tasksVal` is a non-nil (but incomplete) map for every task that has already run. The condition is satisfied even though the particular reference (`.tasks.clothing`) is missing. The template engine is invoked, cannot find the key "clothing", and throws the error.

### Why shouldSkipField Didn't Work:

- `shouldSkipField` is evaluated on the **key** of the map currently being processed (`"items"`).
- It correctly signals "skip", so the key/value pair is copied without further recursion.
- However, `items` lives **inside another map** - the global traversal reaches that deeper level again (via the parent `config.tasks[*]` field) where the key is no longer `"items"` but a generic string value, so the skip rule no longer applies.

## 🎯 Solution Strategy: Surgical Template Logic Fix

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

### Target Behavior:

Only attempt to render `.tasks.*` expressions when **all referenced task keys exist in the current `tasks` map**. Otherwise leave the raw string for later evaluation.

### Implementation Plan:

**File to Modify**: `pkg/tplengine/engine.go`

**Step 1**: New helper to extract task IDs from template strings:

```go
// returns every literal ".tasks.<id>" fragment it finds
var taskRefRx = regexp.MustCompile(`\.tasks\.([a-zA-Z0-9_-]+)`)

func referencedTaskIDs(s string) []string {
    matches := taskRefRx.FindAllStringSubmatch(s, -1)
    ids := make([]string, 0, len(matches))
    for _, m := range matches {
        if len(m) == 2 {
            ids = append(ids, m[1])
        }
    }
    return ids
}
```

**Step 2**: Enhanced decision logic in `ParseMapWithFilter`:

```go
if HasTemplate(v) && containsRuntimeReferences(v) {
    if data != nil {
        if tasksVal, ok := data["tasks"]; ok && tasksVal != nil {
            tv, okMap := tasksVal.(map[string]any)
            if okMap {
                // NEW: ensure every referenced task id is present
                allResolved := true
                for _, id := range referencedTaskIDs(v) {
                    if _, exists := tv[id]; !exists {
                        allResolved = false
                        break
                    }
                }
                if !allResolved {
                    return v, nil // defer evaluation – outputs not ready
                }
            } else {
                // tasksVal is not a map, treat as unresolved
                return v, nil
            }
            // fall through → render now (all ids found)
        } else {
            return v, nil
        }
    } else {
        return v, nil
    }
}
```

### Benefits:

- **No Breaking Changes**: Workflows with available task outputs still render normally
- **Fixes Weather Workflow**: Templates referencing future tasks remain placeholders until ready
- **Architectural Integrity**: Single-pass, deterministic template evaluation
- **Minimal Surface Area**: Only internal decision logic changes

With this patch, `clothing_validation.items: "{{ .tasks.clothing.output.save_data.clothing }}"` will remain the raw template string until after the `clothing` task finishes, eliminating the premature evaluation error.

Returning control to the main agent for comprehensive analysis completion.
