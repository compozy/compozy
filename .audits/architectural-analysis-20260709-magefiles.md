# Architectural Analysis Report

**Date**: 2026-07-09
**Files Analyzed**: 25 Go files under `magefiles/` (post-move)
**Dead Code Files**: 0
**Duplication Groups**: 1 (conceptual — git helpers vs web-assets git usage)

---

## Executive Summary

- **Dead Code**: 0 files, 0 unused Mage targets (all exported targets listed by `mage -l`)
- **Duplicated Functionality**: 1 conceptual group (git command wrappers centralized in `git.go`)
- **Architectural Anti-Patterns**: 1 god object (pre-move `magefile.go`) — remediated
- **Type Issues**: N/A for this Go package (no `any` / `@ts-ignore`)
- **Code Smells**: Large module (fixed); residual long data table in `Boundaries`

**Estimated Cleanup**: ~0 lines dead; structure cleanup moved ~3100 LOC out of repo root and under 500-line files

---

## Dead Code

### Completely Dead Files (DELETE)

| File       | Reason | Confidence |
| ---------- | ------ | ---------- |
| None found | —      | —          |

**Total Lines**: 0

### Dead Exports (REMOVE)

| File       | Export | Reason                                                        |
| ---------- | ------ | ------------------------------------------------------------- |
| None found | —      | Mage targets are CLI entrypoints; helpers are package-private |

### Possibly Dead (VERIFY)

| File       | Export | Reason | Verification Needed |
| ---------- | ------ | ------ | ------------------- |
| None found | —      | —      | —                   |

---

## Duplication

### Group: Git command helpers

**Type**: Conceptual Duplication (resolved by extraction)
**Instances**:

- `magefiles/git.go` — `gitShowFile`, `gitHasDiff`, `gitCommandOutput`, `gitTags`, `gitOutput`
- Callers in `web_assets_publish.go`, `build.go`

**Analysis**: Pre-move helpers lived inline in the god file next to web-assets publish. Extracted to `git.go`.
**Recommendation**: Keep `git.go` as the single place for git subprocess helpers.
**Impact**: One place to harden timeouts/error wrapping later.

---

## Architectural Anti-Patterns

### God Object / God File (REMEDIATED)

- **Pre-move**: `magefile.go` (1820 lines) — verify, codegen, web, assets publish, Daytona, E2E, boundaries
- **Post-move**: split across `magefiles/*.go`, largest production file `web_assets.go` (~409 lines)

### Circular Dependencies

- None found within `magefiles/` (single package)

### Tight Coupling

- Acceptable: Mage targets shell out to `go`, `bun`, `git`, `sh` with repo-relative paths
- Working directory contract: Mage uses `-w .` (repo root); tests use `TestMain` chdir to module root

### Layer Violations

- Mage package imports selected `internal/*` helpers (`openapits`, `e2elane`) — intentional for codegen/E2E lanes
- Does not import `internal/daemon` composition root

---

## Type Issues

None found (Go; build-tag gated package).

---

## Code Smells

| Smell                | Location                        | Notes                   |
| -------------------- | ------------------------------- | ----------------------- |
| Large Module         | pre-move `magefile.go`          | Fixed                   |
| Long Function (data) | `boundaries.go` `Boundaries`    | Rule table; acceptable  |
| Magic strings        | path constants in `defaults.go` | Already named constants |

---

## Post-Move Layout

```
magefiles/
  defaults.go              # consts, shared types, Default=Verify
  deps_lint.go             # Deps, Fmt, Lint, Modernize
  test.go / gotest_lane.go # unit/integration lanes + hermetic env
  e2e.go                   # E2E lanes + lane binaries
  codegen.go / codegen_guard.go
  web.go / web_assets.go / web_assets_publish.go
  daytona.go / daytona_stamp.go
  install.go / boundaries.go / verify.go / verifylock.go
  build.go / command.go / git.go
  *_test.go + testmain_test.go
```

**Constraints retained**:

- `//go:build mage` on every file (excluded from `go list/test ./...`)
- Exported Mage target names unchanged for Makefile/CI

---

## Impact Assessment

- Root directory: −10 Mage-tagged files
- Hard-cap compliance: all production files ≤409 lines
- Operator API: `make *` / `mage <target>` unchanged
- Test invocation: `go test -tags mage -race ./magefiles`
