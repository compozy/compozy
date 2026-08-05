# BUG-20260805-archived-detail-unarchive-missing: Archived detail cannot restore the session

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Cora
- **Journey Step:** J-archive-session-without-deleting, restore an archived session from its detail view
- **Scenarios:** RT-session-archive-catalog
- **Found:** 2026-08-05 · **Report:** `docs/qa/reports/2026-08-04-session-archive.md`

## Summary

An archived session detail correctly blocked Attach and Resume, but its topbar did not offer
Unarchive. A user who opened the preserved history had to leave the session and find it again in a
catalog before restoring it.

## Reproduction

- **Charter:** CH-archive-session-catalog · **Tour:** Feature Tour
- **Environment:** laptop / 1440x900 / wifi-fast / en-US

1. Archive a stopped session from the global catalog.
2. Open the archived session by its direct route.
3. Inspect the topbar actions.

**Expected:** Unarchive is the primary lifecycle action; Attach and Resume remain unavailable.
**Actual:** The topbar offered no lifecycle action for restoring the session.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/journey-log.jsonl`
- `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/session-catalog-desktop.png`

## Fix

- **Root cause:** The archived-detail branch removed Attach and Resume but did not replace them with the existing unarchive mutation in the session page controls and topbar slot.
- **Fix commit:** e40dc76.
- **Regression test:** `web/src/systems/session/hooks/__tests__/use-session-topbar-slot.test.tsx`.

## Verification

- **Retested:** 2026-08-05, same persona/journey · **Report:** `docs/qa/reports/2026-08-04-session-archive.md`
- **Result:** The archived detail rendered Unarchive, restored the same session, and never exposed Attach or Resume while the archive marker was present.
