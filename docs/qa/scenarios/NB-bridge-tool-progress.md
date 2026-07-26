---
id: NB-bridge-tool-progress
area: NB
title: Observe safe bridge tool progress
persona: Maya
journey: J-watch-agent-work-channel
expected: With progress enabled, a tool-heavy bridged turn shows an ordered and canonically redacted started-to-completed-or-failed lifecycle in the public channel; terminal-command previews expose command names without arguments, native-tool result fields use canonical redaction, queue pressure coalesces intermediate updates without losing terminal state, and the corresponding session transcript and ACP history contain no progress chrome.
entry_points: Bridge channel turn; public bridge delivery path; web session transcript; ACP history
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/agh-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/logs/bridge-fake-api.jsonl
last_report: docs/qa/reports/2026-07-12-hermes-bridge.md
overlaps:
---

An operator or teammate sees trustworthy, non-secret bridge progress without polluting the agent transcript.

Added by the Hermes bridge Task 01 impact flag. It covers canonical projection, redaction, ordered queue coalescing, terminal preservation, and transcript purity through public bridge delivery. Task 09 assigned it to `J-watch-agent-work-channel` and `CH-bridge-progress-stress`; Task 10 owns execution.

QA 2026-07-13: visible Slack progress was ordered, accumulated, canonically redacted, and separated from one material final. Exact integration proved transcript purity and opt-out silence.

Phase D impact flag 2026-07-13: public progress safety changed materially. Terminal previews now expose command names without positional or flag arguments, canonical redaction covers URL userinfo credentials, and native-tool preview, structured, and content fields use the same redactor. Status reset to `untested`; historical QA evidence remains intact. No QA retest ran.
