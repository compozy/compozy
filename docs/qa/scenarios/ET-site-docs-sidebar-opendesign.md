---
id: ET-site-docs-sidebar-opendesign
area: ET
title: Docs sidebar matches OpenDesign section and group anatomy
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs docs, the sidebar shows warm accent-mix section labels for the eight meta.json sidebar groups, 28px single-line rows with elevated hover and active accent rail (long labels truncate with ellipsis and native title tooltip), Lucide icons only on top-level rows, and in-folder separators (e.g. Loops Operate/Author/Reference) as subordinate group labels with hairlines inside a 1px guide-line — never the same chrome as top-level sections.
entry_points: compozy.com /docs; /docs/loops; docs/design/opendesign/docs/docs-design-system.html §03
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-site-docs-first-session; ET-compozy-public-brand-navigation; ET-site-docs-typography-opendesign
---

QA impact 2026-07-29: docs sidebar chrome was refactored to the OpenDesign §03
contract (section vs group labels, guide-line folders, depth-0 icons). Long
nav labels were later constrained to single-line truncate to match the 28px
row contract. Later the same day, in-folder group labels bound
`--text-group-label` as part of the §02 typography align. The next QA cycle
owns visual parity against docs-page.html with Loops/Network open on desktop
and the mobile drawer.
