---
id: MS-settings-single-scroll-owner
area: MS
title: Settings stops at the real content boundary
persona: Dora
journey: J-administer-runtime-settings
expected: The Settings window has one vertical scroll owner inside the bounded desktop shell. Scrolling General to the bottom reveals the final setting once, leaves the outer document at scroll position zero, and repeated downward input cannot create empty space below the content.
entry_points: web desktop shell → Settings → General
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/settings-before-scroll.png; /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/settings-at-bottom.png
last_report: docs/qa/reports/2026-08-27-runtime-ui-regressions.md
overlaps: MS-web-settings-takeover-redesign
---

QA impact 2026-08-27: the outer Settings shell and routed page now participate in one bounded flex
layout, leaving `SettingsPageFrame` as the only vertical scroll owner.
