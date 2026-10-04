# BUG-20261004-attention-deleted-workspace-stale: Notifications retain a deleted project until reload

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-administer-runtime-settings, retire a muted project
- **Scenarios:** MS-attention-settings-roundtrip
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora removes a muted project through the CLI while Notifications remains open. HTTP and UDS
confirm its mute was removed from both profiles, but the open page retains a row displaying the
deleted workspace ID. Reload removes that row and preserves the other muted project.

## Reproduction

- **Charter:** CH-herdr-attention-settings · **Tour:** Multi-Tab Tour
- **Environment:** isolated real daemon, desktop Chrome 1512 × 862, en-US

1. Mute Archive desk in default and resume-editorial; also mute Review desk in resume-editorial.
2. Keep Notifications open under resume-editorial and remove Archive desk through the CLI.
3. Fresh-read both profiles through HTTP/UDS: default is empty, resume-editorial retains only Review desk.
4. Inspect the still-open page: it retains ws_6c24a6f1c0f171bc alongside Review desk.
5. Reload: only Review desk remains.

**Expected:** The active policy view reconciles the removed workspace without keeping its obsolete mute.
**Actual:** Workspace labels refresh, exposing the removed ID, while the mute list remains stale.

## Evidence

Receipts under docs/qa/evidence/2026-10-02-untested/:
- attention-mutes-dora-remove-archive.json
- attention-mutes-dora-default-pruned.json and attention-mutes-dora-editorial-pruned.json
- attention-mutes-dora-pruned-and-restored-web.json: MUTE_BUTTONS_BEFORE_RELOAD includes the deleted ID
- attention-mutes-dora-pruned.png: refreshed policy retains only Review desk
- attention-mutes-dora-ended.json: restoration and 19-frame recording closure

Both temporary registrations are removed after the walk. Original global values and empty mute
sets are restored. No later write from the stale view was attempted, so a write failure is not claimed.

## Fix

- **Root cause:** Pending diagnosis after the persona session.
- **Fix commit:** Pending.
- **Regression test:** Pending owning-layer identification.

## Verification

Pending repair and a fresh original-persona replay of external deletion and live reconciliation.
