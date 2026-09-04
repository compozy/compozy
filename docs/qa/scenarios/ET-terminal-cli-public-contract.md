---
id: ET-terminal-cli-public-contract
area: ET
title: Manage terminals through the complete CLI contract
persona: Ada
journey: J-operate-terminal-by-cli
expected: Non-interactive terminal verbs expose structured success and error output; attached open and attach accept shared input immediately with no control flags; CLI projections agree with HTTP and UDS; selectors obey profile rules.
entry_points: compozy terminal; HTTP and UDS /api/workspaces/{workspace_id}/terminals list, create, exec, input-requests, journal, recordings, artifacts, get, delete, attach-ticket, terminal stream, read, signal, wait, answer, reject, recording; catalog stream
qa_status: pass
bug_ids: BUG-20260826-terminal-journal-workspace-id; BUG-20260826-terminal-attach-profile-scope; BUG-20260826-terminal-cli-raw-mode; BUG-20260826-terminal-config-set-unsupported
fix_status: fixed
retest_status:
fix_commits: b745ebcbcfe6
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/qa/live-evidence.md; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-profile-selection-precedence
---

QA impact 2026-09-04: interactive attach removed watch/control/takeover flags and now joins shared input
immediately. Reset for the v3 wire contract and the reduced nine-tool native catalog.

reset 2026-08-31: `kill` on an already-ended terminal is now an idempotent success reporting the recorded exit (signal stays a structured failure); the prior verdict pinned the old 409.
Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

2026-08-30 CI repair flag: the Linux PTY harness discarded asynchronous stdin delivery errors,
leaving E2E-011 unable to distinguish a failed write from a detach regression.

2026-08-30 acknowledged-write re-walk: passed 5/5. E2E-011 confirmed watch-mode input rejection,
control takeover, single `Ctrl-\` passthrough, double-key detach, clean CLI exit, and retained remote
output while every PTY harness write reported delivery.

Walk:

1. Exercise `open` (attached and `--detach`), `exec`, `list`, `get`, `attach`, `kill`, `signal`, `input-requests`, `respond`, `journal`, `record` (start and stop), and `quote`.
2. Compare every non-interactive result with the matching HTTP and UDS route: list, create, exec, input requests, journal, recording and artifact downloads, get, delete, attach-ticket, read, signal, wait, answer, reject, and recording control.
3. Open the catalog SSE and per-terminal WebSocket over HTTP and UDS; verify initial state, live update, replay cursor, terminal attach, and close behavior on both transports.
4. Exercise the documented flag, selector, terminal-state, and capability failures and compare exact codes.
5. Attach two interactive CLI clients at once; verify both can write, the shared-input banner, detach chord, single-key passthrough, and exited-terminal refusal without expecting JSON from the interactive stream.

2026-08-30 CI repair re-walk: passed. Current-tree E2E-001 proved attached Bash command output,
listing, and durable journal identity; E2E-011 proved watch, control, single SIGQUIT, the double-key
detach chord, and exited-terminal refusal. The CLI detach timing regression also passed under `-race`.

2026-08-30 delayed-reader repair re-walk: passed. Exact-head CI run `33286027721` proved that the
150 ms chord window could expire before a saturated reader delivered the second byte. The current
tree expands the human chord window to 500 ms without changing single-key passthrough. E2E-011 passed
10/10 focused repetitions and one fresh post-flag public-interface walk; the delayed-reader regression
passed five times under `-race`.

2026-08-30 raw-mode finalization re-walk: passed. Exact-head CI run `33296083331` proved the detach
chord could complete while the CLI restored its local terminal mode before the final terminal-state
query finished; a delayed `Ctrl-\\` then reached the local line discipline as SIGQUIT. Attached open
and attach now keep raw input active through the detach notice. E2E-011 passed 5/5 focused repetitions,
and the CLI timing and raw-mode suites passed three times under `-race`.

2026-09-04 targeted re-walk: passed. Two CLI clients attached with the shared-input banner, submitted
concurrent whole commands, and observed each other's output. One detached with the documented chord
while the other remained writable. CLI, UDS, and HTTP state/journal/tail projections agreed, SIGINT
reported delivered and produced exit 130, and the runtime advertised wire v3 with no control flags.
