# BUG-20260727-runtime-legacy-identity: Candidate runtime exposes retired product identities

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada; Dora
- **Journey Step:** J-validate-compozy-hard-cut, runtime and public-contract identity
- **Scenarios:** RT-compozy-environment-namespace; ET-compozy-native-tool-invocation; ET-compozy-extension-contract-identity; ET-compozy-public-brand-navigation
  ET-compozy-extension-contract-identity; NB-compozy-wire-identity;
  ET-compozy-public-brand-navigation; ET-spec-cycle-skill-bundle
- **Found:** 2026-07-27 · **Report:** docs/qa/reports/2026-07-27-devtool-oss-launch.md
- **Origin:** n/a

## Summary

The candidate binary uses the Compozy executable and storage identity, but its live status output,
managed-agent prompts, public headers, event payloads, artifact descriptors, bridge contracts, and
Web state still expose retired product names. A fresh operator or agent therefore sees two product
identities in one runtime and cannot rely on the migration's hard-cut contract.

## Reproduction

- **Charter:** CH-compozy-platform-hard-cut; CH-compozy-wire-public-hard-cut · **Tour:** Garbage Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US

1. Build and start the current candidate in a fresh isolated `COMPOZY_HOME`.
2. Run `compozy status -o json` and inspect provider diagnostics.
3. Inspect managed-session startup prompts, native-tool descriptors, OpenAPI headers, SSE data
   parts, tool-artifact metadata, bridge manifests, and Web persistence keys.

**Expected:** Every user-facing and agent-facing runtime surface uses one Compozy identity; retired
Retired wire, artifact, bridge, prompt, and Web-state names are absent with no aliases or fallback
readers.

**Actual:** The live status response used the retired product name in a provider credential diagnostic, and the
same retired identity remains active across the inspected public surfaces.

## Evidence

- Candidate status output from the isolated lab exposed the retired provider message before the
  first charter kickoff.
- Source audit found active retired identities in runtime prompts and diagnostics, native-tool
  descriptors, extension and bridge contracts, OpenAPI headers, SSE parts, artifact URI/MIME
  values, and Web local-storage keys.
- Post-fix contract evidence is green before live retest: regenerated OpenAPI/site consumers,
  six scoped Go race lanes (9,590 tests), Web/UI/site Turbo lanes, 46 lint-plugin tests, package
  boundaries, and an inspected 1440x900 Compozy-menu capture.
- Lab manifest:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-120735-072314-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Fix

- **Root cause:** Earlier migration batches cut executable, environment, package, and selected
  network identities by task boundary, but never closed the remaining runtime/public-contract
  inventory. The final candidate therefore composed new Compozy surfaces with active retired wire
  and presentation contracts.
- **Fix commit:** `e4df8634`
- **Regression test:** existing canonical contract, prompt, provider, bridge, transcript, artifact,
  and Web suites must assert the Compozy-only values; the isolated charter rerun must prove the live
  candidate emits no retired identity.

## Verification

- **Retested:** 2026-07-27 in the original isolated lab before channel or kickoff creation.
- **Result:** Pass. Fresh `status-live.json` and `doctor-live.json` expose Compozy-only provider
  diagnostics; an exact case-insensitive scan of live version/status/doctor evidence found no retired
  header, prefix, diagnostic, or wire identity.
