---
id: ET-terminal-cli-public-contract
area: ET
title: Manage terminals through the complete CLI contract
persona: Ada
journey: J-operate-terminal-by-cli
expected: Non-interactive terminal verbs expose structured success and error output; attached open and attach preserve their interactive stream contract; CLI projections agree with HTTP and UDS; selectors obey profile rules.
entry_points: compozy terminal; HTTP and UDS /api/workspaces/{workspace_id}/terminals list, create, exec, input-requests, journal, recordings, artifacts, get, delete, attach-ticket, terminal stream, read, signal, wait, answer, reject, recording; catalog stream
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-selection-precedence
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Exercise `open` (attached and `--detach`), `exec`, `list`, `get`, `attach` (watch and control), `kill`, `signal`, `input-requests`, `respond`, `journal`, `record` (start and stop), and `quote`.
2. Compare every non-interactive result with the matching HTTP and UDS route: list, create, exec, input requests, journal, recording and artifact downloads, get, delete, attach-ticket, read, signal, wait, answer, reject, and recording control.
3. Open the catalog SSE and per-terminal WebSocket over HTTP and UDS; verify initial state, live update, replay cursor, terminal attach, and close behavior on both transports.
4. Exercise the documented flag, selector, terminal-state, and capability failures and compare exact codes.
5. Attach in watch and control modes; verify the watch banner, detach chord, single-key passthrough, and exited-terminal refusal without expecting JSON from the interactive stream.
