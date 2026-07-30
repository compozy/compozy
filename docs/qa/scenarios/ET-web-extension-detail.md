---
id: ET-web-extension-detail
area: ET
title: Inspect an installed extension
persona: Bruno
journey: J-marketplace-acquisition
expected: The extension detail route survives refresh and renders runtime state, required and missing environment variables, diagnostics and last_error severity, provenance and trust, and links active provided bundles to their activation details.
entry_points: /marketplace/extension/$entryId?installed_name=$name; Marketplace Extensions Installed row
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-management-lifecycle.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-extension-lifecycle-final.png;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-015; ET-022; ET-023
---

Planning note: exact catalog-entry projection and cross-owner cache reconciliation remain pending; no bug fix is associated with this scenario.

Added by marketplace Task 07. Compare healthy and degraded states with the extension-detail OpenDesign contract.

QA impact 2026-07-16: extension detail now consumes the installed API's exact Marketplace projection;
reset to prove catalog-backed description and Update remain visible outside the first browse page.

QA impact 2026-07-16: updating from the detail route now reconciles Marketplace discovery together
with installed inventory; assert the Marketplace badge and action after returning to browse.

QA impact 2026-07-16: the Bundles provided rail and Remove dialog now distinguish loading, failed,
active, and confirmed-inactive dependency states; retest the error message and Retry path.

QA impact 2026-07-18: the Marketplace hard-cut detail preserves runtime state, health message,
daemon/PID/uptime, capabilities, actions, environment, diagnostics, provenance, and active-bundle
links. Exercise both healthy/running and degraded/stopped payloads.

QA impact 2026-07-29: extension detail now renders the daemon's update affordance (with explicit
consent for unverified archives), the workspace dev overlay badges (`dev`, `overrides published`,
origin path, generation), crash-loop counters and restart backoff, and separates archive integrity
(`digest_matched`) from curated checksum pinning. Enable/disable and Update are withheld while a dev
overlay is selected. Flag only — retest in the next QA cycle.

QA impact 2026-07-29 deep-review remediation: a workspace dev overlay can be unlinked even when a
published extension with the same name is referenced by an active bundle. The published removal remains
blocked. Retest both branches from the installed-extension dialog.
