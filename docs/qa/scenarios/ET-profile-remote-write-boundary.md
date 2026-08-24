---
id: ET-profile-remote-write-boundary
area: ET
title: Keep profile management local while preserving remote reads
persona: Ada
journey: J-operate-profiles
expected: Enabled remote HTTP tiers expose only scoped profile reads; every profile-state write returns 403 profile_remote_management_forbidden with the canonical action, while the same mutation succeeds through local HTTP, UDS, CLI, and delegated command-palette flows.
entry_points: remote and local /api/profiles routes; compozy profile; profile.use|create|update|rename|archive|unarchive|delete palette actions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
