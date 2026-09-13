---
id: ET-web-marketplace-sources-add
area: ET
title: Add a team marketplace from Browse
persona: Marina
journey: J-marketplace-acquisition
expected: "Preview validates without registration; Add creates a source section and its plugin installs with displayed digest and one unverified confirmation."
entry_points: "/marketplace; Add > Marketplace"
qa_status: pass
bug_ids: [BUG-20260913-marketplace-name-conflict-trail, BUG-20260913-marketplace-collision-suggestion, BUG-20260913-marketplace-config-replay, BUG-20260913-marketplace-version-digest-conflict]
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/source-readded.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps:
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Try both document locations, invalid refs, checked paths, suggested names, retained/reserved names, cancellation and a client-layout plugin. Refresh the Installed view after acquisition.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
