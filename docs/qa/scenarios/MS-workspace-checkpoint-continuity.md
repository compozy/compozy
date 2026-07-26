---
id: MS-workspace-checkpoint-continuity
area: MS
title: Preserve workspace continuity through one checkpoint summary
persona: Théo
journey: J-11
expected: Eligible session stops asynchronously update one workspace-scoped `project_checkpoint_summary.md` through the active memory provider and decision WAL. The next session receives the full checkpoint; failed updates preserve the prior bytes; decision revert restores the previous summary; another workspace never receives the artifact.
entry_points: daemon session stop/start; workspace memory files; memory decisions CLI/HTTP/UDS/native tools
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-context-rebuild; MS-022
---

Use two workspaces and complete two sessions with distinct durable facts in the first workspace.
Confirm that one checkpoint file is updated in place, its provenance contains both source sessions,
and a new session receives the latest facts inside `<agh_checkpoint_summary>`. Force the summary
provider to fail and confirm the file stays byte-identical. Revert the latest checkpoint decision
and confirm the prior content returns. Start a session in the second workspace and confirm that no
checkpoint fact from the first workspace is present.

QA impact 2026-07-15: new workspace checkpoint lifecycle and prompt continuity behavior. Planning
flag only; no QA session ran in this implementation slice.

Phase C planning 2026-07-19: persona normalized to Théo and linked to J-11; companion to US-004
(§3.3 checkpoint summaries). Forensic contract (SD-006): timestamped command + observed output for
the in-place summary update with both source sessions in provenance, the byte-identical file after
an injected provider failure, the decision-WAL revert restoring prior content, and the
workspace-isolation probe.
