---
id: ET-profile-lifecycle-race-guards
area: ET
title: Hold the profile guards under concurrent lifecycle pressure
persona: Dora
journey: J-operate-profiles
expected: Archiving a profile against a concurrent claim, trigger, or spawn never produces work for an archived owner or half-applies. Queued runs freeze with the profile and become claimable again on unarchive without duplication. A pending lifecycle operation reserves old and new names and derived paths, so competing create or rename fails profile_name_taken without moving another profile. Extension mutation respects the same lifecycle gate.
entry_points: compozy profile archive|unarchive|create|rename; HTTP/UDS profile lifecycle routes; concurrent automation trigger, task claim, session spawn, extension install
qa_status: pass
bug_ids: BUG-20261004-task-owner-response-omissions; BUG-20261004-archive-race-storage-error; BUG-20261002-task-action-profile-scope; BUG-20261003-approval-cli-profile-owner; BUG-20261004-extension-profile-creator-attribution
fix_status: fixed
retest_status: pass
fix_commits: e9e4a46a6; 831436907; 0b9c79779
evidence: docs/qa/evidence/2026-10-02-untested/profile-owner-admission-replay-ended.json
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

QA 2026-10-04 repair replay: extension-created, operator-bound, legacy-unknown and manually
recreated profile identities now retain correct public attribution across a normal restart.
The affected delivery gate passes. BUG-20261004-extension-profile-creator-attribution is verified;
the remaining independent concurrency legs keep the full scenario untested.

QA 2026-10-04 continuation: an interrupted real rename reserves both names. Create and a competing
profile rename onto either name return profile_name_taken; the old-name refusal names the holding
operation and the new-name refusal names the held profile. Public retry succeeds after removing the
owned empty destination obstruction. The original name is restored, and the competing profile keeps
its identity. Evidence: approval-unavailable-owner2-ended.json. Claim/trigger/spawn, delivery, and
queued-run lifecycle legs remain pending.

QA 2026-10-04 continuation: corrected CLI run enqueue succeeds. Archive freezes exactly one
queued run; unarchive restores the same run without duplication. Authenticated acquisition and
a real bounded Codex child spawn retain the active owner while concurrent archive refuses.
In a separate job-trigger overlap, archive wins; session admission refuses, the job is disabled,
and one failed attempt remains. Scheduler and profile availability are restored. The CLI control
recurrence is verified. Missing backlog ownership fields and a raw storage diagnostic keep this
scenario fail until their independent findings are repaired and re-walked.
Evidence: profile-work-race-replay-ended.json.

QA 2026-10-04 final repair replay: fresh queued work retains its profile in backlog and update
responses, through archive/unarchive and independent HTTP/UDS reads. The losing automation trigger
reaches the insertion guard and reports profile_archived with recovery guidance; one failed run
remains, no session is admitted, and history contains no raw SQLite error. Active-owner session
creation still succeeds and blocks archive. The scheduler, owner and sessions are restored to
their baseline availability. Combined with the retained claim/spawn, pending-name and extension
races, the current scenario passes. The charter's pre-retirement notification-permit leg is excluded
by the September 27 scope change, as documented in the report, rather than counted as verified.
