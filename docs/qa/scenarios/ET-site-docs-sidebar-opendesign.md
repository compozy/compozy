---
id: ET-site-docs-sidebar-opendesign
area: ET
title: Docs sidebar matches OpenDesign section and group anatomy
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs docs, the sidebar shows warm accent-mix section labels for the eight meta.json sidebar groups, 28px single-line rows with elevated hover and active accent rail (long labels truncate with ellipsis and native title tooltip), Lucide icons only on top-level rows, and in-folder separators (e.g. Loops Operate/Author/Reference) as subordinate group labels with hairlines inside a 1px guide-line — never the same chrome as top-level sections. A folder explicitly closed while one of its descendants is active stays closed after reload; its Overview child remains independently navigable.
entry_points: compozy.com /docs; /docs/loops; docs/design/opendesign/site/site-docs-sidebar.html
qa_status: pass
bug_ids: BUG-20260730-docs-mobile-sidebar-offset; BUG-20260730-sidebar-close-lost-reload
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc02-docs-sidebar; /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc07-docs-sidebar-mobile
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-docs-first-session; ET-compozy-public-brand-navigation; ET-site-docs-typography-opendesign
---

QA impact 2026-07-29: docs sidebar chrome was refactored to the OpenDesign sidebar
contract (section vs group labels, guide-line folders, depth-0 icons). Long
nav labels were later constrained to single-line truncate to match the 28px
row contract. Later the same day, in-folder group labels bound
`--text-group-label` as part of the typography align. The next QA cycle
owns visual parity against site-docs-sidebar.html with Loops/Network open on desktop
and the mobile drawer.

QA impact 2026-07-29: top-level sidebar icons were aligned with the reference tree in
`docs/design/opendesign/site/site.js` — Migration now uses ArrowRightLeft, Autonomy ShieldCheck,
Automation Clock, Extensions Puzzle, Operations Wrench, and the root Overview row BookOpen, which
also ends the four-way reuse of Activity. Confirm every top-level row renders one distinct glyph and
that no row is iconless.

QA impact 2026-07-29 deep-review remediation: reset after persisted close state was made authoritative
over active-route expansion and sidebar geometry moved to canonical tokens.
