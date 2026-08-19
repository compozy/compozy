# BUG-20260729-tool-invoke-structural-redaction: CLI redaction erased public structural handles

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-marketplace-parity, reuse native-tool output through the generic CLI
- **Scenarios:** ET-cli-tool-invoke-structural-handles; LP-select-typed-loop-entities
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** Fresh isolated native-tool CLI/HTTP/UDS parity replay

## Summary

`compozy tool invoke ... -o json` replaced valid daemon-authored bundle IDs and continuation cursors
with `[REDACTED]`. Direct HTTP and UDS tool calls preserved those public handles, so an agent using
the CLI could not feed the result into the next supported operation.

## Reproduction

1. Activate the same managed bundle in two workspaces.
2. Invoke `compozy__marketplace_search` through generic CLI tool invocation and through direct HTTP
   and UDS tool endpoints.
3. Compare the workspace-scoped bundle activation IDs and a paginated `next_cursor`.

**Expected:** Public IDs, digests, and cursors survive defensive display sanitization; sensitive
keys and secret-shaped free text remain redacted.
**Actual before the fix:** The CLI entropy heuristic replaced public structural handles with
`[REDACTED]` while direct daemon surfaces returned them intact.

## Evidence

- Two-workspace bundle projections, cursor parity, secret scanning, and cleanup assertions:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace`.

## Fix

- **Root cause:** The generic CLI recursively applied scalar entropy redaction to every structured
  JSON string, bypassing the canonical field-aware redactor.
- **Correction:** Valid structured tool results now use field-aware JSON redaction. Cursor fields are
  protected public envelope handles; invalid raw fallback text retains diagnostic redaction.
- **Fix commit:** `f3b8837`
- **Regression tests:** `Should redact invoke metadata fields` in
  `internal/cli/client_tools_test.go` and the structured-envelope case in
  `internal/redact/json_test.go`.

## Verification

- Complete `internal/cli` and `internal/redact` race suites pass.
- Rebuilt generic CLI tool invocation matches the caller workspace's HTTP payload, preserves stable
  bundle IDs and `next_cursor`, and excludes the other workspace's activation.
- The fixture activation, extension, and policy override were removed after the replay.
- **Retested:** rebuilt candidate green; governed fix commit `f3b8837`

## Re-found (2026-08-18)

- **Persona:** Lea · **Report:** `docs/qa/reports/2026-08-18-typed-loop-inputs.md`
- `compozy__vault_list` replaced the metadata-only `ref` with `[REDACTED]`, so an agent could not
  reuse the discovered Vault reference in `compozy__loop_run`.
- `compozy__loop_list` replaced the complete declaration of an input named `token`, hiding its
  `type`, `required`, and `ref.kind` contract even though no secret value was present.
- The first symptom remained in the CLI's second defensive sanitizer; the second came from the
  daemon result limiter treating a user-authored schema key as secret-bearing data.
- The rebuilt CLI now preserves the exact metadata-only Vault ref and the complete public `token`
  declaration while still redacting the secret input value in `compozy__loop_run` output.
- **Evidence:** `docs/qa/reports/2026-08-18-typed-loop-inputs.md` and the isolated lab journey log at
  `/Users/pedronauck/dev/qa-labs/compozy-typed-loop-inputs-20260819-015537-040869-lab/qa-artifacts/qa/journey-log.jsonl`.
