---
id: ET-web-extensions-manage
area: ET
title: Manage installed extensions and their kits
persona: Bruno
journey: J-marketplace-acquisition
expected: The Extensions Installed scope lists daemon-owned inventory, applies enable changes immediately, derives truthful update state, previews kit changes, and permits typed removal with owned-resource cleanup.
entry_points: /marketplace/extensions?tab=installed; Marketplace Manage actions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-management-lifecycle.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-extension-web-management-final.png;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
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
