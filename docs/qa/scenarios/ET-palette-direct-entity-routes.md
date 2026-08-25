---
id: ET-palette-direct-entity-routes
area: ET
title: Open a workspace loop from the palette on its own target
persona: Sol
journey: J-command-os-from-palette
expected: Selecting a concrete workspace Loop from the root command palette after choosing its workspace opens the loop detail route directly, and the same route remains readable through an independent deep link.
entry_points: Command-K root search; Global workspace scope
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-eng-131-palette-direct-20260825-004748-874920-lab/qa-artifacts/evidence/direct-loop-detail.png; /Users/pedronauck/dev/qa-labs/compozy-eng-131-palette-direct-20260825-004748-874920-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-24-eng-131-palette-direct.md
overlaps: ET-palette-domain-views;ET-palette-nested-views;ET-palette-action-panel
---

Added for the targeted ENG-131 walk. This scenario owns the real workspace-loop journey recorded
in the report; focused automated suites cover the other direct-route projections, while the broader
root/pushed entity matrix remains a separate follow-up scenario rather than an unearned pass here.
