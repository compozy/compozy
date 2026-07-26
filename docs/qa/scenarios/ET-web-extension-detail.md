---
id: ET-web-extension-detail
area: ET
title: Inspect an installed extension
persona: Bruno
journey: J-marketplace-acquisition
expected: The extension detail route survives refresh and renders runtime state, required and missing environment variables, diagnostics and last_error severity, provenance and trust, and links active provided bundles to their activation details.
entry_points: /marketplace/extension/$entryId?installed_name=$name; Marketplace Extensions Installed row
qa_status: untested
bug_ids:
fix_status:
retest_status: pending exact catalog-entry projection and cross-owner cache reconciliation
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-management-lifecycle.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-extension-lifecycle-final.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-015; ET-022; ET-023
---

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
