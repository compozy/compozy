---
id: ET-terminal-shared-control
area: ET
title: Work concurrently in one terminal without a control handoff
persona: Marina
journey: J-supervise-agent-terminal
expected: Every authorized interactive participant in the same workspace and profile can write, resize, answer input, signal, and close immediately; complete input submissions stay atomic and actor-attributed; no controller, lease, takeover, claim, yield, or terminal-scoped typing grant appears.
entry_points: Terminal app; compozy terminal attach; terminal WebSocket; terminal HTTP and UDS mutations; compozy__terminal_write; compozy__terminal_signal; compozy__terminal_close; compozy__terminal_request_input
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/qa/live-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/web-a-shared-terminal.png; /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/web-b-shared-terminal.png; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps: ET-terminal-stream-resilience; ET-terminal-cli-public-contract; ET-terminal-agent-handoff-input; ET-terminal-lease-fencing
---

Added by the 2026-09-04 shared-control hard cut. This is the canonical owner of simultaneous terminal
mutation. The two legacy ownership scenarios in `overlaps` are retired product memories, not active
contracts.

Walk:

1. Open one running terminal in two browser contexts and one interactive CLI attachment under the same
   workspace and profile; confirm every interactive surface accepts input immediately.
2. Submit distinct newline-terminated markers from all participants while output is live; confirm every
   marker arrives once and whole, and the journal attributes each completed command to its sender.
3. Keep an authorized agent bound to the same terminal and write through `compozy__terminal_write`; confirm
   the write is accepted without claim, yield, takeover, or a terminal-scoped typing grant.
4. Resize from different interactive participants, answer one hidden input request from a participant
   other than its creator, send a signal, and close from another participant; confirm each mutation
   succeeds and every public read converges.
5. Open an explicit read-only presentation attachment; confirm it receives transcript and presence but
   rejects mutation locally without changing anyone else's ability to act.
6. Inspect UI, CLI output, API projections, hook events, and native-tool results; confirm none exposes a
   controller or ownership transition and supported lifecycle events still match the final state.

2026-09-04 targeted walk: passed. Two independent browser sessions and two attached CLI clients wrote
to one terminal without a handoff. Concurrent CLI submissions stayed whole, journal entries retained
distinct actors, one client's detach did not revoke the other, signal delivery interrupted a running
command, and a new browser session reconnected with writable input. HTTP, UDS, CLI, the native-tool
catalog, and the hook catalog exposed no controller, lease, claim, takeover, or yield state.
