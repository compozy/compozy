---
id: LP-child-loop-config-web-authoring
area: LP
title: Author one child Loop configuration in the Web editor
persona: Bruno
journey: J-recover-loop-node-failure
expected: A Loop author can enter typed run-loop.params.config_overrides in the Web editor, publish and re-open the same definition, then run the parent so only that child receives the finite budgets and runtime rules while stored configuration remains unchanged.
entry_points: web /loops/:name/editor; web /loops/:name/run; HTTP Loop definition and run detail; compozy loop status; compozy loop inspect
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-26-child-loop-config-overrides-web/editor-published.png; docs/qa/evidence/2026-08-26-child-loop-config-overrides-web/editor-reloaded.png; docs/qa/evidence/2026-08-26-child-loop-config-overrides-web/parent-run-done.png
last_report: docs/qa/reports/2026-08-26-child-loop-config-overrides-web.md
overlaps: LP-child-loop-config-overrides; LP-editor-authoring-walk
---

QA 2026-08-26: In a fresh isolated workspace, Bruno selected a real run-loop node, retained and
corrected malformed JSON, published numeric budgets plus a runtime-rule array, reloaded the editor,
and confirmed the same typed object through public HTTP. The provider-free parent and child reached
`done`; structured CLI showed the child sources as `per_run`, while the stored child retained cap
50, zero tokens, and no runtime rules.
