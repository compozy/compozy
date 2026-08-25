---
id: GL-agent-session-control
area: GL
title: Let an agent control an authorized child Goal through typed surfaces
persona: Ada
journey: J-29
expected: An authenticated agent sets, replaces, reads, pauses, resumes, and clears a target session Goal through native, HTTP, UDS, and CLI surfaces with matching structured results, while an unrelated session is rejected and the Goal keeps its immutable origin and workspace participation.
entry_points: compozy__goal_control; POST /api/workspaces/{workspace_id}/sessions/{session_id}/goal; UDS equivalent; compozy session goal
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-eng-148-agent-session-control-20260825-014009-304323-lab/qa-artifacts/qa/journey-log.jsonl; internal/daemon goal lifecycle and eight-child isolation race suite; final CLI/HTTP/UDS/native focused checks
last_report: docs/qa/reports/2026-08-24-eng-148-agent-session-control.md
overlaps: GL-025;GL-026;GL-034
---

ENG-148 flag: new typed agent-manageability behavior. Walk the same lifecycle from an agent-owned
session to itself and to a descendant, compare direct HTTP/UDS/CLI/native output, then attempt a
foreign target. Include a runtime override and verify the target's Goal origin/network provenance
does not change. Record failed binding evidence rather than treating it as an empty projection.
