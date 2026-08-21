---
id: MS-attention-settings-roundtrip
area: MS
title: Keep operator attention settings consistent across every surface
persona: Dora
journey: J-administer-runtime-settings
expected: Settings → Notifications, config.toml, and compozy config get/set agree on the global toasts, sound, and system values; HTTP, UDS, and Web read and replace muted_workspaces for the selected profile without changing another profile; valid changes apply live without a daemon restart, concurrent writes preserve a complete candidate, and deleting a workspace removes every profile-owned mute row.
entry_points: web Settings → Notifications; config.toml [attention]; compozy config get/set attention.toasts|sound|system; GET/PATCH /api/settings/attention?scope=user or ?scope=profile&profile=<name> over HTTP and UDS; workspace deletion
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json; docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps:
---

Start from the documented defaults, change each global delivery field through one public surface,
and confirm every other global surface reads the same active value without a restart. Exercise
concurrent complete-section writes, mute different workspaces in two profiles through the typed
Settings route, delete one workspace, and prove only its profile-owned rows are removed while the
other profile and global delivery settings stay intact.

QA impact 2026-08-16: Task 02 added the live attention config and settings transport. Flag only;
task_08 owns execution after the web surface lands.

QA 2026-08-16 Herdr parity: Sequential config, HTTP, UDS, and Web coverage kept the complete attention section consistent, proved public workspace mute identifiers and non-null list payloads, and retained the active policy across reload without a restart.

2026-08-23 qa-impact (Profiles): the Profiles hard cut removes the
`attention.muted_workspaces` config array. The authoritative rows live in
`attention_workspace_mutes`; Web, HTTP, and UDS select them by profile, while `config.toml` and the
config CLI retain only the three global delivery booleans. Already `untested`, so no reset was
needed. The walk must seed foreign-profile rows and prove reads, replacements, notification
suppression, cache identity, and workspace-delete cascades stay isolated.
