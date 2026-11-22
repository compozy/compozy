# BUG-20261122-preview-malformed-manifest-500: Previewing a local package with a malformed manifest returns a masked 500

- **Status:** open
- **Impact (user-side):** Friction. The Web review and API clients get "Internal Server Error" instead of the TOML error
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-kit-lifecycle, install preview
- **Scenarios:** ET-extension-dependencies (found while preparing a fixture)
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md

## Summary

A local package has an `extension.toml` that does not parse. Here a literal `\n` ended a
`[[requires.extensions]]` line. `POST /api/extensions/preview-install` answers
`500 {"error":"Internal Server Error"}`, and the daemon log has only the
`httpapi: request … status:500` line, so the cause is not visible. The CLI install of the same folder
reports the real cause, but with `-o json` it prints a human line instead of JSON:
`error: extension: decode manifest "…/acme-high/extension.toml": toml: line 14 (last key "requires.extensions"): expected a top-level item to end with a newline, comment, or EOF, but got '\\' instead`,
with exit 1.

## Reproduction

- **Environment:** isolated lab, binary from `0cb8b750d` stamped `v0.7.9`.

1. Write a local package whose `extension.toml` contains `ref = "/x"\nmin_version = "9.0.0"`, with a
   literal backslash-n on the same line.
2. `POST /api/extensions/preview-install {"source":"local_path","ref":"<dir>","allow_unverified":true}`.
3. `compozy extension install <dir> --allow-unverified --yes -o json`.

**Expected:** a 4xx validation response naming the manifest file and the TOML error, and a JSON
error document from the CLI with `-o json`.
**Actual:** a masked 500 from the API with no cause logged, and a human error line from the CLI.

## Evidence

- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-rewalk/cli/transcript.md`: the first three "LOW …"
  entries and the first "LOW preview acme-high" entry, all recorded before the line "(fixture typo
  fixed above; re-running)".
- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-rewalk/cli/daemon-automation-disabled.log`, 14:55:26:
  `httpapi: request … /api/extensions/preview-install status:500`.

## Pre-existing

Yes. `internal/api/core/extensions_install_preview.go` has no diff against `origin/main`
(`9d5e297d7`). The handler maps the preview error through `ExtensionStatusCode`, which yields 500
for a manifest decode error. Not specific to dependencies.

## Verification

Pending fix.
