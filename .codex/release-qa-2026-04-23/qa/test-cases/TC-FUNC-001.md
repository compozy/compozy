## TC-FUNC-001: Codex Default Model Uses GPT-5.5

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 20 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** Integration
**Automation Status:** Missing
**Automation Command/Spec:** Focused Go tests under `internal/core/agent`, `internal/core/model`, and `internal/cli`
**Automation Notes:** Existing tests assert older `gpt-5.4` examples. They must be updated to lock the new default and ensure runtime behavior works.

### Objective

Verify Codex and Droid built-in runtime defaults resolve to `gpt-5.5` when no explicit model is supplied, while explicit model/config values still override correctly.

### Preconditions

- Source constant and registry specs are available.
- No live LLM call is required for default resolution proof.

### Test Steps

1. Inspect `model.DefaultCodexModel`.
   **Expected:** Constant value is `gpt-5.5`.
2. Resolve runtime model for `codex` with an empty model.
   **Expected:** Result is `gpt-5.5`.
3. Build/resolve Droid launch defaults with an empty model.
   **Expected:** Droid bootstrap `--model` argument is `gpt-5.5`.
4. Resolve runtime model with explicit `--model custom-model`.
   **Expected:** Explicit model remains `custom-model`.
5. Run focused Go tests covering these paths.
   **Expected:** Tests pass and fail if the default regresses to `gpt-5.4`.

### Edge Cases

| Variation           | Input       | Expected Result                               |
| ------------------- | ----------- | --------------------------------------------- |
| Whitespace model    | `"   "`     | Falls back to `gpt-5.5`                       |
| Alias model overlay | `ext-model` | Resolves through `modelprovider.ResolveAlias` |
| Explicit old model  | `gpt-5.4`   | Preserved as explicit override                |
