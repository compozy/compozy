---
id: ET-profile-lifecycle-race-guards
area: ET
title: Hold the profile guards under concurrent lifecycle pressure
persona: Dora
journey: J-operate-profiles
expected: Archiving a profile against a concurrent claim, trigger, or spawn never produces work for an archived owner or half-applies. Queued runs freeze with the profile and become claimable again on unarchive without duplication. A pending lifecycle operation reserves old and new names and derived paths, so competing create or rename fails profile_name_taken without moving another profile. Extension mutation respects the same lifecycle gate.
entry_points: compozy profile archive|unarchive|create|rename; HTTP/UDS profile lifecycle routes; concurrent automation trigger, task claim, session spawn, extension install
qa_status: fail
bug_ids: BUG-20261003-approval-cli-profile-owner; BUG-20261004-extension-profile-creator-attribution
fix_status: pending
retest_status: pending
fix_commits: e9e4a46a6
evidence: docs/qa/evidence/2026-10-02-untested/approval-owner-replay-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-cli-lifecycle; ET-profile-operations-recovery; ET-profile-approval-owner-resume; ET-declared-profile-install
---

Archiving a profile against a concurrent claim, trigger, or spawn never produces work for an archived owner or half-applies. Queued runs freeze with the profile and become claimable again on unarchive without duplication. A pending lifecycle operation reserves old and new names and derived paths, so competing create or rename fails profile_name_taken without moving another profile. Extension mutation respects the same lifecycle gate.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA 2026-10-03: a real pending palette approval blocks archive/delete and is excluded from work counts.
Owner-selected approval show/cancel fail with not found; remaining lifecycle legs await that repair.

QA 2026-10-03 repair replay: owner show/cancel now succeeds, foreign show/cancel stays refused,
HTTP/UDS plan blockers clear after cancellation, and archive/unarchive/delete succeed with zero
work items. BUG-20261003-approval-cli-profile-owner is verified; the complete scenario remains
untested because its independent resume/concurrency legs have not all been walked.

QA 2026-10-04: simultaneous same-name creates yield one owner and one profile_name_taken refusal.
An overlapping extension install and operator create also preserve one operator-authored profile,
but extension detail falsely claims created_by_extension=true. The attribution bug owns that
repair; claim/trigger/spawn, delivery and remaining reservation legs are still pending. Evidence:
profile-race-namespace-*.json and profile-race-extension-replay-*.json in this cycle's report.
