# BUG-20261122-install-activation-failure-masked-500: An install whose subprocess fails to start returns a masked 500

- **Status:** open
- **Impact (user-side):** Friction. The Web shows "Internal Server Error · Installed first:
  acme-depa", with no hint that the dependency's process failed to start
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-extension-distribution, source-union install with dependencies
- **Scenarios:** ET-web-extension-union-install (D16 partial failure step)
- **Found:** 2026-11-22 · **Report:** docs/qa/reports/2026-11-22-gh-1845-pr-b-extension-dependencies-rewalk.md

## Summary

`acme-three` requires `acme-depa` (healthy) and then `acme-depx`, whose subprocess exits at once.
The Web joint install commits `acme-depa`. Then `acme-depx` fails to initialize, and the daemon log
records `extension.lifecycle.failed … initialize subprocess: subprocess: write frame: write |1:
broken pipe`, and `acme-depx` is rolled back. The response is
`500 {"error":"Internal Server Error","code":"extension_install_failed","dependency":"acme-depx","dependencies_installed":["acme-depa"]}`.
The Web correctly appends "Installed first: acme-depa", but the message the user reads is "Internal
Server Error". A plain install of `acme-depx` (no dependencies, with or without `enable`) returns a
bare `500 {"error":"Internal Server Error"}` as well.

## Reproduction

- **Environment:** isolated lab, binary and `vite` Web from `4ce0671ff` stamped `v0.7.9`.

1. Create local `acme-depx` whose `bin/acme` is `#!/bin/sh` + `exit 1`, and local `acme-three`
   requiring `acme-depa` and then `acme-depx` (fixtures in
   `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/fixture/`).
2. Web: install `<lab>/fixtures/acme-three` from a local build with Allow an unverified package. Confirm
   both dependency gates and the root consent.
3. `POST /api/extensions {"source":"local_path","ref":"<lab>/fixtures/acme-depx","allow_unverified":true}`.

**Expected:** a 4xx or 5xx whose message names the activation failure (for example
"acme-depx failed to start: initialize subprocess …"), so the Web shows "<cause> · Installed first:
acme-depa".
**Actual:** "Internal Server Error · Installed first: acme-depa" in the Web, and a bare masked 500
from the plain install.

## Evidence

- Screenshot: `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/screenshots/12-partial-failure-installed-first.png`
  (`https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/pr-1961/4ce0671/12-partial-failure-installed-first.png`).
- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/cli/daemon.log`, 19:05:32–19:05:35: the
  `acme-depa … outcome: installed` line, `extension.lifecycle.failed` for `acme-depx`, the
  `acme-depx … outcome: failed` line and `httpapi: request … /api/extensions status:500`.
- `docs/qa/evidence/2026-11-22-gh-1845-pr-b-d12/cli/transcript.md`: "plain install of acme-depx …".

## Pre-existing

Likely yes for the masking. The plain, no-dependency install of the same package returns the same
bare 500. The dependency path (PR B) adds `code`, `dependency` and `dependencies_installed`, but
keeps the masked message.
