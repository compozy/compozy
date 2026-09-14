# BUG-20260914-agent-plugin-dev-rejected: Valid Agent Plugins cannot enter the CLI development loop

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-dev-lifecycle, link an authored package
- **Scenarios:** ET-agent-plugin-dev-reload
- **Report:** docs/qa/reports/2026-09-14-marketplace-review-public.md

## Reproduction

CH-marketplace-public-ownership, Data Tour: create a Claude plugin with a skill and a manifest name different from its directory. Run `extension validate <directory>` (valid), then `extension dev <directory> --workspace design` (rejected with `resource-only authoring requires a native Compozy manifest`). The same package installs through Marketplace successfully.

Evidence: `docs/qa/evidence/2026-09-14-marketplace-review-public/step-055.json` and `step-056.json`. The persona session stopped at this rejected link; no patched result is attributed to that attempt.

## Fix

The CLI always invokes the native bundle builder before linking or reloading. In addition, development generation classification only checks root plugin.json, omitting supported client layouts. Use one shared development preparation path that preserves native immutable bundles and validates portable source generations; reuse the securely read client manifest for identity and loading.

Owning invariant: all supported layouts use the authored identity through development preparation, activation and reload, preserving the last good generation after an invalid change. Owning suite: `TestManagerDevelopmentLifecycle` in `internal/extension/dev_integration_test.go`.

Focused race-enabled real SQLite lifecycle cases passed for all four layouts (2.532s); public re-walk and current-head CI remain pending.
