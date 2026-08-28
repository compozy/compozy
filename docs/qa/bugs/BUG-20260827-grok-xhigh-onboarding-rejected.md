# BUG-20260827-grok-xhigh-onboarding-rejected: A valid Grok 4.6 choice blocks onboarding

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-19 Choose a default runtime during onboarding, step 4
- **Scenarios:** RT-071; ET-web-runtime-selector-minimal-slider; RT-cursor-logical-runtime-options
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

A first-time user can choose Cursor Grok 4.6 with Extra high reasoning from the live catalog, but Continue rejects the advertised combination and leaves the user stuck on the first onboarding step.

## Reproduction

- **Charter:** CH-030 · **Tour:** Feature Tour
- **Environment:** laptop / 1280px desktop viewport / wifi-fast / en-US; isolated local daemon with the operator's native Cursor login

1. Open the first-run onboarding wizard.
2. Open Runtime, select Cursor Agent, then Cursor Grok 4.6.
3. Move the Reasoning slider to Extra high.
4. Choose the provider CLI authentication path and press Continue.

**Expected:** The selection is accepted because the live catalog advertises `grok-4.6` with `xhigh`, and onboarding advances.
**Actual:** The page reports `reasoning effort "xhigh" is unavailable for provider model "cursor"/"grok-4.6"` and remains on the Runtime step.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/onboarding-grok-4-6-xhigh-rejected.png`
- The independent CLI, HTTP, and native-tool catalog reads returned byte-identical public projections in which `grok-4.6` includes `xhigh` with Fast both off and on: `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-models-cli.json`.

## Fix

- **Root cause:** settings curation validated only public `reasoning_efforts`. Cursor truthfully stores launch-bound combinations in private transport bindings, so the catalog and selector accepted `xhigh` while the settings write rejected it.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `TestProviderSettingsUsesMergedCatalogProjection/Should accept a default effort advertised by a launch binding`

## Verification

- **Retested:** 2026-08-27 in a fresh browser session against the rebuilt isolated daemon
- **Result:** pass — onboarding accepted Cursor Grok 4.6 with Extra high reasoning, advanced to Workspace, and persisted the logical `grok-4.6` plus `xhigh` without exposing a transport alias.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/onboarding-grok-4-6-xhigh-accepted.png`; `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-settings-after-onboarding.json`
