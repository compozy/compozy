---
id: ET-site-docs-masthead-opendesign
area: ET
title: Docs page masthead matches OpenDesign context-row anatomy
persona: Dora
journey: J-evaluate-compozy-beta
expected: On a runtime docs page (e.g. /docs/loops), the article masthead shows a context row with mono badge crumbs (accent product · clickable parents · fg leaf) and 28px page actions on the right above the title; Playfair title without a 12ch cap; 18px muted lead; meta strip with audience and section page count only — no Audience/Focus definition list and no Fumadocs chrome breadcrumb above the masthead.
entry_points: compozy.com /docs/loops; docs/design/opendesign/site/site-docs-sidebar.html; docs/design/opendesign/site/site-example-page.html
qa_status: pass
bug_ids: BUG-20260730-docs-index-invalid-hydration
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc01-docs-landing; /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc03-example-page
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-docs-sidebar-opendesign; ET-site-docs-first-session; ET-site-docs-typography-opendesign
---

QA impact 2026-07-29: docs page masthead was rebuilt to the OpenDesign context-row
contract (context-row crumbs + actions, truthful meta strip). Later the same
day, masthead lead was bound to `--text-site-doc-lead` as part of the §02
typography align. The next QA cycle owns visual parity against site-example-page.html
on desktop mid-width and mobile wrap of the action cluster.
