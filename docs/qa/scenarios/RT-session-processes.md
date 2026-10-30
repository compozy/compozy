---
id: RT-session-processes
area: RT
title: Inspect and control session processes across stop and restart
persona: Théo
journey: J-session-processes
expected: Owned background processes and ports are visible; explicit terminate verifies exit; keep preserves detached descendants across restart and permits orphan cleanup
entry_points: Session composer Processes control; command palette Processes left running; compozy session processes; /api/processes
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: reports/2026-10-29-session-processes.md
last_report: reports/2026-10-29-session-processes.md
overlaps:
---

Start an agent that launches `sleep 9999 &` and an HTTP server. Inspect both identities, attribution,
and the listening port through the session list. Stop with terminate; verify both disappear and
cannot serve requests. Start another session, stop with keep, restart only the isolated daemon,
and terminate the retained orphan identities through the operator aggregate. Include a native
terminal as the adjacent lifecycle canary: it closes on daemon shutdown, while detached children
remain. Never signal the daemon or the agent leader.

Exercise stop, archive, and delete policy choices in the Web dialogs. Verify independent API reads
and cross-profile isolation. On macOS, observe the protected sleep while attached before releasing
its parent; the documented pre-observation blind spot is not a passing attribution case.

Automation: `web/e2e/__tests__/session-processes.spec.ts` (`E2E-PROC-001`) owns composer discovery
of an externally parented, marked detached Node process and stop-with-terminate. `E2E-PROC-002`
walks archive and delete from the session list with both keep and terminate, verifies the policy
reaches the daemon, and checks process survival or exit independently. Real OS identity,
lineage/restart persistence, and reused-PID safety remain in `internal/procutil` and
`internal/sessionprocess`; wire policy belongs to the API/client suites.
