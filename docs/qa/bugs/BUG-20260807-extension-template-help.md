# BUG-20260807-extension-template-help: Connectivity provider templates are absent from CLI help

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-extension-kit-lifecycle, template selection
- **Scenarios:** ET-extension-code-first-authoring; ET-extension-manifest-v2-surfaces
- **Found:** 2026-08-07 · **Report:** `docs/qa/reports/2026-08-07-remote-gateway.md`

## Summary

The embedded scaffold catalog accepts `connectivity-provider-go` and `connectivity-provider-ts`, and the official skill teaches both, but `compozy extension init --help` lists only the five older templates.

## Reproduction

- **Charter:** CH-gateway-provider-degradation · **Tour:** Feature Tour
- **Environment:** CLI / isolated lab / en-US

1. Run `compozy extension init --help`.
2. Compare the `--template` description with the embedded scaffold catalog.

**Expected:** Every accepted embedded template is discoverable from the command help.
**Actual:** Both connectivity-provider templates are omitted.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/41-extension-template-discovery.json`

## Fix

- **Root cause:** The flag description duplicated the template catalog as a hard-coded sentence and was not updated when the new templates shipped.
- **Fix commit:** Task 09 QA remediation batch (pending local commit)
- **Regression test:** `TestExtensionInitHelpListsEveryScaffoldTemplate`

## Verification

- **Retested:** 2026-08-07
- **Result:** Pass — live CLI help listed every embedded template and both connectivity templates scaffolded successfully. External builds remain covered by the pre-existing unpublished-SDK blocker.
