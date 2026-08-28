# BUG-20260828-provider-model-fast-capability: Curation accepted Fast without a provider binding

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-20, curate a truthful model default
- **Scenarios:** MS-054
- **Found:** 2026-08-28 · **Report:** docs/qa/reports/2026-08-28-cursor-onboarding-runtime-defaults.md

## Summary

Provider model curation accepted `default_speed = "fast"` for a live Cursor model whose physical
bindings contained no Fast variant. The saved default could therefore request a transport mode the
provider did not advertise.

## Reproduction

1. Refresh the live Cursor model catalog.
2. Run `compozy provider models set cursor gemini-3-flash --default-speed fast -o json`.
3. Read the provider configuration.

**Expected:** Curation returns `speed_rejected` without writing or applying configuration.
**Actual before the fix:** Curation applied the change and persisted `default_speed = "fast"`.

## Evidence

- Fresh isolated runtime at `http://127.0.0.1:53552`.
- `gemini-3-flash` exposed one physical binding with no Fast dimension, but the CLI returned an
  applied generation with `default_speed: fast`.

## Fix

- **Root cause:** Fast validation treated physical bindings with an absent Fast dimension as
  unknown support. For a live binding, absence means the binding is the normal variant; only an
  explicit `fast=true` binding advertises Fast.
- **Fix:** Preserve permissive behavior only when a catalog model has no physical bindings. Once
  bindings exist, require at least one explicit Fast binding.
- **Fix commit:** pending; included in the PR #498 follow-up commit
- **Regression test:** `Should reject Fast when the model advertises only normal configurations`
  in `internal/settings/config_apply_service_test.go` now uses a physical binding without Fast.

## Verification

- **Retested:** 2026-08-28 in a new isolated lab after rebuilding the daemon.
- **Result:** pass — `gemini-3-flash` Fast returned exit 71 with `speed_rejected`, raw config remained
  unchanged, and Grok 4.6 continued to accept xhigh/Fast across CLI/UDS, HTTP, and native curation.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/notes/cursor-defaults-retest-evidence.md`
