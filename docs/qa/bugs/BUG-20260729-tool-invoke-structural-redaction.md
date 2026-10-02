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

## Regressed (2026-10-02)

- **Persona:** Ada; CH-cli-tool-structural-handles, Feature Tour.
- **Report:** `docs/qa/reports/2026-10-02-untested.md`.
- On the fresh `f0c134ad7` build, generic CLI invocation of `compozy__tool_info` for
  `compozy__profile_list` replaces the public `credential_requirements` output-schema types and
  required-property names with `[REDACTED]`. This is schema metadata, not credential values.
- Direct `compozy tool info`, HTTP invocation, and UDS invocation preserve the same descriptor.
  The divergence localizes the failure to the generic CLI output boundary; root-cause code
  investigation and repair remain pending.
- **Evidence:** `docs/qa/evidence/2026-10-02-untested/profile-list-tool-info.json`,
  `profile-list-descriptor-info.json`, `profile-tool-info-http.json`, and
  `profile-tool-info-uds.json` in the same directory.
- **Reproduction:** `compozy tool invoke compozy__tool_info --workspace <workspace>
  --input '{"tool_id":"compozy__profile_list"}' -o json`, then compare the nested
  `result.structured.tool.descriptor.output_schema` with `compozy tool info compozy__profile_list`.

### Current repair and focused replay

- **Root cause:** The daemon's tool-result sanitizer classifies `credential_requirements` as
  public metadata, but the shared redactor used by the CLI classified the entire object as
  sensitive. It recursively replaced all string values, including schema types and field names.
- **Correction:** Align the shared sensitive-key classification with the existing public field
  contract. Nested sensitive keys still traverse normal redaction; no output path disables it.
- **Owning invariant and suite:** Public credential requirement metadata and discovery schemas
  survive sanitization while nested secret values remain hidden. Both cases live in
  `TestEngineRedactJSONPreservesStructuredEnvelope` in `internal/redact/redact_test.go`.
- **Red/green:** Both added cases reproduced the failure before the change. The complete
  race-enabled redactor suite passes after it, with 80.1% statement coverage.
- **Live replay:** The rebuilt CLI's complete structured `compozy__tool_info` result equals the
  HTTP and UDS results. The schema contains zero redaction markers, and its discovered
  `compozy__profile_list` tool executes successfully. Evidence:
  `docs/qa/evidence/2026-10-02-untested/redaction-fixed-*.json` and `redact-race.log`.
- **Still open for delivery:** Commit/gate evidence and the broader structural-handle charter
  (cursor reuse, workspace isolation, and live secret containment) remain in progress.

### Additional discovery-schema divergence (2026-10-02)

- The continuation reproduced a second metadata corruption: generic CLI discovery of
  `compozy__extensions_install` changes the JSON Schema pattern `^vault:extensions/.+$` into
  `^[REDACTED]+$`. Direct CLI discovery, HTTP invocation, and UDS invocation retain the pattern.
  The unchanged `input_schema_digest` therefore describes a different schema from the CLI output.
- **Root cause:** The shared JSON redactor recognizes field names but its scalar-protection
  callback has no path context. The exact secret-reference matcher treats schema validation
  syntax as a live Vault reference. The earlier credential-key repair cannot address this case.
- **Evidence:** `handles-install-descriptor-{direct,generic,http,uds}.json` in this cycle's
  evidence directory. The persona session ended before implementation work resumed.
- **Repair boundary:** Preserve validation patterns only at JSON Schema keyword positions in
  tool-discovery descriptors. Free text, defaults, unrelated `pattern` fields, and sensitive
  values must retain normal redaction. The owning suite is
  `TestToolRenderingAndValidationHelpers` in `internal/cli/tool_test.go`.
- **Correction:** Scalar protection receives the traversed object-key path. The CLI recognizes
  schema keyword paths beneath discovery descriptors; it does not exempt whole schemas or
  arbitrary fields called `pattern`. Existing CLI redaction helpers moved into the cohesive
  `client_tools_redaction.go` file to keep the transport source below the project size limit.
- **Proof:** The new owning regression failed with `^[REDACTED]+$` before repair. Complete
  race-enabled CLI and redactor suites pass (11.353s and 2.415s). Fresh rebuilt CLI `tool_info`
  and `tool_search` calls now equal complete HTTP and UDS structured results, including the exact
  pattern and unchanged digest. See `schema-pattern-red.log`, `schema-pattern-race.log`, and
  `schema-pattern-fixed-*` in this cycle's evidence directory.

## Completed current verification (2026-10-02)

- **Fix commit:** `196cd3010`; all affected local gate lanes passed.
- Fresh generic discovery matches complete HTTP/UDS structured output. Marketplace cursors
  open the next page and refuse cross-workspace reuse. A real generated TypeScript extension
  produces the same immutable generation hash across CLI/HTTP/UDS; that exact returned hash
  activates the dev extension and the returned tool ID invokes it successfully.
- The running extension returns a sensitive-key value and secret-shaped free text using synthetic
  examples only. Generic CLI hides both while retaining the public label and usable tool identity;
  ordinary structured results, including sensitive-key redaction, match HTTP and UDS.
- The foreign workspace cannot invoke the extension. Removal leaves no registration or tool in
  either workspace, HTTP/UDS return 404, and the owned subprocess exits. Authored source remains
  in the lab project, as the dev-removal contract promises.
- **Evidence:** `handles-build-parity-*`, `handles-reuse-generation-hash.json`,
  `handles-reuse-tool-id-*`, `handles-owned-tool-*`, `handles-foreign-tool-refusal.json`, and
  `handles-clean-*` in this cycle's evidence directory.
- **External limit:** The current Marketplace Airtable entry requires Compozy 0.5.0 and refused
  this candidate. Compatible local extension generation supplies the successful digest-reuse leg;
  no version or redaction bypass was used.
