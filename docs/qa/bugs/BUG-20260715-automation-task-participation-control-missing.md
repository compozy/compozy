# BUG-20260715-automation-task-participation-control-missing: Automation Task target cannot choose Network participation

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Nia
- **Journey Step:** J-network-local-default, configure task-backed automation
- **Scenarios:** NB-execution-participation-defaults; NB-participation-controls-serialize
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Automation Task drafts carried a typed Network participation field, but the Job editor exposed no control for it. Operators could neither see the Local default nor explicitly prepare a Live task-backed fire from Web.

## Reproduction

- **Charter:** CH-network-local-default · **Tour:** Feature Tour
- **Environment:** 375/768/1280 px / isolated daemon-served Web / en-US

1. Open Create job.
2. Select `Run task`.

**Expected:** A Local/Live participation control appears before owner selection and serializes only `network_participation`.
**Actual:** The Task target moved directly from description to owner.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md`
- Lab screenshots: `qa/screenshots/automation-task-participation-{local,live}-{375,768,1280}.png`.

## Fix

- **Root cause:** the form hook already stored the draft field, but `AutomationJobForm` never mounted the shared participation control or update handler.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the existing Automation Job form suite changes Local to named Live and requires the emitted draft/payload to contain only the typed field.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** Local is visible by default, Live reveals channel/strategy, returning to Local removes Live-only fields, and all checked widths remain usable without submitting the QA draft.
