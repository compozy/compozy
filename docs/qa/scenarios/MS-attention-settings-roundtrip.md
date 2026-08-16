---
id: MS-attention-settings-roundtrip
area: MS
title: Keep operator attention settings consistent across every surface
persona: Dora
journey: J-administer-runtime-settings
expected: Settings → Attention, config.toml, compozy config get/set, HTTP, and UDS read and write the same toasts, sound, system, and muted_workspaces values; valid changes apply live without a daemon restart, concurrent writes preserve a complete config, and deleting a muted workspace removes its id.
entry_points: web Settings → Attention; config.toml [attention]; compozy config get/set attention.*; GET/PATCH /api/settings/attention over HTTP and UDS; workspace deletion
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Start from the documented defaults, change each attention field through one public surface, and
confirm every other surface reads the same active value without a restart. Exercise concurrent
complete-section writes, mute two workspaces, delete one, and prove only the deleted workspace id is
pruned while the remaining settings stay intact.

QA impact 2026-08-16: Task 02 added the live attention config and settings transport. Flag only;
task_08 owns execution after the web surface lands.
