# BUG-20260906-terminal-detach-close-race: A completed detach can reconnect the CLI

- **Status:** fixed locally — deterministic race regression and three unchanged Bash CLI journeys pass; current-head CI pending
- **Impact:** Correctness
- **Severity:** Major · **Priority:** P1
- **Scenario:** ET-terminal-cli-public-contract
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

PR #557 CI run `34059656353`, Web shard 4, failed the unchanged E2E-001 CLI golden path after the double `Ctrl-\` chord. The CLI retained its attachment instead of printing the detach notice and exiting; the shell command had already completed.

The input writer sends DETACH and reports `errTerminalDetached` to the stream owner. The server can accept that frame and close the socket before the input writer returns. If the stream owner selects the socket read error first, its cleanup joins the input writer but previously discarded that result. The outer attach loop then treats the socket EOF as transient and reconnects, consuming the operator's completed detach intent.

The stream owner now retains the joined input result. A completed detach wins over the simultaneous server read failure; context cancellation, output errors, unsuccessful writes and socket cleanup errors remain failures. No timeout or chord behavior changes.

Invariant: a detach accepted by the peer completes the attachment without reconnection even when the peer closes before the input write returns. Owning layer: `internal/cli`. Canonical suite: `TestTerminalClientStreamShouldDetachWithoutReconnect` in `client_terminal_stream_test.go`. Its connection wrapper controls only the network I/O completion order. The new case fails deterministically before the repair with WebSocket close 1006 / unexpected EOF and passes after it. All `TestTerminalClient*` cases pass with the race detector, including transient reconnect and input continuity.

Evidence: `.cache/sessions-cli-detach-{red,green}.log`; the unchanged real E2E-001 re-walk uses the rebuilt daemon/CLI in `.cache/sessions-detach-compozy` and the shared verification lock.

Real-run evidence: `.cache/sessions-cli-detach-bash-e2e.log` and its results/artifacts, 3/3 passed in 18.7s with the rebuilt daemon/CLI. Bash matches the CI shell. The separate default-Fish journal attribution failure also reproduces on the old binary; it is not a passing result or a regression from this repair.
