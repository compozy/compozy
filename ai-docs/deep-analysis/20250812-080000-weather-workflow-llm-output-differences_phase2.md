# 🔎 Deep Analysis Phase 2: Weather Workflow LLM Output Differences

## 📊 Initial Findings from RepoPrompt Analysis

The RepoPrompt analysis has identified the root cause of the weather workflow LLM output differences after the template fix. The issue is **NOT** a bug in the Groq API or model, but rather a systematic loss of per-iteration context data in collection tasks.

## 🧩 Critical Discovery: Context Variable Loss in Collection Tasks

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📍 **Location**: `engine/task2/collection/expander.go` - `injectCollectionContext` method
⚠️ **Severity**: Critical
📂 **Category**: Runtime/Logic

### Root Cause Analysis

The template engine changes have created an **unintentional loss of per-iteration data** in collection child tasks. The recent modifications to `ParseMapWithFilter` and variable building are preventing the proper resolution of `.item` and `.index` variables during template evaluation.

### Evidence Chain

1. **Before Changes**: Each collection iteration received proper `.item` context
   - Activity analysis tasks got unique activity names
   - Different mock weather conditions per activity
   - Varied, descriptive outputs from Groq

2. **After Changes**: Collection iterations receive identical or missing `.item` context
   - All activity analysis tasks receive same/empty item value
   - Identical mock weather conditions (60/25/Clear sky)
   - Generic, less descriptive outputs from Groq

### Technical Analysis

The problem stems from the interaction between two components:

#### 1. Collection Context Injection (`expander.go`)

```go
// Current problematic logic
if itemVar := parentConfig.GetItemVar(); itemVar != "" {
    withMap[itemVar] = item
}
```

**Issue**: When `GetItemVar()` returns a template string like `"{{ .some.path }}"` (due to unresolved references), it:

- Passes the `itemVar != ""` check
- Gets added to `withMap` as a literal template string
- Never gets resolved to actual item values during execution

#### 2. Variable Building (`variable_builder.go`)

```go
// Current logic in AddCurrentInputToVariables
if v, ok := (*currentInput)[shared.FieldItem]; ok {
    vars["item"] = v
}
```

**Issue**: When the `itemVar` is a template string, the lookup for `shared.FieldItem` ("item") fails because the actual key is the unresolved template string.

### Specific Prompt Impact

The LLM prompts are receiving:

```yaml
# Before (working)
prompt: "Mock validate activity 'Exploring Golden Gate Park' for weather conditions"

# After (broken)
prompt: "Mock validate activity '{{ .item }}' for weather conditions"
# OR
prompt: "Mock validate activity '' for weather conditions"
```

This explains why:

- Activities became less descriptive (model compensating for missing context)
- All validations return identical values (no specific activity context)
- Clothing recommendations became generic (no varied activity context)

## 🎯 Comprehensive Fix Strategy

Based on the RepoPrompt analysis, the fix requires:

### 1. Always Surface Standard Aliases ("item"/"index")

- Modify `injectCollectionContext` to guarantee canonical field names
- Ensure `.item` and `.index` are always available regardless of custom aliases

### 2. Force-Promote Canonical Keys in Variable Builder

- Update `AddCurrentInputToVariables` to surface standard keys even when collection uses internal field names
- Prevent dependency on alias presence

### 3. Add High-Fidelity Prompt Tracing

- Implement debug hooks to capture exact prompts sent to Groq
- Enable verification that templates are properly resolved

### 4. Ensure Task Output Re-evaluation

- Guarantee that `.tasks.weather.output` references are re-evaluated per iteration
- Prevent runtime placeholder blocking for legitimate context variables

## 🔗 Files Requiring Changes

1. **engine/task2/collection/expander.go** - Fix collection context injection
2. **engine/task2/shared/variable_builder.go** - Fix variable building logic
3. **pkg/tplengine/engine.go** - Add prompt debugging hooks
4. **engine/task2/shared/constants.go** (NEW) - Define canonical field names

## 🌐 Impact Assessment

- **Weather Workflow**: Will restore proper per-iteration context and varied LLM outputs
- **All Collection Tasks**: Will have guaranteed access to `.item` and `.index` variables
- **Template Resolution**: Will properly evaluate context variables while maintaining template fix benefits
- **Backward Compatibility**: Maintained through canonical + alias dual-publishing approach

## Next Phase Required

The analysis confirms that this is a **critical context propagation issue** requiring immediate code changes. The root cause is definitively identified and a comprehensive fix strategy is established.

Proceeding to comprehensive analysis and detailed fix implementation plan.
