# BUG-20260826-terminal-cli-raw-mode: CLI detach chord terminates the client

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-operate-terminal-by-cli, attach interactively and detach without stopping the shell
- **Scenarios:** ET-terminal-cli-public-contract; ET-terminal-stream-resilience
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-integrated-terminal.md

## Summary

`terminal attach` advertises `Ctrl-\\ Ctrl-\\` as its detach chord, but the CLI leaves the local TTY
in cooked mode. On macOS the first byte is handled as `SIGQUIT`, terminating the Compozy client instead
of sending the detach frame. Other control bytes are likewise interpreted locally rather than reaching
the remote PTY.

## Reproduction

- **Charter:** CH-terminal-cli-transport-parity · **Tour:** Feature Tour
- **Environment:** isolated macOS runtime / interactive CLI / wifi-fast / en-US

1. Open a detached interactive terminal.
2. Attach with `terminal attach <id> --control`.
3. Run a command and confirm its output is streamed.
4. Press `Ctrl-\\ Ctrl-\\` as printed by the CLI.

**Expected:** The CLI sends one protocol detach frame, restores the local TTY, and leaves the remote
shell running.
**Actual:** The operating system delivers `SIGQUIT`; the Go client exits with a fatal signal-stack
panic while the remote shell remains running.

## Evidence

- Interactive CLI reproduction against terminal `term-d8c9b0853ab6` in the isolated QA lab.
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/terminal-journal-default.json`
  proves the remote command completed before the detach attempt.

## Fix

- **Root cause:** interactive open/attach called the stream client without entering raw terminal mode.
- **Fix commit:** current branch
- **Regression test:** `internal/cli/client_terminal_stream_test.go` —
  `TestTerminalRawInputShouldRestoreTheLocalTerminal`.

## Verification

- **Retested:** focused race test and isolated interactive attach/detach pass.
- **Evidence:** `go test -race ./internal/cli -run 'TestTerminal(ClientStream(TargetShouldCarryProfileScope|ShouldTakeOverBeforeWriteAttach)|RawInputShouldRestoreTheLocalTerminal)' -count=1`
