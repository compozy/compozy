# BUG-20260713-task-owner-cannot-clear: Web task edit cannot clear an exact-session owner

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-complete-task-tree release a Ready child to the worker pool
- **Scenarios:** TA-004; TA-parent-rollup-completion
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md

## Summary

The Edit task modal allows an exact-session owner to be changed to `Unassigned`, disables the owner reference, and closes after Save changes. The mutation does not clear the persisted owner: both the detail page and a fresh Edit task read still show `Exact session / sess-b1c980b86709053d`. No error or unchanged-state explanation appears.

The structured HTTP/UDS contract already has an explicit `clear_owner` path, so this is a Web mutation-projection failure rather than an unsupported runtime operation.

## Reproduction

1. Open Ready task `task-f6638f9897b1b0f8`, owned by exact session `sess-b1c980b86709053d`.
2. Choose Edit.
3. Change Owner kind from Exact session to Unassigned.
4. Confirm that Owner reference becomes disabled, then choose Save changes.
5. Inspect the detail and reopen Edit.

**Expected:** The PATCH expresses the owner-clear operation, the detail shows Owner Unassigned, and an eligible task-role worker can claim the queued run.
**Actual:** Save closes without error, but the exact-session owner remains persisted on every fresh read.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-owner-clear-not-persisted.dom.txt`
- Detail after Save still rendered `Owner sess-b1c980b86709053d`; fresh Edit rendered `Exact session` selected with the same reference.

## Fix

- **Root cause:** Selecting `Unassigned` changed the owner discriminator but retained the stale reference, and the edit request builder omitted both `owner` and the runtime's explicit `clear_owner` operation. The PATCH therefore represented no ownership change.
- **Resolution:** Owner-kind changes clear incompatible references, and the edit projection now emits `clear_owner: true` whenever `Unassigned` is authoritative while omitting the stale owner union.
- **Fix commit:** pending
- **Regression test:** The canonical Task edit route suite covers clear persistence on a fresh read, non-clear reassignment, cancel, and retry after PATCH failure; the Task editor projection suite asserts every supported owner union and explicit clearing.

## Verification

- **Passed 2026-07-13:** Bruno changed child `task-f6638f9897b1b0f8` from Exact session to Unassigned. The fresh detail rendered `Owner Unassigned`, and a separately reopened Edit modal kept `Unassigned` selected with a disabled, empty owner reference. The same replay passed on child `task-a090a4e5ba779d61`.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-owner-clear-fixed-detail.dom.txt`, `ch-task-owner-clear-fixed-fresh-edit.dom.txt`, and `ch-task-owner-clear-second-child-fixed.dom.txt`.
- The subsequent real task-role activation/AGH-71 rollup is tracked independently; it is no longer blocked by owner clearing.
