---
id: NB-bridge-edit-reply
area: NB
title: Preserve bridge edit and reply intent
persona: Maya
journey: J-edit-reply-context
expected: A supported Slack or Telegram message edit reaches the routed agent as a distinct edit with the affected message identity and replacement or deletion operation; Slack, Telegram, and Google Chat replies include bounded already-observed parent text and author when available, while a cache miss remains empty without a provider fetch or workspace, instance, or conversation bleed.
entry_points: Public Slack, Telegram, and Google Chat inbound bridge webhooks; routed session prompt
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/test-cases/bridge-qa-fixture.json; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/logs/bridge-fake-api.jsonl
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps: NB-024; NB-bridge-tool-progress
---

An operator or teammate can correct a message or reply in context without the agent confusing
historical quoted text with the current instruction.

Added by the Hermes bridge Task 06 impact flag. Task 09 assigned it to `J-edit-reply-context` and `CH-edit-reply-context`; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: Slack `message_changed` reused the routed session and reflected corrected intent. Exact Slack/Telegram/Google Chat/cache/Host API/daemon owners proved typed edits, bounded context, cold misses, and isolation. Discord ordinary edits remain explicitly unsupported.
