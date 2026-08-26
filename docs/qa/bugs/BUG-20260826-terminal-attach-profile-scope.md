# BUG-20260826-terminal-attach-profile-scope: CLI attach loses the selected terminal profile

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-switch-profile-terminal-scope, attach to a terminal owned by a named profile
- **Scenarios:** ET-terminal-profile-selectors; ET-terminal-profile-segmentation; ET-terminal-cli-public-contract
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-integrated-terminal.md

## Summary

The CLI can list and inspect a terminal under a named profile, but `terminal attach --profile <name>`
opens the stream without that profile selector. The daemon therefore resolves the WebSocket against the
default profile and returns `terminal_not_found` after the CLI has already announced the takeover.

## Reproduction

- **Charter:** CH-terminal-profile-fence · **Tour:** Garbage Tour
- **Environment:** isolated macOS runtime / CLI / wifi-fast / en-US

1. Create profiles `qa-alpha` and `qa-beta`.
2. Open a detached terminal under `qa-alpha`.
3. Confirm `terminal get --profile qa-alpha` returns the running terminal.
4. Run `terminal attach <id> --profile qa-alpha --control`.

**Expected:** The local WebSocket resolves `qa-alpha` and attaches to the terminal.
**Actual:** The CLI prints the control-taken banner and then exits with `terminal not found`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/terminal-open-alpha.json`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/terminal-get.json`
- Interactive CLI reproduction against terminal `term-41799a9db0df`.

## Fix

- **Root cause:** HTTP requests pass through `profileQueryValues`, but the terminal WebSocket client
  built its URL directly and omitted the resolved profile query for local UDS connections.
- **Fix commit:** current branch
- **Regression test:** `internal/cli/client_terminal_stream_test.go` —
  `TestTerminalClientStreamTargetShouldCarryProfileScope`.

## Verification

- **Retested:** focused race test and the isolated CLI profile walk pass.
- **Evidence:** `go test -race ./internal/cli -run 'TestTerminalClientStream(TargetShouldCarryProfileScope|ShouldTakeOverBeforeWriteAttach)' -count=1`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/terminal-open-alpha.json`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/terminal-get.json`
