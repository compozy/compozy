---
id: ET-site-docs-sidebar-opendesign
area: ET
title: Docs sidebar matches OpenDesign section and group anatomy
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs docs, the sidebar shows warm accent-mix section labels for the eight meta.json sidebar groups, 28px single-line rows with elevated hover and active accent rail (long labels truncate with ellipsis and native title tooltip), Lucide icons only on top-level rows, and in-folder separators (e.g. Loops Operate/Author/Reference) as subordinate group labels with hairlines inside a 1px guide-line — never the same chrome as top-level sections.
entry_points: compozy.com /docs; /docs/loops; docs/design/opendesign/docs/docs-design-system.html §03
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-07-29 walk on a local `next build` + `next start :4598`. All routes returned 200: /docs/, /docs/examples/ and its five wave-one pages, /marketplace/, /marketplace/{skills,mcp,extensions}/, /marketplace/bridges/, /marketplace/bundled/dev-cycle/, /marketplace/mcp/context7/, and the bridge setup guides. Visual-contract bundles VC-01..VC-05 under .compozy/tasks/site-docs-ia/evidence/visual/gap-closure/ validate PASS with 0 blocking divergences. VC-04 shows one distinct glyph per top-level row — Migration ArrowRightLeft, Autonomy ShieldCheck, Automation Clock, Extensions Puzzle, Operations Wrench, Overview BookOpen — with no iconless row and no remaining four-way Activity reuse.
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

QA impact 2026-07-29: top-level sidebar icons were aligned with the reference tree in
`docs/design/opendesign/site/site.js` — Migration now uses ArrowRightLeft, Autonomy ShieldCheck,
Automation Clock, Extensions Puzzle, Operations Wrench, and the root Overview row BookOpen, which
also ends the four-way reuse of Activity. Confirm every top-level row renders one distinct glyph and
that no row is iconless.
