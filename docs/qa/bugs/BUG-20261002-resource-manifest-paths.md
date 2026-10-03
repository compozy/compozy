# BUG-20261002-resource-manifest-paths: The documented passive extension cannot be built

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-dev-lifecycle, build and dev-link a passive agent kit
- **Scenarios:** ET-resource-only-extension-dev
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

An extension author following the public resource-only guide cannot link the kit: the CLI rejects
its documented `agents = ["agents"]` declaration before building or publishing any resources.

## Reproduction

- **Charter:** CH-resource-only-extension-dev · **Tour:** Feature Tour
- **Environment:** Isolated macOS lab, real CLI/daemon, en-US, Studio Operations workspace.

1. Copy the resource-only manifest example from `extensions/develop` into a new source directory.
2. Name it `studio-reference-kit` and add `agents/reference_editor/AGENT.md` with valid frontmatter.
3. Run `compozy extension dev <source> --workspace <studio-workspace> --profile studio -o json`.

**Expected:** The documented native manifest builds and projects its agent into the selected workspace.
**Actual:** The CLI exits 1: `resources.agents` expects a table but receives a string.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/passive-kit-dev.json` records the refusal.
- `passive-authoring-paths.json` records the authored sources without a language project.
- `passive-batuta-build-{1,2}.json` independently shows the current table-shaped manifest builds twice
  with an identical generation hash.

## Fix

- **Root cause:** Profile placement changed static resource declarations from strings to typed
  path/profile objects without a loader translation for the shipped public string format. The
  public authoring and manifest guides still teach the string form.
- **Fix commit:** Pending.
- **Regression test:** Existing `internal/extension/manifest_test.go` owns TOML/JSON manifest decoding,
  normalized path/profile identity, strict resource fields, and canonical re-encoding.
- **Compatibility:** Translate the previous input shape only at the decoder boundary, keep canonical
  output as objects, and document migration before removal in v0.3.0-beta.31 (SD-013 regime 2).

## Verification

Both format regressions fail before the change and pass afterward. The focused manifest/build race
suite passes. A rebuilt binary completes dev, edit/reload, watch publication, invalid-edit last-good
retention, explicit profile placement, and removal through CLI with independent HTTP/UDS reads.
The current-format Batuta hash and copied resource bytes remain unchanged; a real TypeScript
extension also builds, links, invokes, and removes successfully. Commit and delivery gate pending.
See the completed Bruno debrief and `passive-*` evidence in the report.
