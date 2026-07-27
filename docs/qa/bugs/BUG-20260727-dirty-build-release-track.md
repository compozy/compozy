# BUG-20260727-dirty-build-release-track: Local candidate builds cannot start the daemon

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-validate-compozy-hard-cut, source build and fresh daemon boot
- **Scenarios:** RT-compozy-cli-binary; RT-compozy-home-layout
- **Found:** 2026-07-27 · **Report:** docs/qa/reports/2026-07-27-devtool-oss-launch.md
- **Origin:** n/a

## Summary

Ada can build the current source successfully, but that candidate binary exits before daemon
readiness whenever the worktree is dirty. Local development and release-candidate QA therefore
cannot reach any runtime surface from the normal source build.

## Reproduction

- **Charter:** CH-compozy-platform-hard-cut · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US

1. Build the current source with `make build` while the worktree has tracked changes.
2. Start `./bin/compozy daemon start -o json` with a fresh isolated `COMPOZY_HOME`, HTTP port, and
   UDS path.
3. Observe the detached process exit before readiness.

**Expected:** The source-built candidate starts normally and classifies an unpublishable dirty
build as development state for update checks.

**Actual:** Startup fails with `unsupported prerelease track "16-gb2ad2446-dirty"` before the
daemon becomes reachable.

## Evidence

- Build: `make build` completed successfully from candidate
  `b2ad244622142ed97f2b5b170a5267bbbb50d359`.
- Boot transcript: lab command returned
  `daemon: create settings update manager: update: resolve release track: unsupported prerelease
  track "16-gb2ad2446-dirty"`.
- Lab manifest:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-120735-072314-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Fix

- **Root cause:** `isDevVersion` recognizes a bare dirty commit as development state but does not
  recognize the valid `git describe --dirty` form for a build made after a tag. The update manager
  therefore routes the dirty source build into published-channel parsing, where the Git distance
  and dirty suffix are misclassified as a release prerelease track.
- **Fix commit:** pending
- **Regression test:** `internal/update/version_test.go` — dirty tagged Git-describe builds must be
  classified as development builds.

## Verification

- **Retested:** 2026-07-27 in the original isolated lab after rebuilding the hard-cut candidate.
- **Result:** Pass. `version-live.json` reports `v0.2.15-16-gb2ad2446-dirty`; the daemon reached
  ready state on port 61235 and `status-live.json` reports the same dirty development version.
