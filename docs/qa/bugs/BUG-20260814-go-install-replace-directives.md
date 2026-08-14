# BUG-20260814-go-install-replace-directives: Published beta cannot be installed through the documented Go channel

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-evaluate-compozy-beta, step 4
- **Scenarios:** REL-beta-install-paths
- **Found:** 2026-08-14 · **Report:** docs/qa/reports/2026-08-13-release-pipeline-recovery.md
- **Origin:** n/a

## Summary

Dora can install beta.16 through the hosted installer and npm, but the pinned Go command fails
before compilation because the published module contains `replace` directives.

## Reproduction

- **Charter:** CH-beta-install-channels · **Tour:** Feature Tour
- **Environment:** macOS arm64 / Go module proxy / en-US

1. Set `GOBIN` to an empty disposable directory.
2. Run `go install github.com/compozy/compozy/cmd/compozy@v0.3.0-beta.16`.

**Expected:** Go installs the same `0.3.0-beta.16` binary exposed by the hosted installer and npm.

**Actual:** Go rejects the published module because its `go.mod` contains `replace` directives.

## Evidence

- https://github.com/compozy/compozy/releases/tag/v0.3.0-beta.16
- Go reported: `The go.mod file for the module providing named packages contains one or more replace directives.`

## Fix

- **Root cause:** the release module retains development-only replacement directives that remote
  `go install ...@version` explicitly forbids.
- **Fix commit:** pending
- **Regression test:** pending canonical release-module suite selection

## Verification

- **Retested:** pending a new immutable beta after the production module is corrected.
- **Result:** blocked; beta.16 cannot be changed after publication.
