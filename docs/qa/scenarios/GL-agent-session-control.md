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

Issue #595 acceptance: activate the session's own Goal during an ordinary Codex prompt after context usage arrives. Read Goal, session supervision, Loop nodes, and task states through CLI and HTTP, then refresh/reconnect. The first context observation must not fail ownership validation; known context reads must resolve their pinned event. Pause/resume and real approval or quarantine retain actionable attention, and recovery clears derived attention. Stop and remove separate live Goal sessions, cancel again by Run, and confirm terminal state with retained audit and unaffected foreign sessions.

Targeted #595 retest evidence and limits: [Goal lifecycle report](../reports/2026-09-10-issue-595-goal-lifecycle.md). This slice does not replace earlier evidence or claim an unrun full-scenario sweep.

Pending-stop restart regression: after advancing a Goal to a new Run generation, retain the old
queued checkpoint and its already-canceled prompt. Stop the owning session and restart the daemon.
Expect readiness, cancellation of the current Goal generation, and removal of the settled stop
receipt, with historical checkpoints and session events preserved. Repeat startup to verify that
recovery does not attempt to revoke the historical prompt again.

Targeted recovery evidence (2026-09-14): the real retained database reproduced the prepared-prompt
conflict. A beta.25 recovery build with the current-generation filter reached readiness, then the
original beta.25 runtime restarted successfully and the desktop reported `state: product` with
`runtime.attached: true`. All 13 sessions remained present; generation 1's checkpoint matched its
backup, generation 2's Run was canceled, and the stop receipt was removed. The existing Goal runtime
and time-travel integration suites passed with the race detector. This verifies the restart slice,
not a new full cross-surface scenario sweep.
