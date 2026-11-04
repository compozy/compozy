## What & why

CompozyOS had Windows runtime support but no distributable Windows desktop. Electron packaging,
release inventory and update planning only handled macOS and Linux, while the app control server
required a Unix socket and POSIX permissions. A Windows package would therefore fail at startup
before exposing the product.

This change adds a Windows x64 desktop using the existing Electron shell and bundled Go daemon:

- Per-user NSIS installation, Start-menu/desktop shortcuts, `compozyos://` registration and Windows
  application identity. Installation does not require elevation; uninstall retains user data.
- Authenticated named-pipe app control. A private `app.sock` discovery file replaces the Unix socket
  only on Windows. The Go client accepts only local CompozyOS pipe addresses and verifies the server
  process user before sending the rotated token. Desktop directories establish Windows DACLs.
  The pipe DACL admits only its owner; recreated directories regain private permissions and corrupt
  discovery metadata recovers on startup.
- Windows-capable directory synchronization during first-run provisioning and update-journal writes,
  using the existing filesystem helper.
- NSIS update asset selection, silent native installation after explicit Update activation, and
  app/runtime relaunch through the existing journal and bootstrap lifecycle.
- Runtime integrity manifests refreshed after Authenticode signing, preserving verification of the
  actual bundled executable.
- Windows installer and blockmap in the exact release inventory, GoReleaser checksums/signatures,
  artifact collection and channel publication (`latest.yml`). Retries and rollback preserve historical
  macOS/Linux generations, including removal of a Windows feed when restoring a pre-Windows release.
  Windows repair verifies remote installer bytes against the restored manifest SHA-512.
- Native Windows PR CI with an ephemeral signing identity, real installer/update fixtures, focused
  backend/control checks and existing packaged startup/deep-link journeys. Release and release
  dry-run jobs also install and launch the transferred Windows artifact.
- Windows installation/build/signing documentation, operational skill guidance, QA scenarios and a
  cross-surface impact audit.

Authored and reviewed locally by Codex in the isolated `feat/windows-desktop` worktree.

## How you verified it

Passing local evidence so far:

- Windows amd64 cross-build of `cmd/compozy` and cross-compilation of the CLI test binary.
- Desktop typecheck through root Turborepo.
- Root Turborepo focused control, release and installer-policy suites: 27 tests passed; one native
  Windows case is skipped locally and runs in Windows CI.
- Race-enabled focused Go release inventory/authority, update planning and update-journal tests.
- Focused TypeScript lint, Go lint for macOS and Windows targets, and Actions workflow validation.
- Focused Go CLI bootstrap/provisioning tests with real process execution.
- Race-enabled release workflow contract tests, including all nine compatibility inventory entries.

The Go test-shape checker passes the changed CLI and release suites. Its whole-file scan also flags
the unchanged Homebrew renderer test in `internal/config/release_config_test.go`; this pre-existing
shape finding is outside the desktop changes.

The Windows installer/update/uninstall journey and all repository-wide gates run in CI. Native
Windows results, final-head CI and review receipts are pending; cross-compilation is not runtime
proof. The installed-app journey checks a path with spaces and non-ASCII characters, same-home CLI
control, actual NSIS update/relaunch, matching daemon version, archived update success and retained
configuration/workspace data after uninstall.

The first CI pass also exposed passive SQLite checkpoint timeouts in the thirty-session load lane.
The storage code and that scenario are unchanged by this PR; the failure remains under investigation.

## Impact

See `docs/_memory/change-impact.md`, “Windows desktop distribution”.

No CLI verbs, HTTP/UDS DTOs, native tool IDs, extension APIs, hooks or configuration keys change.
There is no database migration. macOS/Linux controls and existing update tracks remain supported.
Windows standalone runtime archives retain the documented manual replacement path. Windows x64
is the native desktop target; no native ARM64 installer is published by this change.

Production release signing reuses the five existing organization `AZURE_*` secrets through
`electron-builder`'s native Azure Trusted Signing support. The public account endpoint and exact
certificate CN are supplied by repository variables `AZURE_SIGNING_ENDPOINT` and
`AZURE_PUBLISHER_NAME`. No new PFX secrets are required. Organization-secret access and the
publisher metadata must be available before the first signed release. The release job fails if signing
is unavailable or produced signatures are invalid. CI uses a disposable test certificate.
This PR prepares distribution; it does not publish a product release or rotate credentials.

- [x] Focused checks pass locally; delivery also requires CI green on the final head
- [x] New behavior is covered by the owning release, control, bootstrap and update suites
- [x] Agent authorship and verification scope are stated above
