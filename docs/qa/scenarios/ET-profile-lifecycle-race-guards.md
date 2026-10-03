---
id: ET-profile-lifecycle-race-guards
area: ET
title: Hold the profile guards under concurrent lifecycle pressure
persona: Dora
journey: J-operate-profiles
expected: Archiving a profile against a concurrent claim, trigger, or spawn never produces work for an archived owner or half-applies. Queued runs freeze with the profile and become claimable again on unarchive without duplication. A pending lifecycle operation reserves old and new names and derived paths, so competing create or rename fails profile_name_taken without moving another profile. Extension mutation respects the same lifecycle gate.
entry_points: compozy profile archive|unarchive|create|rename; HTTP/UDS profile lifecycle routes; concurrent automation trigger, task claim, session spawn, extension install
qa_status: fail
bug_ids: BUG-20261003-approval-cli-profile-owner
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-cli-lifecycle; ET-profile-operations-recovery; ET-profile-approval-owner-resume; ET-declared-profile-install
---

Archiving a profile against a concurrent claim, trigger, or spawn never produces work for an archived owner or half-applies. Queued runs freeze with the profile and become claimable again on unarchive without duplication. A pending lifecycle operation reserves old and new names and derived paths, so competing create or rename fails profile_name_taken without moving another profile. Extension mutation respects the same lifecycle gate.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA 2026-10-03: a real pending palette approval blocks archive/delete and is excluded from work counts.
Owner-selected approval show/cancel fail with not found; remaining lifecycle legs await that repair.
