---
id: NB-provider-progress-rendering
area: NB
title: Render live tool progress across bridge providers
persona: Maya
journey: J-watch-agent-work-channel
expected: Slack, Telegram, and Discord render default-on tool progress as one editable channel bubble with typing and reaction affordances; opted-in Teams and Google Chat update one status; WhatsApp emits sparse append-only statuses; the final answer stays separate; disabling progress makes no platform call; GitHub and Linear acknowledge progress without writing to issues; concurrent progress events for one delivery create exactly one dispatcher and one progress bubble; and repeated events in the same phase do not reapply the same reaction.
entry_points: Public bridge turns through Slack; Telegram; Discord; Teams; Google Chat; WhatsApp; GitHub; Linear adapters; per-instance delivery_defaults.progress
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/logs/bridge-fake-api.jsonl
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps: NB-bridge-tool-progress; NB-long-bridge-replies
---

An operator or teammate sees current tool activity in the channel without confusing it with the final answer or issue content.

Added by the Hermes bridge Task 03 impact flag. Task 09 assigned it to `J-watch-agent-work-channel` and `CH-bridge-progress-stress`; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: chat-provider progress and issue-provider no-side-effect policies passed exact owners; manual Slack mode-off emitted only final-answer calls. The all-provider simultaneous soak remains in the automation backlog.

Phase D impact flag 2026-07-13: progress dispatcher creation is now atomic per delivery across the six chat adapters, preventing concurrent events from opening duplicate bubbles; repeated events in one phase no longer reapply the same reaction. Status reset to `untested`; historical provider evidence remains intact. No QA retest ran.
