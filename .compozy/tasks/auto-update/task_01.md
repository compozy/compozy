---
status: pending
title: Implement CLI auto-update with upgrade command and background notification
type: backend
complexity: high
dependencies: []
---

# Task 1: Implement CLI auto-update with upgrade command and background notification

## Overview

Add the full auto-update system to the Compozy CLI: a new `internal/update` package for state persistence, version checking with 24h cache, and install-method-aware upgrade logic; a `compozy upgrade` Cobra command; and a background goroutine in `cmd/compozy/main.go` that notifies users of available updates on stderr after any command completes.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST add `creativeprojects/go-selfupdate` dependency via `go get`
- MUST create `internal/update/state.go` with `StateEntry` read/write using `gopkg.in/yaml.v3` and XDG-compliant path resolution (`$XDG_CONFIG_HOME/compozy/state.yml`, default `~/.config/compozy/state.yml`)
- MUST create `internal/update/check.go` with `CheckForUpdate` function that uses 24-hour cache, skips when `version.Version == "dev"` or `COMPOZY_NO_UPDATE_NOTIFIER` is set
- MUST create `internal/update/install.go` with `DetectInstallMethod` (Homebrew, NPM, go install, binary) and `Upgrade` function that prints package-manager commands for managed installs and performs in-place self-update for binary installs
- MUST configure `go-selfupdate` with `ChecksumValidator{UniqueFilename: "checksums.txt"}` and repo slug `compozy/compozy`
- MUST create `internal/cli/upgrade.go` as a Cobra subcommand registered in `internal/cli/root.go`
- MUST modify `cmd/compozy/main.go` to launch a background goroutine for version check before `cmd.Execute()` and print notification to stderr after execution
- MUST use `cmd.ErrOrStderr()` or `os.Stderr` for all update notifications — never stdout
- MUST use Lipgloss styles from `internal/charmtheme` for colored notification output
- MUST handle all errors silently in the background check goroutine (no user-visible failures)
- SHOULD respect `GITHUB_TOKEN` env var for higher API rate limits (handled by go-selfupdate automatically)
</requirements>

## Subtasks
- [ ] 1.1 Add `creativeprojects/go-selfupdate` dependency and create `internal/update/` package
- [ ] 1.2 Implement state file persistence with XDG path resolution and YAML read/write
- [ ] 1.3 Implement `CheckForUpdate` with 24h cache, dev-build skip, and env var opt-out
- [ ] 1.4 Implement `DetectInstallMethod` and `Upgrade` with per-method behavior
- [ ] 1.5 Create `compozy upgrade` Cobra command and register it in root command
- [ ] 1.6 Add background goroutine in `main.go` with channel-based notification after command execution
- [ ] 1.7 Write unit tests for all `internal/update/` functions

## Implementation Details

Create new package `internal/update/` with three files following the TechSpec 'System Architecture' section. The state file uses `gopkg.in/yaml.v3` (already in go.mod) for marshaling. XDG path resolution follows the pattern in `internal/setup/agents.go:126-129`. The Cobra command follows the simple command pattern from `internal/cli/commands_simple.go`. The background goroutine follows the gh CLI pattern described in TechSpec 'System Architecture' data flow.

See TechSpec sections: 'Core Interfaces' for type definitions, 'Data Models' for state file format, 'Integration Points' for go-selfupdate configuration.

### Relevant Files
- `internal/version/version.go` — Read `Version` variable for current version comparison
- `internal/setup/agents.go:105-148` — XDG path resolution pattern (`resolveEnvironment`)
- `internal/cli/commands_simple.go:43-68` — Simple Cobra command pattern (`newMigrateCommand`)
- `internal/cli/root.go:84-94` — Command registration via `root.AddCommand()`
- `internal/cli/exit.go` — `withExitCode()` error pattern
- `internal/charmtheme/theme.go` — Lipgloss color constants (`ColorWarning`, `ColorBrand`, `ColorMuted`)
- `internal/cli/theme.go` — CLI styling patterns
- `cmd/compozy/main.go` — Entry point to modify with background goroutine
- `command/command.go` — `ExitCode()` extraction (no changes needed, reference only)
- `.goreleaser.yml:67-69` — Confirms `checksums.txt` with sha256

### Dependent Files
- `cmd/compozy/main.go` — Modified to add goroutine and notification logic
- `internal/cli/root.go` — Modified to register `newUpgradeCommand()`
- `go.mod` / `go.sum` — Modified by `go get` for new dependency

### Related ADRs
- [ADR-001: Use creativeprojects/go-selfupdate](adrs/adr-001.md) — Library choice and ChecksumValidator configuration
- [ADR-002: Background Version Check with 24h Cache in main.go](adrs/adr-002.md) — Goroutine pattern, state file location, skip conditions
- [ADR-003: Detect Installation Method](adrs/adr-003.md) — Detection heuristics and per-method upgrade behavior

## Deliverables
- `internal/update/state.go` — StateEntry type, ReadState, WriteState, StateFilePath
- `internal/update/check.go` — ReleaseInfo type, CheckForUpdate, ShouldCheckForUpdate
- `internal/update/install.go` — InstallMethod type, DetectInstallMethod, Upgrade
- `internal/update/state_test.go` — State persistence tests
- `internal/update/check_test.go` — Version check and cache tests
- `internal/update/install_test.go` — Install detection tests
- `internal/cli/upgrade.go` — Cobra upgrade command
- Modified `internal/cli/root.go` — Upgrade command registration
- Modified `cmd/compozy/main.go` — Background goroutine and notification
- Unit tests with 80%+ coverage **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `ReadState` returns nil for missing state file (no error)
  - [ ] `ReadState` returns nil for corrupted YAML file (no error)
  - [ ] `ReadState` / `WriteState` round-trip preserves all fields via `t.TempDir()`
  - [ ] `StateFilePath` returns `$XDG_CONFIG_HOME/compozy/state.yml` when env var is set
  - [ ] `StateFilePath` returns `~/.config/compozy/state.yml` when `$XDG_CONFIG_HOME` is unset
  - [ ] `CheckForUpdate` skips check when `version == "dev"`
  - [ ] `CheckForUpdate` skips check when `COMPOZY_NO_UPDATE_NOTIFIER` is set
  - [ ] `CheckForUpdate` skips check when state is less than 24h old (cache hit)
  - [ ] `CheckForUpdate` queries GitHub when state is older than 24h (cache miss)
  - [ ] `CheckForUpdate` returns nil when current version >= latest version
  - [ ] `CheckForUpdate` returns ReleaseInfo when latest version > current version
  - [ ] `DetectInstallMethod` returns `InstallHomebrew` for path containing `/Cellar/`
  - [ ] `DetectInstallMethod` returns `InstallNPM` for path containing `node_modules`
  - [ ] `DetectInstallMethod` returns `InstallGo` for path under `$GOPATH/bin`
  - [ ] `DetectInstallMethod` returns `InstallBinary` for unknown path (default fallback)
  - [ ] `Upgrade` prints `brew upgrade --cask compozy` for Homebrew installs
  - [ ] `Upgrade` prints `npm install -g @compozy/cli@latest` for NPM installs
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` passes (fmt + lint + test + build)
- `compozy upgrade` command appears in `compozy --help` output
- Running any command with a stale/missing state file triggers a background GitHub API check
- Running any command with a fresh state file (<24h) skips the network call
- Update notification appears on stderr after command execution when a newer version exists
- `COMPOZY_NO_UPDATE_NOTIFIER=1` suppresses all update checks
- Dev builds (`version == "dev"`) never check for updates
