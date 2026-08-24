---
id: ET-profile-lifecycle-race-guards
area: ET
title: Hold the profile guards under concurrent lifecycle pressure
persona: Dora
journey: J-operate-profiles
expected: Archive committed against a concurrent claim, trigger, spawn, or bridge delivery never produces work for an archived owner and never half-applies; queued runs freeze with the profile and become claimable again on unarchive without duplicating; a notification delivery holding its permit refuses archive with the retryable profile_deliveries_in_flight and no delivery is repeated after unarchive; a pending lifecycle operation reserves its old and new names and derived paths so a competing create or rename fails profile_name_taken naming the holding operation; two concurrent same-name creates leave exactly one profile.
entry_points: compozy profile archive|unarchive|create|rename; POST /api/profiles/{name}/archive|unarchive; POST /api/profiles; POST /api/profiles/{name}/rename; concurrent automation trigger, task claim, session spawn, bridge delivery, notification dispatch, extension install
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-cli-lifecycle; ET-profile-operations-recovery; ET-profile-approval-owner-resume; ET-declared-profile-install
---

Minted by Profiles task 12 (planning): the spec names these races explicitly
(rename-versus-create/delete, archive-versus-claim/trigger/spawn, concurrent same-name create,
install-versus-create, crash between apply and finalize) and ADR-001 places the state change, the
running and leased checks, and the automation pause in one immediate transaction.
`ET-profile-cli-lifecycle` owns the sequential lifecycle and `ET-profile-operations-recovery` owns
crash recovery; this row owns concurrency. Task 13 owns the walk, the evidence, and the verdict.

Walk:

1. With a queued run waiting and a scheduled automation about to fire, archive the owning profile
   and confirm the archive and the claim cannot both win: either the claim happened before the
   archive committed, or it is refused — never a run created for an archived owner.
2. Confirm the archive result reports the paused automations and the count of frozen queued runs,
   and that the frozen runs are claimable again after unarchive with the same identities and no
   duplicates.
3. Race an archive against a session spawn and a bridge delivery in the same profile and confirm the
   creation boundary refuses with the typed archived-owner reason rather than writing a row.
4. Hold a notification delivery permit open and attempt the archive; confirm
   `profile_deliveries_in_flight` is returned as retryable, that the archive is refused, and that
   after the delivery settles the archive succeeds.
5. Kill the daemon while a permit row survives, restart, and confirm the delivery replays by its
   deterministic id exactly once and unarchive does not repeat it.
6. Start a rename so its operation is pending, then attempt to create or rename another profile to
   the reserved old name and to the reserved new name; confirm both fail `profile_name_taken` naming
   the holding operation, and that the derived filesystem paths are reserved too.
7. Issue two same-name creates concurrently and confirm exactly one profile exists afterwards with
   the loser receiving a typed refusal.
8. Run an extension install that declares a profile at the same moment the operator creates that
   name manually, and confirm the result is one profile, bound rather than seeded, with the
   create-once marker written exactly once.

Expected evidence: interleaved transcripts with timestamps for each race, run and row counts before
and after, the archive result payload, the permit refusal body, post-restart delivery counts by
delivery id, both reservation refusals naming the operation, the profile catalog after the
concurrent creates, and the marker row after the install race.
