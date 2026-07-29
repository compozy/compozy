# BUG-20260729-nearest-workspace-case-alias: CWD discovery selects a broad workspace across case aliases

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, nearest-root resolution
- **Scenarios:** MS-workspace-resolution-chain, ET-native-workspace-scope-isolation
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-cross-workspace-access.md
- **Origin:** CH-workspace-binding-canary live QA

## Summary

`compozy workspace info` from a nested registered workspace can select a broader registered ancestor when the same case-insensitive filesystem path is spelled with a different case. The QA lab was registered under `/Users/pedronauck/dev/...`, while the process CWD was reported as `/Users/pedronauck/Dev/...`; lexical containment rejected the nearest root and fell through to `/Users/pedronauck`.

## Reproduction

- **Charter:** CH-workspace-binding-canary · **Tour:** Feature Tour
- **Environment:** fresh isolated `northstar-pay-20260729-124649-419333` lab; broad workspace `ws_123f5c59f2c50ec9`; nested workspace `ws_88c639b2d79cf1c5`.

1. Register a broad root and a nested root using a case alias accepted by a case-insensitive filesystem.
2. Enter a subdirectory below the nested root through the filesystem's canonical-case spelling.
3. Run `compozy workspace info -o json`.

**Expected:** Nearest-root discovery selects `ws_88c639b2d79cf1c5` with `resolution_source: cwd`.

**Actual:** Discovery selects broad workspace `ws_123f5c59f2c50ec9`.

## Evidence

- Explicit lowercase path and explicit nested ID both resolve to `ws_88c639b2d79cf1c5`.
- The uppercase canonical-case path resolves to `ws_123f5c59f2c50ec9` before the fix.
- Red regression: `TestResolveDiscoversNearestEnclosingWorkspace/Should_choose_the_nearest_registered_root_across_filesystem_case_aliases` returns `ws_outer`, not `ws_inner`.

## Fix

- **Root cause:** Confirmed: `pathIsWithinRoot` uses only lexical `filepath.Rel`, which cannot establish containment across case aliases on a case-insensitive filesystem.
- **Correction:** Lexical containment remains the fast path; when it misses, discovery walks
  existing ancestors and compares filesystem identity with `os.SameFile`, preserving nearest-root
  ordering across symlink and case aliases.
- **Fix commit:** `4e81f17`
- **Regression test:** the existing resolver nearest-root suite owns the filesystem-identity alias invariant.

## Verification

- The canonical nearest-root regression failed before the correction with `ws_outer` and passes
  afterward with `ws_inner`.
- `go test -race ./internal/workspace/...` passed 107 tests.
- The rebuilt isolated daemon resolved the uppercase `/Dev` CWD spelling to nested workspace
  `ws_88c639b2d79cf1c5` with `resolution_source: cwd`.
- The workspace catalog remained exactly 11 entries; no subdirectory registration was minted.
- `make lint`, `make test-e2e-runtime`, and `make test-e2e-web` passed after the source freeze.
