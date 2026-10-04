---
id: ET-profile-remote-write-boundary
area: ET
title: Keep profile management local while preserving remote reads
persona: Ada
journey: J-operate-profiles
expected: Enabled remote HTTP tiers expose only scoped profile reads; every profile-state write returns 403 profile_remote_management_forbidden with the canonical action, while the same mutation succeeds through local HTTP, UDS, CLI, and delegated command-palette flows.
entry_points: remote and local /api/profiles routes; compozy profile; profile.use|create|update|rename|archive|unarchive|delete palette actions
qa_status: fail
bug_ids: BUG-20261004-profile-palette-drops-arguments; BUG-20261004-profile-palette-navigation-stall; BUG-20261004-profile-switch-loses-command-result
fix_status: open
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-10-02-untested/profile-local-boundary-ada-ended.json; docs/qa/evidence/2026-10-02-untested/profile-local-retry-ada-ended.json; docs/qa/evidence/2026-10-02-untested/profile-palette-lifecycle-fixed-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-cli-lifecycle; ET-agent-command-invoke; ET-profile-palette-view
---

Flagged by Profiles task 04. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Enable a paired remote surface and compare profile list/current/selection reads with scoped local
   truth; verify no private data or secret refs appear.
2. Attempt create, update, rename, archive, unarchive, delete, selection PUT, and operation retry on the
   remote listener; every response must be `403` with the same structured code, message, and action.
3. Repeat each mutation on local HTTP and UDS and through its CLI flow; verify normal success.
4. Discover and invoke every stable `profile.*` palette descriptor; prove selection/lifecycle delegates
   to the canonical surface, preserves plan revisions, and honors destructive confirmation.

Expected evidence: remote/local HTTP and UDS matrices, CLI transcripts, palette descriptors and invoke
results, and proof that rejected remote calls changed no profile state.

QA 2026-10-04: all eight local mutation families succeed through HTTP, UDS, and CLI, with
independent readback and real failed-operation retry. The paired remote leg is unavailable:
Gateway is disabled and the installed Tailscale provider has no TS_AUTHKEY binding. No remote
policy verdict is claimed. Delegated profile.create loses its dialog intent; direct Web creation
loses its name, and the first Web attempt stalls. The local findings remain under repair.

Repair replay 2026-10-04: all seven attached palette descriptors now complete their canonical
handoffs. Ada owns invocation and independent reads; Bruno owns dialog review and confirmation.
The supplied create/rename names survive, plan revisions match rename/archive/delete requests,
and delete still requires command approval and confirmation. Cancellation preserves the profile;
the final delete is independently absent after reload, with the original selection restored.
The argument-loss and switching-result bugs are verified. This row remains Fail because the
separate earlier navigation stall has no established cause; the paired remote leg also remains
unverified for the already recorded external prerequisite.
