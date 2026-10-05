---
id: LP-web-runs-breadcrumb
area: LP
title: Loop run views keep a labeled trail back to the catalog
persona: Marina
journey: J-03
expected: The Loops window head on `/loop-runs` is a drill-in trail `Loops › Runs` (back and the Loops crumb open `/loops`). Opening a run keeps `Runs` in the trail (`Loops › Runs › {loop} › Run from {age}`); Compare inserts the run id as a parent and leaves `Compare` as the leaf. A deep link to `/loop-runs` still shows the trail. The catalog itself stays inventory chrome with no breadcrumb.
entry_points: web /loop-runs; web /loop-runs/$runId; web /loop-runs/$runId/diff; web /loops
qa_status: fail
bug_ids: BUG-20261005-loop-breadcrumb-history-overrides-parent
fix_status: pending
retest_status: pending
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: LP-008; ET-web-route-chrome-topbar
---

story: As an operator I can always see where I am in Loops and click back to the catalog or the workspace-wide Runs list without relying on the dock.

src: web/src/systems/os/apps/loops/loop-window-crumbs.ts; web/src/systems/os/apps/loops/loop-runs-location.tsx; web/src/systems/os/apps/loops/loop-run-detail-location.tsx; web/src/systems/os/apps/loops/loop-run-diff-location.tsx

2026-10-05 expectation reconciliation: the detail leaf's human label follows the approved
plain-language UI change ecb3abc4e257ab10275e4006d271d000f594f98d (#683). Compare keeps its
explicit run-id parent. Parent destination and deep-link recovery requirements are unchanged.

2026-10-05 partial Marina walk: deep-link trail, hidden Runs parent, Compare generation
selection and explicit run-id parent navigation pass on the 430x932 touch persona.
Back from Compare lands on Runs; Back and Loops from Runs do not navigate under touch.
Navigation ownership/target diagnosis is pending, so no pass or fixed verdict is assigned.
The nine-frame loops-trail-marina recording is closed; receipts and screenshots are in
docs/qa/evidence/2026-10-02-untested/loops-trail-marina-*.

2026-10-05 functional replay: Marina's fresh touch replay confirms Compare/run/hidden Runs/catalog parents and direct-link Back navigation, followed by reload and independent window-manager revision 398. Evidence: loops-trail-marina-verified-parent-chain.json, loops-trail-marina-verified-deep-links.json and loops-trail-final-window-uds.json. Delivery gate and fix commit are pending.
