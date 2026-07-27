---
id: RT-compozy-home-layout
area: RT
title: Create the Compozy home layout on fresh boot
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: A fresh daemon boot creates the canonical .compozy home and workspace overlay, status and doctor report only those paths, and no legacy .agh directory or fallback is created or read.
entry_points: compozy daemon start; compozy status -o json; compozy doctor -o json; fresh isolated home and workspace
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-refuse-legacy-database
---

QA impact 2026-07-26: the runtime home and workspace overlay moved from `.agh/` to
`.compozy/` as a zero-legacy hard cut. Planning flag only; the next QA cycle owns a
fresh-boot and structured-diagnostics pass.
