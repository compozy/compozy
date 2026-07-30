# BUG-20260730-goal-outbox-config-rejected: Goal outbox controls were not writable

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-28 configure durable Goal relay, step 1
- **Scenarios:** TA-092
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Bruno could not write the documented Goal outbox batch size or poll interval through the CLI config surface.

## Reproduction

- **Charter:** CH-goal-config-lifecycle · **Tour:** Feature Tour
- **Environment:** isolated current-source daemon / CLI / en-US

1. Set `goals.outbox_batch_size` and `goals.outbox_poll_interval` globally.
2. Read both paths back.

**Expected:** Typed values persist and report restart-required lifecycle.
**Actual:** The pre-fix CLI rejected both paths as unsupported.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** The config writer registry omitted the two already-supported Goal outbox leaves.
- **Fix commit:** bd0617c
- **Regression test:** `internal/cli/config_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** Both values round-trip and report `restart-required`.
