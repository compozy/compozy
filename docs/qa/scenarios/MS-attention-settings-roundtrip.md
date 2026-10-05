---
id: MS-attention-settings-roundtrip
area: MS
title: Keep operator attention settings consistent across every surface
persona: Dora
journey: J-administer-runtime-settings
expected: Settings → Notifications, config.toml, and compozy config get/set agree on the global toasts, sound, and system values; HTTP, UDS, and Web read and replace muted_workspaces for the selected profile without changing another profile; valid changes apply live without a daemon restart, concurrent writes preserve a complete candidate, and deleting a workspace removes every profile-owned mute row.
entry_points: web Settings → Notifications; config.toml [attention]; compozy config get/set attention.toasts|sound|system; GET/PATCH /api/settings/attention?scope=user or ?scope=profile&profile=<name> over HTTP and UDS; workspace deletion
qa_status: untested
bug_ids: BUG-20261004-attention-default-profile-refused; BUG-20261004-attention-deleted-workspace-stale; BUG-20261004-settings-offline-save-stuck
fix_status: fixed
retest_status: pass
fix_commits: 84f02d6b2
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json; docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-10-02-untested.md
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

2026-10-04 partial replay: explicit profile-scoped attention reads succeed for resume-editorial
but return 400 for default on both HTTP and UDS. Web pop-up changes agree with CLI/HTTP/UDS;
CLI sound changes apply live and match the public config.toml attention section. The open Web
control converges after the initial 12-second observation window and before reload. The final
debrief corrects its initial attribution to reload; no stale-control bug is established.
Baseline true/true/false channels and empty mutes are restored and independently read, the
eight-frame recording is closed, and screenshots are inspected. Profile mute isolation/pruning,
two-tab writes and actual channel delivery remain Pending. Evidence: attention-settings-dora-ended.json
and its linked receipts under docs/qa/evidence/2026-10-02-untested/.

Post-session contract review invalidates the default-profile refusal finding: the documented
default selector is scope=user, already successful in the baseline receipt. Profile scope is
reserved for non-default profiles. No product or test change is needed. The scenario remains
untested for its outstanding legs; the invalid registry entry is retained for provenance.

The next Dora walk proves default/non-default mute isolation, Web add/remove with independent
UDS reads, profile-switch cache identity, and workspace-delete pruning in both profiles while
retaining another mute. The open Web page retains the deleted workspace ID until reload; that
separate observed defect is linked above. All original settings are restored and both owned
workspace registrations removed. Two-tab writes and actual delivery remain Pending. Evidence:
docs/qa/evidence/2026-10-02-untested/attention-mutes-dora-ended.json.

A fresh deletion walk confirms a channel write from the stale page fails with HTTP 500 and a
visible Internal Server Error; independent UDS proves the prior complete policy survived.
The repair separates channel writes from optional mute replacement, rereads the canonical policy
after catalog removal, and classifies unknown workspace replacements through the existing 404
boundary. Owning red/green checks pass; real replay and delivery verification remain pending.
Evidence: attention-delete-dora-ended.json and attention-deletion-{web,store}-green.json.

The rebuilt original-persona deletion replay now passes: the open page prunes only the removed
mute in 3.817 seconds; a channel change from a still-stale page returns 200 without replacing
mutes; explicit missing-workspace replacements return 404 over HTTP and UDS with full rollback.
Refresh and independent reads preserve the remaining mute and saved sound value. Baseline values
and empty lists are restored and all owned registrations removed. Six screenshots are inspected
and the 14-frame recording is closed. Evidence: attention-repair-dora-ended.json. Gate/commit,
two-tab complete writes and actual channel delivery remain pending; the full row is not promoted.

Further walks prove sequential two-tab complete candidates survive independent reads and reload,
and actual Chrome denied/granted permissions display Blocked/Allowed truthfully. A denied toggle
does not write; global system intent saved through HTTP and UDS agrees with the other surfaces,
while the blocked browser does not claim delivery. The original Ask (default) permission, global
true/true/false and empty mutes are restored. A real session also produces a guide and a visible
completion toast; custom notification acknowledgement alone is not claimed as visual delivery.
Evidence: attention-tabs-dora-ended.json, attention-delivery-dora-ended.json, and
attention-permission-dora-ended.json. No simultaneous writer, unsupported platform, channel/mute
suppression or actual OS notification delivery is claimed. The full charter remains pending.

The real suppression continuation now passes both workspace and channel legs. While Studio
Operations is muted, the managed session's native notification returns muted-workspace and the
continuous browser observation records no published notification, toast or media event. Its
Finished row remains visible with the mute marker and survives reload. With mutes cleared and
all channels disabled, native delivery succeeds and one matching notification reaches the browser,
but no toast or media event appears; the Finished row remains. Restoring the baseline causes no
old-toast replay in the observed eight-second window. Independent reads and refresh confirm the
original policy, and the owned session is verified stopped. Evidence: attention-suppression-dora-ended.json.
Commit 84f02d6b2 closes the deletion defect after real replay and the current-tree delivery gate.
The tracker remains untested for the full charter's simultaneous-writer and platform-delivery legs;
fix/retest fields describe the linked repaired defect only.


2026-10-04 adjacent Settings recovery: Sound now settles offline refusal with actionable guidance,
retains its saved value and stays interactive. Reconnection does not write; an explicit toggle
succeeds and matches independent public GET. Restoring the full policy and reloading leaves the
default browser permission, no restart and no active sessions. Evidence:
docs/qa/evidence/2026-10-02-untested/settings-offline-guidance-dora-ended.json and
settings-offline-guidance-restored-attention.json. The linked offline-save repair awaits its
gate/commit; simultaneous-writer and platform-delivery legs remain pending.
