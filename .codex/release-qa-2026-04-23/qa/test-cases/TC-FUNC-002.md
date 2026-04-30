## TC-FUNC-002: Docs And Config Examples Match Runtime Defaults

**Priority:** P1 (High)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 20 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:** `bun run test:config`, Go doc fixture tests, and `rg "gpt-5\\.4"`
**Automation Notes:** Active docs, bundled skill references, config examples, and help fixtures must not advertise the old default after the source change.

### Objective

Verify release-facing documentation and generated/help fixtures describe `gpt-5.5` for Codex/Droid defaults.

### Preconditions

- Active docs are in README, `docs/`, `skills/compozy/references/`, CLI help fixtures, and example agents.

### Test Steps

1. Search active docs/config/help surfaces for `gpt-5.4`.
   **Expected:** Only intentionally explicit or non-default references remain; active default examples use `gpt-5.5`.
2. Run config/documentation tests.
   **Expected:** Tests pass with updated expected output.
3. Inspect README and bundled skill config examples.
   **Expected:** Default model examples and per-IDE default text are consistent with runtime behavior.

### Edge Cases

| Variation              | Input                                       | Expected Result                                      |
| ---------------------- | ------------------------------------------- | ---------------------------------------------------- |
| Explicit override docs | User sets `model = "gpt-5.4"` intentionally | Text clarifies it is an override, not the default    |
| Historical docs        | `docs/plans/**`                             | Out of scope unless referenced by current docs/tests |
