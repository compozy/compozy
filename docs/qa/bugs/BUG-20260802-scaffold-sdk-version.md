# BUG-20260802-scaffold-sdk-version: Go extension templates referenced an unpublished SDK version

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, scaffold and build a code-first provider
- **Scenarios:** ET-extension-code-first-authoring
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 real-user QA

## Summary

`compozy extension init ... -t tool-provider-go` generated a module that required
`github.com/compozy/compozy/sdk/go v0.3.0-beta.1`, which was not published. The documented first
build failed at `go mod tidy` before extension code could compile.

## Reproduction

1. Scaffold a Go tool provider with the embedded template.
2. Run `go mod tidy` in the generated extension.
3. Observe `unknown revision sdk/go/v0.3.0-beta.1`.

**Expected:** Every embedded Go template uses the current published SDK and builds from a fresh
scaffold.
**Actual:** Both templates pinned an unavailable version.

## Evidence

- `go list -m -versions github.com/compozy/compozy/sdk/go` returned `v0.3.0-beta.3` as the
  published version.
- Fresh scaffold `bundles-removal-provider-v2` completed `go mod tidy`, build, validation, install,
  enable, native catalog discovery, and tool invocation after the fix.

## Fix

- **Root cause:** The SDK version was duplicated as stale literal text in embedded templates.
- **Correction:** Scaffolding owns one `scaffoldGoSDKVersion` constant and replaces a shared
  template placeholder in every Go template.
- **Fix commit:** `7866661`
- **Regression test:** `internal/extension/build_test.go` verifies all embedded Go templates render
  the published version with no unresolved placeholder.

## Verification

- The canonical scaffold test passed under `-race`.
- A fresh generated provider resolved `v0.3.0-beta.3`, built, validated with zero issues, installed,
  enabled, registered its tool, and returned a real invocation result.

