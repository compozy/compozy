---
id: ET-profile-web-aggregate-owner-surfaces
area: ET
title: Show owners, banners, empty states, and usage per profile across the web surfaces
persona: Ada
journey: J-scope-work-by-profile
expected: Aggregate listings tag every row with its owner and mute an archived one; scoped listings stay tag-free; worktree rows carry the owner tag in every profile; a deep link into another profile's session renders an owner banner with a one-tap switch instead of bouncing; empty listings name the active profile; the Home usage panel shows scoped figures per profile and a labeled per-profile breakdown under the aggregate.
entry_points: Sessions, Tasks, Loop runs, Automation jobs and triggers, and Worktrees listings; /session/{id} deep link; Home usage panel; command palette session results
qa_status: pass
bug_ids: BUG-20260826-profile-palette-session-owner; BUG-20260826-profile-palette-worktree-owner; BUG-20260826-session-cli-profile-scope
fix_status: fixed
retest_status: pass
fix_commits: current-remediation-batch
evidence: /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-tasks-owner-tags.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-command-palette-session-owner-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-jobs-triggers-owner-tags.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-loop-runs-owner-tags.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/usage-all-profiles-breakdown.png
last_report: docs/qa/reports/2026-08-26-profile-identity-final.md
overlaps: ET-profile-aggregate-owner-labels; ET-profile-deep-link-owner; ET-profile-scoped-work-reads
---

Flagged by Profiles task 07. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. With three profiles — one of them archived — turn All profiles on and confirm every task, loop
   run, automation job, and automation trigger row names its owner; session rows instead carry only
   the owner's colored glyph, with the name available on hover and to assistive tech. Confirm the
   archived owner reads as archived rather than only appearing dimmer.
2. Switch to a real profile and confirm the same listings carry no owner tags at all.
3. In both states confirm worktree rows still carry their owner tag, and that no worktree disappears
   from a scoped list.
4. Open a session belonging to another profile by its direct URL. Confirm the item is not bounced,
   that the banner names the owning profile, and that the switch action lands on that profile.
   Confirm the surrounding listings did not widen.
5. Enter a profile with no work and confirm each empty listing names that profile rather than saying
   only that the list is empty.
6. Open Home usage scoped to one profile and confirm the figures cover that profile only and no
   breakdown is offered; turn All profiles on and confirm the per-profile breakdown appears with the
   archived owner included and pre-profile history under default.

Expected evidence: screenshots of the aggregate and scoped listings, the archived owner tag, a
worktree row in two profiles, the deep-link banner before and after the switch, one profile-named
empty state, and both usage states.

QA 2026-08-26: Passed after fixing owner propagation for command-palette sessions and worktrees.
Tasks, runs, jobs, triggers, usage, empty states, and the foreign-owner deep-link banner matched the
aggregate and scoped contracts in fresh browser loads.
