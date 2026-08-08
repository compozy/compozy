---
id: NB-bridge-tool-progress
area: NB
title: Observe safe bridge tool progress
persona: Maya
journey: J-watch-agent-work-channel
expected: With progress enabled, a tool-heavy bridged turn shows an ordered and canonically redacted started-to-completed-or-failed lifecycle in the public channel; terminal-command previews expose command names without arguments, native-tool result fields use canonical redaction, queue pressure coalesces intermediate updates without losing terminal state, and the corresponding session transcript and ACP history contain no progress chrome.
entry_points: Bridge channel turn; public bridge delivery path; web session transcript; ACP history
qa_status: blocked-verify
bug_ids: BUG-20260808-bridge-enable-without-config
fix_status: fixed
retest_status: pass
fix_commits: pending review-remediation batch
evidence: /home/pedronauck/dev/qa-labs/compozy-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/compozy-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/logs/bridge-fake-api.jsonl;/Users/pedronauck/dev/qa-labs/compozy-qa-misc-network-goal-release-site-20260730-060405-932516-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/test-cases/10-bridge-progress-integration.log;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/provider-attempt.json
last_report: docs/qa/reports/2026-08-08-remote-gateway-toolmeta-remediation.md
overlaps:
---

An operator or teammate sees trustworthy, non-secret bridge progress without polluting the agent transcript.

Added by the Hermes bridge Task 01 impact flag. It covers canonical projection, redaction, ordered queue coalescing, terminal preservation, and transcript purity through public bridge delivery. Task 09 assigned it to `J-watch-agent-work-channel` and `CH-bridge-progress-stress`; Task 10 owns execution.

QA 2026-07-13: visible Slack progress was ordered, accumulated, canonically redacted, and separated from one material final. Exact integration proved transcript purity and opt-out silence.

Phase D impact flag 2026-07-13: public progress safety changed materially. Terminal previews now expose command names without positional or flag arguments, canonical redaction covers URL userinfo credentials, and native-tool preview, structured, and content fields use the same redactor. Status reset to `untested`; historical QA evidence remains intact. No QA retest ran.

QA impact 2026-08-08: review remediation registered `compozy__gateway` in the native progress metadata inventory, changing the public bridge label from fallback text to the canonical `Reading` presentation. Reset for a focused bridge-progress re-walk before workstream close.

QA re-walk 2026-08-08: the deterministic bridge harness passed routing, redaction, terminal-state preservation, and transcript purity after fixing absent `provider_config` reads. The scenario remains `blocked-verify` because no authorized ACP/provider turn or repository fixture can emit `compozy__gateway` through a public bridge and visibly prove the new `Reading` and `🌐` presentation.
