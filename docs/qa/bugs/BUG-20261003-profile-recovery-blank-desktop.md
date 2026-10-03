# BUG-20261003-profile-recovery-blank-desktop: An unavailable profile leaves Settings blank with a generic retry notice

- **Status:** open
- **Fix commit:** pending
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, inspect a failed lifecycle operation from Settings
- **Scenarios:** ET-profile-operations-recovery
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-lifecycle-plan-recovery, isolated profile-recovery lab, b915570a8 plus initial
audit/identity fixes, desktop 1512x862, en-US, browser-use:

1. Remember recovery-drafts globally, then rename it with a conflicting imported destination.
2. The catalog now names recovery-published, with its failed operation reserving the owner.
3. Open /settings/profiles in a fresh browser tab.
4. The dock shows recovery-published, but Settings never opens. A blank desktop only says
   "Can't save window layout — retrying"; it does not identify the profile reservation or its remedy.
5. Correct the directory conflict and explicitly retry through CLI. Reload now opens Settings.

Expected: the client reports the unavailable owner and a useful recovery path; it must not turn
a durable lifecycle failure into an unexplained layout retry. The profile must remain reserved
until recovery completes. CLI/HTTP already provide profile_unavailable with the operation ID and
the instruction to inspect profile ops.

## Evidence

docs/qa/evidence/2026-10-02-untested/profile-recovery-fixed-ada-{web-entry,entry-divergence,
refresh-after,ended}.json; profile-recovery-entry-divergence.png and profile-recovery-after-retry.png.
The first entry waits twelve seconds for Settings and fails. The screenshot is inspected.
Recovery follows correction of the actual failed operation; no refresh loop or internal state
mutation is used. The browser recording is stopped before source investigation.

## Investigation

Pending. Distinguish profile-selection resolution from layout error presentation. Preserve the
unavailable-owner guard and remembered selection; do not silently reassign ownership or suppress
the failed write. Fix and re-walk this separately from the already reproduced audit/identity bugs.
