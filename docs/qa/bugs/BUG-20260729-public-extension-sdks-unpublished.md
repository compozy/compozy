# BUG-20260729-public-extension-sdks-unpublished: Published quickstart depends on unavailable SDK releases

- **Status:** verified
- **Impact (user-side):** Blocks newcomer first success
- **Severity:** Critical · **Priority:** P0
- **Personas Affected:** Lea; Ada
- **Journey Step:** J-extension-newcomer-first-success, run the generated extension outside the repository
- **Scenarios:** ET-extension-quickstart-verbatim; ET-extension-dx-scorecard
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-ext-improvs.md
- **Origin:** Task 11 release-stamped external-workspace replay

## Summary

The public quickstart generates a Go extension that requires `github.com/compozy/compozy/sdk/go v0.3.0-beta.1`, but that nested module version is not published. The TypeScript SDK named by the same public authoring surface is also absent from npm. A newcomer can scaffold the project, but cannot start the generated extension without adding an undocumented repository-local dependency override.

## Reproduction

1. From a directory outside the Compozy repository, run `compozy extension init hello --template tool-provider-go` with the release-stamped binary.
2. Run `compozy extension dev hello` exactly as documented.
3. Independently query the Go module and npm registries for the generated SDK coordinates.

**Expected:** The generated project resolves only published dependencies and reaches a live dev generation.
**Actual:** Go reports unknown revision `sdk/go/v0.3.0-beta.1`; npm reports `@compozy/extension-sdk` as not found.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/newcomer/quickstart.json`
- Generated module: `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/project/hello/go.mod`
- `go list -m github.com/compozy/compozy/sdk/go@v0.3.0-beta.1` fails with unknown revision `sdk/go/v0.3.0-beta.1`.
- `npm view @compozy/extension-sdk` returns registry HTTP 404.

## Fix

- **Root cause:** The release tag and package publication state do not include the nested Go module tag or npm package required by the generated templates.
- **Correction:** Pending external release publication. Repository-local `replace` directives, local proxies, or unpublished example substitutions are not acceptable release evidence.
- **Fix commit:** not applicable until the SDK artifacts are published
- **Regression gate:** The release workflow already owns SDK co-ship verification; a release candidate must additionally prove the public registry coordinates from a clean external workspace.

## Verification

- **Result:** BLOCKED. Re-run the verbatim quickstart against the published Go and TypeScript SDK versions after registry publication.
