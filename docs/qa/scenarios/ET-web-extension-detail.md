---
id: ET-web-extension-detail
area: ET
title: Inspect an installed extension
persona: Bruno
journey: J-marketplace-acquisition
expected: The extension detail route survives refresh and renders runtime state, required and missing environment variables, bound key names, kit inventory, diagnostics and last_error severity, provenance, and trust.
entry_points: /marketplace/extension/$entryId?installed_name=$name; Marketplace Extensions Installed row
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/extension-spec-cycle-trust.png; docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/extension-update-precondition.md
last_report: docs/qa/reports/2026-08-10-loop-browser-runtime-closeout.md
overlaps: ET-015; ET-022; ET-023; ET-web-extension-kit-inventory
---

Planning note: exact catalog-entry projection and cross-owner cache reconciliation remain pending; no bug fix is associated with this scenario.

Added by marketplace Task 07. Compare healthy and degraded states with the extension-detail OpenDesign contract.

QA impact 2026-07-16: extension detail now consumes the installed API's exact Marketplace projection;
reset to prove catalog-backed description and Update remain visible outside the first browse page.

QA impact 2026-07-16: updating from the detail route now reconciles Marketplace discovery together
with installed inventory; assert the Marketplace badge and action after returning to browse.

QA impact 2026-07-18: the Marketplace hard-cut detail preserves runtime state, health message,
daemon/PID/uptime, capabilities, actions, environment, diagnostics, and provenance. Exercise both
healthy/running and degraded/stopped payloads.

QA impact 2026-07-29: extension detail now renders the daemon's update affordance (with explicit
consent for unverified archives), the workspace dev overlay badges (`dev`, `overrides published`,
origin path, generation), crash-loop counters and restart backoff, and separates archive integrity
(`digest_matched`) from curated checksum pinning. Enable/disable and Update are withheld while a dev
overlay is selected. Flag only — retest in the next QA cycle.

QA impact 2026-08-02: the detail adds truthful kit inventory, bound environment key names, and the
shared Network confirmation affordance. Reset for the next QA cycle.

QA impact 2026-08-10: Extension Update now opens update-specific trust copy and action semantics
instead of reusing Install language. Reset for a fresh update-confirm, cancel, refresh, and detail
walk from the current build.

QA completion: blocked-verify 2026-08-10 — Bruno inspected `spec-cycle` runtime health and official
verified trust after a disable, reload, and restore cycle. The public inventory reported
`update_available: false` for both installed official extensions, so the update-confirm/cancel branch
could not be reached without fabricating daemon state. The full Playwright suite passed the seeded
update-confirm contract; a future catalog with a newer official generation must receive the manual
retest.
