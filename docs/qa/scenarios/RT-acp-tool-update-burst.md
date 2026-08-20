---
id: RT-acp-tool-update-burst
area: RT
title: Complete a prompt through a repeated ACP tool update burst
persona: Ada
journey: J-15
expected: More than 1,024 repeated in-progress updates for one tool produce one canonical tool call, preserve meaningful enrichments, emit the terminal result before prompt completion, and keep the provider connected.
entry_points: compozy session prompt -o jsonl; UDS session prompt; session raw event stream
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/francisross/dev/qa-labs/compozy-acp-tool-update-burst-20260820-130528-159317-lab/qa-artifacts/qa/logs/session-events-summary.json
last_report: docs/qa/reports/2026-08-20-acp-tool-update-burst.md
overlaps: RT-acp-stream-disconnect-recovery
---

Issue #439 exposed queue pressure caused by repeated nonterminal ACP updates for one tool call. This
scenario owns the successful burst path; the linked recovery scenario owns behavior after a provider
has already disconnected.
