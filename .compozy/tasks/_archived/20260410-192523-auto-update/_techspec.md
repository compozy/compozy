# TechSpec: CLI Auto-Update

## Executive Summary

Add two capabilities to the Compozy CLI: (1) a `compozy upgrade` command that updates the binary to the latest release, and (2) a background version check that notifies users of available updates when running any command.

The implementation uses `creativeprojects/go-selfupdate` for release detection and binary replacement, integrated with GoReleaser's existing `checksums.txt` validation. A background goroutine in `cmd/compozy/main.go` performs the check in parallel with command execution, using a 24-hour YAML-based cache at `~/.config/compozy/state.yml` to avoid excessive GitHub API calls. Installation method detection (Homebrew, NPM, go install, binary) determines whether `compozy upgrade` performs in-place self-update or prints the appropriate package manager command.

**Primary trade-off**: Adding a network call on every CLI invocation (cached at 24h) in exchange for keeping users informed about security patches and feature releases. The goroutine pattern ensures zero latency impact on command execution.

## System Architecture

### Component Overview

```
cmd/compozy/main.go
  |
  |-- [goroutine] update.CheckForUpdate()
  |     |-- Read ~/.config/compozy/state.yml
  |     |-- (if stale) Query GitHub Releases API
  |     |-- Write state.yml
  |     \-- Return *ReleaseInfo via channel
  |
  |-- cmd.Execute()  (normal Cobra flow)
  |
  \-- [after Execute] Print update notification to stderr

internal/update/
  |-- check.go        -- CheckForUpdate, state cache, ShouldCheckForUpdate
  |-- check_test.go
  |-- install.go      -- DetectInstallMethod, upgrade command logic
  |-- install_test.go
  |-- state.go        -- StateEntry read/write, XDG path resolution
  \-- state_test.go

internal/cli/
  \-- upgrade.go      -- Cobra "upgrade" subcommand
```

**Data flow:**
1. `main.go` launches `update.CheckForUpdate()` in a goroutine with a buffered channel
2. `cmd.Execute()` runs the user's command normally
3. After `Execute()` returns, `main.go` cancels the check context and reads the channel
4. If a newer version exists, a colored notification is printed to stderr
5. `compozy upgrade` calls `update.Upgrade()` which detects install method and either self-updates or prints instructions

## Implementation Design

### Core Interfaces

```go
// internal/update/check.go

// ReleaseInfo holds metadata about a GitHub release.
type ReleaseInfo struct {
    Version     string    `yaml:"version"`
    URL         string    `yaml:"url"`
    PublishedAt time.Time `yaml:"published_at"`
}

// CheckForUpdate queries GitHub for a newer release.
// Returns nil if no update is available or check was
// skipped (cache fresh, dev build, or env var set).
func CheckForUpdate(
    ctx context.Context,
    currentVersion string,
    stateFilePath string,
) (*ReleaseInfo, error)
```

```go
// internal/update/install.go

// InstallMethod identifies how the CLI binary was installed.
type InstallMethod int

const (
    InstallBinary   InstallMethod = iota // direct download
    InstallHomebrew                      // brew install --cask
    InstallNPM                           // npm install -g
    InstallGo                            // go install
)

// DetectInstallMethod inspects the executable path to
// determine the installation channel.
func DetectInstallMethod() InstallMethod

// Upgrade performs or suggests the appropriate upgrade.
// Returns the upgrade message for package-manager installs,
// or performs in-place update for binary installs.
func Upgrade(ctx context.Context, currentVersion string) error
```

### Data Models

**State file** (`~/.config/compozy/state.yml`):

```yaml
checked_for_update_at: 2026-04-06T15:30:00Z
latest_release:
  version: "1.2.3"
  url: "https://github.com/compozy/compozy/releases/tag/v1.2.3"
  published_at: 2026-04-05T10:00:00Z
```

**StateEntry struct:**

```go
// internal/update/state.go

type StateEntry struct {
    CheckedForUpdateAt time.Time   `yaml:"checked_for_update_at"`
    LatestRelease      ReleaseInfo `yaml:"latest_release"`
}
```

### API Endpoints

No new API endpoints. The implementation consumes the GitHub Releases API via `go-selfupdate`:
- `GET https://api.github.com/repos/compozy/compozy/releases/latest` (unauthenticated, 60 req/hour limit)

## Integration Points

### GitHub Releases API

- **Purpose**: Detect latest release version and download assets
- **Authentication**: Unauthenticated (sufficient for public repo with 24h cache). If `GITHUB_TOKEN` is set, `go-selfupdate` uses it automatically for higher rate limits.
- **Error handling**: All errors are silently swallowed in the background check (no user impact). The `compozy upgrade` command surfaces errors normally.
- **Retry strategy**: None needed — single attempt per 24h window. On failure, the stale cache is preserved and the check retries on the next invocation after 24h.

### GoReleaser Assets

- **Checksum validation**: `ChecksumValidator{UniqueFilename: "checksums.txt"}` validates SHA256 against GoReleaser's checksum file
- **Archive formats**: tar.gz (Linux/macOS), zip (Windows) — handled automatically by `go-selfupdate`
- **Cosign signatures**: Not validated during self-update (cosign verification requires the cosign binary which users may not have). Checksum validation provides integrity assurance.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|---------------------|-----------------|
| `cmd/compozy/main.go` | Modified | Add goroutine for background check and post-execute notification. Low risk — isolated from command logic. | Add ~20 lines wrapping `cmd.Execute()` |
| `internal/cli/root.go` | Modified | Register `upgrade` subcommand. Minimal risk — single `root.AddCommand()` call. | Add `newUpgradeCommand()` to command list |
| `internal/update/` | New | New package with check, state, and install logic. No risk to existing code. | Create package with 3 files |
| `internal/cli/upgrade.go` | New | Cobra command definition for `compozy upgrade`. No risk to existing code. | Create single file |
| `go.mod` | Modified | Add `creativeprojects/go-selfupdate` dependency. Low risk — isolated to new package. | `go get github.com/creativeprojects/go-selfupdate` |
| `internal/version/version.go` | Unchanged | Read-only access to `Version` variable. No changes needed. | None |

## Testing Approach

### Unit Tests

**`internal/update/check_test.go`:**
- Test `CheckForUpdate` with fresh state (should query)
- Test `CheckForUpdate` with recent state (should skip, return nil)
- Test `CheckForUpdate` with stale state (>24h, should query)
- Test `CheckForUpdate` with `version.Version == "dev"` (should skip)
- Test version comparison logic (current >= latest returns nil)
- Mock the GitHub API via `go-selfupdate`'s `Source` interface

**`internal/update/state_test.go`:**
- Test `ReadState` / `WriteState` round-trip with `t.TempDir()`
- Test `ReadState` with missing file (returns nil, no error)
- Test `ReadState` with corrupted file (returns nil, no error)
- Test `StateFilePath` XDG resolution with/without `$XDG_CONFIG_HOME`

**`internal/update/install_test.go`:**
- Test `DetectInstallMethod` with Homebrew-style path
- Test `DetectInstallMethod` with NPM-style path
- Test `DetectInstallMethod` with `$GOPATH/bin` path
- Test `DetectInstallMethod` with unknown path (falls back to binary)

### Integration Tests

Not required for this feature. The `go-selfupdate` library handles GitHub API integration. Unit tests with mocked sources provide sufficient coverage. The `compozy upgrade` command can be manually tested against a real release.

## Development Sequencing

### Build Order

1. **`internal/update/state.go`** — State file read/write, XDG path resolution. No dependencies.
2. **`internal/update/check.go`** — `CheckForUpdate` function with 24h cache logic. Depends on step 1 (state persistence).
3. **`internal/update/install.go`** — `DetectInstallMethod` and `Upgrade` function using `go-selfupdate`. Depends on step 2 (uses `CheckForUpdate` internally for version detection in upgrade).
4. **`internal/cli/upgrade.go`** — Cobra command wiring. Depends on step 3 (calls `update.Upgrade`).
5. **`internal/cli/root.go`** — Register upgrade command. Depends on step 4.
6. **`cmd/compozy/main.go`** — Background goroutine and notification. Depends on step 2 (calls `update.CheckForUpdate`).

### Technical Dependencies

- `go get github.com/creativeprojects/go-selfupdate` must be run before step 2
- No infrastructure or external service dependencies beyond the existing GitHub Releases API

## Monitoring and Observability

- **Structured logging**: `slog.Debug` for update check lifecycle (skip reason, version found, cache hit/miss). Only emitted at debug level to avoid noise.
- **Stderr notification**: Colored message `"A new release of compozy is available: X.Y.Z → A.B.C\nRun 'compozy upgrade' to update\n"` printed to stderr after command execution.
- **No metrics or alerting**: This is a local CLI feature with no server-side component.

## Technical Considerations

### Key Decisions

- **24-hour cache interval**: Balances freshness with API rate limits. GitHub allows 60 unauthenticated requests/hour; with 24h cache, even heavy CLI usage stays well under limit.
- **Stderr for notifications**: Ensures update messages never interfere with piped output (`compozy exec --format json | jq`).
- **Silent failure in background check**: Any error in the goroutine is swallowed — the user's command must never fail because of an update check problem.
- **No cosign verification during self-update**: SHA256 checksums provide integrity. Requiring cosign would add a binary dependency most users don't have.

### Known Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| GitHub API rate limiting | Low (24h cache) | Cache prevents repeated calls. `GITHUB_TOKEN` available as escape hatch. |
| Binary permission issues on self-update | Medium (Linux/macOS) | `go-selfupdate` handles atomic replace. Fallback: print error with manual instructions. |
| Detection heuristic misidentifies install method | Low | Conservative detection; unknown paths fall back to self-update (safe default). |
| State file directory doesn't exist | Low | Create `~/.config/compozy/` on first write with `os.MkdirAll`. |

## Architecture Decision Records

- [ADR-001: Use creativeprojects/go-selfupdate for CLI Auto-Update](adrs/adr-001.md) — Chose go-selfupdate for native GoReleaser integration over stdlib+minio or rhysd alternatives.
- [ADR-002: Background Version Check with 24h Cache in main.go](adrs/adr-002.md) — Goroutine in main.go with YAML state cache over Cobra hooks or command wrapper.
- [ADR-003: Detect Installation Method and Suggest Appropriate Upgrade Command](adrs/adr-003.md) — Auto-detect Homebrew/NPM/go-install and print correct command; self-update only for binary downloads.
