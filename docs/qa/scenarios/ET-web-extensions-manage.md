---
id: ET-web-extensions-manage
area: ET
title: Manage installed extensions and their kits
persona: Bruno
journey: J-marketplace-acquisition
expected: The Extensions Installed scope lists profile-effective inventory, applies per-profile enablement immediately, shows declared profiles, needs-setup and dormant placements, reviews install or update changes before mutation, and permits typed removal with owned-resource cleanup.
entry_points: /marketplace/extensions; Marketplace Manage actions; /marketplace/extension/{entry_id} install preview; POST /api/extensions/preview-install; compozy extension install|update|enable|disable|remove; compozy --profile <name> extension enable|disable; GET /api/extensions/{name}/inventory?profile=<name>; GET|PUT /api/extensions/{name}/enablement over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/marketplace-extension-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/spec-cycle-trusted-detail.png
last_report: docs/qa/reports/2026-08-07-critical-runtime-ui-fixes.md
overlaps: ET-019; ET-020; ET-021; ET-ext-inventory; ET-ext-preview
---

Planning note: exact catalog-entry enrichment and cross-owner cache reconciliation remain pending; no bug fix is associated with this scenario.

Added by marketplace Task 07. Exercise the complete toggle, update, typed-remove conflict, deactivate, and successful removal sequence against one real daemon.

QA impact 2026-07-16: installed extension description and Update truth now come from the API's exact
`provenance.catalog_entry_id` join rather than a capped Marketplace browse page; reset for the next QA cycle.

QA impact 2026-07-16: update and removal now invalidate both installed inventory and Marketplace
discovery so action labels and update badges reconcile together.

QA impact 2026-07-29: the Extensions landing count now unions the installed and catalog update
projections (non-zero on the market scope, never claiming exactness from a partial catalog), the
installed cards carry source/registry-tier/integrity badges, the empty-state CLI hint names real
verbs, and an Extensions-only "Install extension" toolbar entry opens the source-union form.
Flag only — retest in the next QA cycle.

QA impact 2026-08-02: extension lifecycle is the only kit-management surface. Reset to cover
inventory, preview, enable, update, disable, and removal without a separate activation scope.

QA impact 2026-08-22: reset for declared-profile summaries, per-profile toggles, needs-setup,
dormant placements, and install/update preview after the global enable-preview surface was removed.

Walk the declared-profile summary and install/update preview first. In the installed scope, select
each profile and verify effective inventory, enablement, needs-setup, and dormant placement detail;
then disable and re-enable one profile without changing another. Confirm the preview request and
response before install or update, and finish with typed removal and extension-owned cleanup.

Expected evidence: Marketplace and detail screenshots, profile-qualified inventory and enablement
payloads, preview request/response pairs, needs-setup and dormant-placement captures, and removal
cleanup output.

2026-08-23 qa-impact (Profiles): the web enablement control now acts on the active profile rather
than on the machine, backed by per-profile exception rows (absent row means enabled). Already
`untested`, so no reset was needed. Confirm the management surface names the profile it is acting
on, that toggling in one profile leaves the others untouched, and that the state it shows matches
the CLI and API for the same profile. The cross-surface contract is owned by
`ET-extension-profile-enablement`.
