# BUG-20260729-resource-docs-protected-kind: Resource guide teaches a forbidden bundle mutation

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-agent-marketplace-parity, follow the desired-state resource write guide
- **Scenarios:** SITE-resource-mutation-boundary
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Origin:** n/a

## Summary

The desired-state resource guide used generic `PUT` and `DELETE` examples for
`bundle.activation`. The runtime correctly rejects those requests because bundle activation owns
requirement confirmation, inventory projection, and deactivation cleanup.

## Reproduction

- **Charter:** CH-untested-046-agent-marketplace-parity-ada · **Tour:** Feature Tour
- **Environment:** isolated macOS lab, desktop / wifi-fast / en-US

1. Follow the guide's `PUT /api/resources/bundle.activation/:id` example through the operator UDS.
2. Observe the response and compare it with the bundle lifecycle documentation.

**Expected:** The resource guide uses a directly mutable registered kind and sends bundle
activations to `/api/bundles/activations` or `compozy bundle`.
**Actual:** The documented request returns `403 resources: direct mutation not allowed`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/006-resource-tool-hook`
- `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/008-resource-crud-retry`

## Fix

- **Root cause:** The guide predated the hard service boundary that moved bundle activation
  mutations behind the bundle lifecycle API, while the generic resource examples remained unchanged.
- **Fix commit:** pending completion gate
- **Regression test:** No prose-string test. The runtime's canonical HTTP/UDS resource suites own
  the 403 boundary; site source generation and the site Turbo lane own the corrected MDX artifact.

## Verification

- **Retested:** pending fix commit
- **Result:** The corrected guide uses `automation.job` for generic optimistic CRUD and explicitly
  routes `bundle.activation` mutations through the bundle service.
