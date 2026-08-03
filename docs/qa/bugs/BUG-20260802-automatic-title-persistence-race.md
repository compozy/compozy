# BUG-20260802-automatic-title-persistence-race: Session title became public before durable identity

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-17, generate and reload an automatic session title
- **Scenarios:** RT-session-auto-title
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 runtime E2E

## Summary

The active-session catalog could expose a generated title before `meta.json` and the global session
catalog finished persisting it. HTTP and UDS list reads showed the title while a direct metadata read
still returned an empty name.

## Reproduction

1. Create an unnamed user session and complete the first meaningful assistant response.
2. Block global session-catalog registration while automatic-title persistence is in progress.
3. Read the active session title and its durable metadata concurrently.

**Expected:** Public reads wait until the full session identity is durable.
**Actual:** The in-memory name changed first, so list projections could publish the title early.

## Evidence

- Failing full runtime E2E: `TestUDSTransportAutomaticSessionTitlePersistsAndMatchesHTTP` observed
  catalog title `Checkout Retry Fencing` while session metadata still had an empty name.
- A deterministic manager regression reproduced the visibility window before the correction.

## Fix

- **Root cause:** `ApplyAutomaticSessionTitle` mutated the live `Session.Name` before metadata and
  catalog persistence completed.
- **Correction:** The session write lock now spans snapshot persistence; failures roll back memory
  and metadata before reads resume, and the event publishes only after the committed snapshot.
- **Fix commit:** `855b273`
- **Regression test:** `TestApplyAutomaticSessionTitleOwnsGeneratedIdentity` blocks catalog
  registration and proves a concurrent title read cannot observe the uncommitted identity.

## Verification

- The canonical manager regression and complete `internal/session` package passed under `-race`.
- The focused UDS parity test passed five consecutive `-race` runs.
- The complete runtime E2E lane passed after the correction.
