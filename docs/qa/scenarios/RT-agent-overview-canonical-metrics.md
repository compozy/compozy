---
id: RT-agent-overview-canonical-metrics
area: RT
title: Agent overview Active Runtime Failed Last activity
persona: Bruno
journey: J-31
expected: Overview metrics are Active, Runtime, Failed, and Last activity from workspace-scoped canonical data; Active uses server totals; Runtime/Failed show honest unavailable when the session page is incomplete and no daemon aggregate exists; never substitutes Total sessions or Resumable.
entry_points: web /agents/$name?tab=overview
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/038-agent-detail-deep-links
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-076
---

Added by agent-details remediation 2026-07-12 after replacing Total sessions / Resumable with the approved metric contract.

QA completion 2026-07-29: workspace-scoped catalog truth for `qa-hook-agent` was 0 active of 8
sessions, 1,629 runtime seconds, 5 failed, and last activity at
`2026-07-29T07:44:58.108927Z`. Overview rendered `0 of 8 sessions`, `27m 9s`, `5`, and `5h ago`;
the Sessions Failed filter rendered exactly the five rows carrying public failure metadata. No Total
sessions or Resumable substitution appeared.
