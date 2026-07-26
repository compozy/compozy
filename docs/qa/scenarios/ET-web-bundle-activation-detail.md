---
id: ET-web-bundle-activation-detail
area: ET
title: Inspect and reconcile a bundle activation
persona: Bruno
journey: J-marketplace-acquisition
expected: The activation detail route survives refresh, renders scope, workspace, profile, resources, inventory, channel binding and timestamps, offers Update only for spec_drift, clears drift after reapply, and deactivates through a confirmation dialog.
entry_points: /marketplace/bundles/activations/$id; /marketplace/bundles?tab=installed
qa_status: untested
bug_ids:
fix_status:
retest_status: pending after PageHead/topbar-actions chrome move
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-management-lifecycle.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/marketplace-bundle-lifecycle-final.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-027; ET-028
---

Added by marketplace Task 07. Compare current and drifted activation states with the bundle-activation-detail OpenDesign contract.

QA impact 2026-07-18: activation detail moved under Marketplace and every installed bundle card or
via-bundle action now links to the new activation route.
