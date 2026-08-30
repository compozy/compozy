---
id: ET-terminal-cli-public-contract
area: ET
title: Manage terminals through the complete CLI contract
persona: Ada
journey: J-operate-terminal-by-cli
expected: Non-interactive terminal verbs expose structured success and error output; attached open and attach preserve their interactive stream contract; CLI projections agree with HTTP and UDS; selectors obey profile rules.
entry_points: compozy terminal; HTTP and UDS /api/workspaces/{workspace_id}/terminals list, create, exec, input-requests, journal, recordings, artifacts, get, delete, attach-ticket, terminal stream, read, signal, wait, answer, reject, recording; catalog stream
qa_status: pass
bug_ids: BUG-20260826-terminal-journal-workspace-id; BUG-20260826-terminal-attach-profile-scope; BUG-20260826-terminal-cli-raw-mode; BUG-20260826-terminal-config-set-unsupported
fix_status: fixed
retest_status: pass
fix_commits: b745ebcbcfe6
evidence: /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/test-e2e-runtime-after-fix.log; docs/qa/reports/2026-08-26-integrated-terminal.md
last_report: docs/qa/reports/2026-08-26-integrated-terminal.md
overlaps: ET-profile-selection-precedence
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Exercise `open` (attached and `--detach`), `exec`, `list`, `get`, `attach` (watch and control), `kill`, `signal`, `input-requests`, `respond`, `journal`, `record` (start and stop), and `quote`.
2. Compare every non-interactive result with the matching HTTP and UDS route: list, create, exec, input requests, journal, recording and artifact downloads, get, delete, attach-ticket, read, signal, wait, answer, reject, and recording control.
3. Open the catalog SSE and per-terminal WebSocket over HTTP and UDS; verify initial state, live update, replay cursor, terminal attach, and close behavior on both transports.
4. Exercise the documented flag, selector, terminal-state, and capability failures and compare exact codes.
5. Attach in watch and control modes; verify the watch banner, detach chord, single-key passthrough, and exited-terminal refusal without expecting JSON from the interactive stream.

2026-08-30 CI repair re-walk: passed. Current-tree E2E-001 proved attached Bash command output,
listing, and durable journal identity; E2E-011 proved watch, control, single SIGQUIT, the double-key
detach chord, and exited-terminal refusal. The CLI detach timing regression also passed under `-race`.

2026-08-30 delayed-reader repair re-walk: passed. Exact-head CI run `33286027721` proved that the
150 ms chord window could expire before a saturated reader delivered the second byte. The current
tree expands the human chord window to 500 ms without changing single-key passthrough. E2E-011 passed
10/10 focused repetitions and one fresh post-flag public-interface walk; the delayed-reader regression
passed five times under `-race`.
