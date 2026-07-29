---
id: ET-site-docs-typography-opendesign
area: ET
title: Docs shell typography matches OpenDesign three-voice ramp
persona: Dora
journey: J-evaluate-compozy-beta
expected: On a runtime docs page (e.g. /docs/loops), Playfair Display appears only on the page title and h2 (retuned heading clamp, hairline top); h3 is Inter 600 at 1.25rem/1.25; lead is 18px muted at 64ch; body is Inter 16px/1.8 muted at 72ch with strong at 600 and underlined links; sidebar section labels use mono badge (10.5px accent-mix) and in-folder groups use mono group-label (10px subtle 72%) — no Inter uppercase, no weight 700, no Playfair below display size.
entry_points: compozy.com /docs/loops; docs/design/opendesign/docs/docs-design-system.html §02; docs/design/opendesign/docs/docs-page.html
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-docs-masthead-opendesign; ET-site-docs-sidebar-opendesign
---

QA impact 2026-07-29: docs shell type ramp was aligned to OpenDesign §02
(three voices, retuned h2 clamp, h3 weight, lead/group-label tokens, prose
link underline). The next QA cycle owns visual parity against the §02
specimen on desktop mid-width and mobile body retune.
