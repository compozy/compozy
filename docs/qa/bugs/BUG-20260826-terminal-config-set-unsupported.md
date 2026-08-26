# BUG-20260826-terminal-config-set-unsupported: terminal settings cannot be changed through the CLI

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-terminal-operator-control, change terminal recording policy before opening a terminal
- **Scenarios:** ET-terminal-cli-public-contract; ET-terminal-journal-recording
- **Found:** 2026-08-26 · **Report:** docs/qa/reports/2026-08-26-integrated-terminal.md

## Summary

`config show` and `config get` expose the public `terminal.*` settings, but `config set` rejects every
terminal path as unsupported. Operators therefore cannot manage terminal policy through the CLI even
though the daemon accepts the same configuration through files.

## Reproduction

- **Charter:** CH-terminal-capacity-config · **Tour:** Feature Tour
- **Environment:** isolated macOS runtime / CLI / wifi-fast / en-US

1. Run `compozy config get terminal.recording --scope user` and observe `false`.
2. Run `compozy config set terminal.recording true --scope user`.

**Expected:** The user configuration persists `terminal.recording = true` and a subsequent read returns
`true`.
**Actual:** The CLI exits with `config path \"terminal.recording\" is not supported by config set`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/config-terminal-recording-before.json`
- Isolated CLI reproduction against the QA home.

## Fix

- **Root cause:** The CLI's typed mutation registry was not extended with the ten public terminal
  configuration paths when the terminal configuration surface was added.
- **Fix commit:** current branch
- **Regression test:** `internal/cli/config_test.go` —
  `TestTerminalConfigSetPathsMatchPublicSurface` and the workspace-scope round-trip in
  `TestConfigCommandsUseWorkspaceScopeAndValidateBeforeWriting`.

## Verification

- **Retested:** focused race tests and the isolated CLI configuration lifecycle pass.
- **Evidence:** `go test -race ./internal/cli -run 'Test(ConfigCommandsUseWorkspaceScopeAndValidateBeforeWriting|TerminalConfigSetPathsMatchPublicSurface)' -count=1`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-20260826-074528-452132-lab/qa-artifacts/qa/config-terminal-recording-before.json`
