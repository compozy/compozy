# 🔎 Deep Analysis Complete: Weather Workflow Template Resolution Failure

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

## 📊 Summary

├─ Findings: 2 total
├─ Critical: 2
├─ High: 0
├─ Medium: 0
└─ Low: 0

## 🧩 Finding #1: Template Engine Type Check Failure

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 **Location**: pkg/tplengine/engine.go:317-331 (canResolveTaskReferencesNow method)
⚠️ **Severity**: Critical
📂 **Category**: Runtime/Logic

**Root Cause:**
The `canResolveTaskReferencesNow()` method in the template engine only accepts `map[string]any` types but fails when the tasks context contains pointer types like `*core.Output` or `*map[string]any`. The type assertion `tasksVal.(map[string]any)` returns false for pointer types, causing templates to be deferred unnecessarily.

**Impact:**
Templates like `{{ .tasks.weather.output }}` fail to resolve during normalization phase and are deferred until execution time. When they finally attempt resolution, the shared pointer references cause all child tasks to receive identical context mutations.

**Evidence:**

```go
// Line 326-328 in pkg/tplengine/engine.go
tasksMap, isMap := tasksVal.(map[string]any)
if !isMap {
    return false  // Fails on pointer types!
}
```

**Solution Strategy:**
Extend type assertion to handle pointer types:

```go
func (e *TemplateEngine) canResolveTaskReferencesNow(v string, data map[string]any) bool {
    if data == nil {
        return false
    }
    tasksVal, ok := data["tasks"]
    if !ok || tasksVal == nil {
        return false
    }

    var tasksMap map[string]any
    switch t := tasksVal.(type) {
    case map[string]any:
        tasksMap = t
    case *map[string]any:
        if t != nil {
            tasksMap = *t
        } else {
            return false
        }
    case *core.Output:
        if t != nil {
            tasksMap = *t
        } else {
            return false
        }
    case *core.Input:
        if t != nil {
            tasksMap = *t
        } else {
            return false
        }
    default:
        return false
    }

    referenced := extractTaskReferences(v)
    return areAllTasksAvailable(referenced, tasksMap)
}
```

**Related Areas:**

- engine/task2/collection/expander.go (context injection)
- engine/core/params.go (type definitions)
- engine/task2/shared/context.go (context building)

## 🧩 Finding #2: Collection Context Shared Pointer Mutation

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 **Location**: engine/task2/collection/expander.go:166-200 (injectCollectionContext method)
⚠️ **Severity**: Critical  
📂 **Category**: Concurrency/Memory

**Root Cause:**
The `injectCollectionContext()` method creates shallow copies when building child task contexts. When parent context contains pointer maps (`*core.Input`, `*core.Output`), all children receive references to the same underlying memory locations. When templates finally resolve and mutate these maps, changes are visible across all sibling tasks.

**Impact:**
All collection child tasks end up with identical outputs because template mutations are shared. The database investigation revealed that inputs are correctly resolved per child (18°C, 76% humidity), but all tasks output the same values (24°C, 61% humidity).

**Evidence:**

```go
// Line 180-185 in engine/task2/collection/expander.go
withMap := map[string]any(*childConfig.With)
for k, v := range *parentConfig.With {
    withMap[k] = v  // Creates shared pointer references
}
```

**Solution Strategy:**
Implement deep copy for collection context injection:

```go
func (ce *CollectionExpander) injectCollectionContext(
    childConfig *task.Config,
    parentConfig *task.Config,
    item any,
    index int,
) error {
    if childConfig.With == nil {
        childConfig.With = &core.Input{}
    }

    // Deep copy existing child context
    withMap, err := ce.deepCopyMap(map[string]any(*childConfig.With))
    if err != nil {
        return fmt.Errorf("failed to deep copy child context: %w", err)
    }

    // Deep copy parent context if available
    if parentConfig.With != nil {
        parentMap, err := ce.deepCopyMap(*parentConfig.With)
        if err != nil {
            return fmt.Errorf("failed to deep copy parent context: %w", err)
        }

        for k, v := range parentMap {
            withMap[k] = v
        }
    }

    // Add collection-specific variables
    withMap["item"] = item
    withMap["index"] = index

    newWith := core.Input(withMap)
    childConfig.With = &newWith
    return nil
}

func (ce *CollectionExpander) deepCopyMap(src map[string]any) (map[string]any, error) {
    dst := make(map[string]any)
    for k, v := range src {
        switch val := v.(type) {
        case map[string]any:
            copied, err := ce.deepCopyMap(val)
            if err != nil {
                return nil, err
            }
            dst[k] = copied
        case *map[string]any:
            if val != nil {
                copied, err := ce.deepCopyMap(*val)
                if err != nil {
                    return nil, err
                }
                dst[k] = &copied
            } else {
                dst[k] = val
            }
        default:
            dst[k] = val
        }
    }
    return dst, nil
}
```

**Related Areas:**

- engine/task2/collection/context_builder.go (context creation)
- engine/task2/shared/context.go (context management)
- pkg/tplengine/engine.go (template processing)

## 🔗 Dependency/Flow Map

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Critical Flow Chain:**

1. **Collection Expansion** (engine/task2/collection/expander.go)
   - Creates child tasks with shared pointer contexts
   - Injects `item` and `index` variables via shallow copy

2. **Template Processing** (pkg/tplengine/engine.go)
   - Type check fails on pointer types → templates deferred
   - Templates eventually resolve but mutate shared memory

3. **LLM Execution** (engine/llm/orchestrator.go)
   - Receives already-rendered prompts with shared mutations
   - All children see identical template resolution results

4. **Database Storage** (engine/infra/store/taskrepo.go)
   - Stores correct inputs per child (✅ working)
   - Stores identical outputs across children (❌ shared mutations)

## 🌐 Broader Context Considerations

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Reviewed Areas:**

- ✅ Template engine resolution logic (pkg/tplengine/engine.go)
- ✅ Collection expansion and context injection (engine/task2/collection/\*)
- ✅ Database task state storage and retrieval (engine/infra/store/taskrepo.go)
- ✅ Context building and variable management (engine/task2/shared/context.go)
- ✅ Workflow configuration structure (examples/weather/workflow.yaml)
- ✅ LLM prompt processing pipeline (engine/llm/orchestrator.go)

**Impacted Areas Matrix:**

- **Template Resolution**: Critical impact → requires pointer type handling
- **Collection Processing**: Critical impact → requires deep copy implementation
- **LLM Execution**: Medium impact → receives corrupted context but works correctly
- **Database Storage**: Low impact → correctly stores inputs/outputs as received
- **Workflow Configuration**: No impact → YAML structure is correct

**Unknowns/Gaps:**

- Performance impact of deep copying large context objects
- Memory usage implications of duplicating context data per child
- Potential race conditions in concurrent collection processing

**Assumptions:**

- Collection child tasks should receive isolated context copies
- Template resolution should happen during normalization when possible
- Pointer type handling is acceptable in template engine
- Deep copying performance is acceptable for collection sizes

## 📐 Standards Compliance

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**Rules satisfied:**

- @go-coding-standards.mdc: Error handling patterns followed
- @architecture.mdc: Clean separation of concerns maintained
- @test-standard.mdc: Existing test patterns should be preserved

**Constraints considered:**

- Context-first design: Solutions maintain context propagation patterns
- DI patterns: No dependency injection changes required
- Error handling: Both solutions include proper error handling
- Memory management: Deep copy solution includes cleanup considerations

**Deviations (if any):**
None identified - solutions align with existing architectural patterns and coding standards.

## ✅ Verified Sound Areas

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

- **Database Layer**: Task state storage/retrieval works correctly - inputs are properly resolved per child
- **LLM Processing**: Orchestrator correctly processes rendered prompts - issue is upstream
- **Workflow Configuration**: YAML structure and template syntax are correct
- **Context Variable Building**: Base context creation logic is sound
- **Template Syntax**: Collection templates like `{{ .tasks.weather.output }}` are valid

## 🎯 Fix Priority Order

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. **Fix Template Engine Type Check** (Critical)
   - Update `canResolveTaskReferencesNow()` to handle pointer types
   - Enables proper template resolution during normalization
   - Prevents deferred resolution that triggers shared mutation

2. **Implement Collection Context Deep Copy** (Critical)
   - Add deep copy logic to `injectCollectionContext()`
   - Ensures each child gets isolated context memory
   - Prevents shared pointer mutations between siblings

## 📋 Phase 2 Investigation Summary

The Phase 2 analysis using RepoPrompt revealed a crucial discovery that **changed the entire diagnosis**:

**Previous Assumption (Incorrect):** Templates were not resolving at all due to template engine failures.

**Actual Finding (Correct):** Templates ARE resolving correctly and child tasks receive proper weather data in their inputs. The issue is that when templates finally resolve (after being deferred due to type check failures), they mutate shared pointer references, causing all children to output identical values despite receiving different inputs.

**Database Evidence:**

- ✅ Child task inputs: Unique weather data (18°C, 76% humidity)
- ❌ Child task outputs: Identical LLM responses (24°C, 61% humidity)

This discovery shifted the investigation from "template resolution failure" to "shared context mutation" - a much more subtle and dangerous bug that explains why all collection children produce identical outputs despite real LLM calls.

Returning control to the main agent. No changes performed.
