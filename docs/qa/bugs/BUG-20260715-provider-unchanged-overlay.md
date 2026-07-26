# BUG-20260715-provider-unchanged-overlay: Unchanged builtin provider save creates a restart-required overlay

- **Status:** verified
- **Impact (user-side):** Turns a read-only canary into a false restart requirement and shadows builtin provider defaults
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-22, unchanged provider GET → PUT
- **Scenarios:** MS-028
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-15-marketplace.md
- **Origin:** Task 11 adjacent Marketplace canary

## Summary

Saving the Claude provider editor without changing a field materialized the complete builtin provider as a global config overlay. The UI changed the provider source from `BUILTIN` to `CONFIG` and required a daemon restart even though the operator expressed no new intent.

## Reproduction

1. Open Settings → Providers on a clean home and inspect the builtin Claude provider.
2. Select **Edit settings** and submit the unchanged form.
3. Re-read the provider source and compare `config.toml` before and after.

**Expected:** The operation is reported as an applied no-op, writes no config bytes, emits no `settings.changed` event, does not invoke the runtime applier, keeps source `BUILTIN`, and requires no restart.
**Actual:** The full projected provider was written to `config.toml`, source became `CONFIG`, and the UI reported `restart required to apply`.

## Evidence

- Mobile red/green replay: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/ch-033-provider-noop.json`.
- Green screenshot: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/ch-033-provider-noop-green.png`.

## Fix

- **Root cause:** Provider mutation classification compared raw resolved structs and had no semantic no-op path. Even when a projected GET matched the effective builtin provider, the writer materialized a new overlay; the config-apply path then treated the result as a live model update.
- **Correction:** Provider writes are classified from normalized effective settings plus raw curated intent. An exact round trip returns the existing `no changes` lifecycle signal, skips config editing and `settings.changed`, bypasses model projection, and records a skipped apply result.
- **Fix commit:** pending Phase D checkpoint
- **Regression tests:** The provider projection owner proves config bytes and builtin source are unchanged; the config-apply owner proves a skipped successful result with zero runtime-applier calls and zero change events.

## Verification

- Both regressions failed before their owning correction.
- `go test -race ./internal/settings -count=1` passed 178 tests.
- The rebuilt-daemon mobile replay returned `Saved provider "claude" · applied immediately`, kept `SOURCE=BUILTIN`, and preserved config SHA-256 `6c29c64322f9c896762e083e15ae91ed577acb3a38266f5ff98f3a63d9167794` byte-for-byte.
