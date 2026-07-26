---
id: LP-delete-custom-loop
area: LP
title: Delete a custom Loop without removing its built-in source
persona: Bruno
journey: J-06
expected: The custom Loop's destructive-action modal requires intentional confirmation and, after refresh, removes that fork from catalog/detail reads without changing the built-in source Loop.
entry_points: web loop detail overflow menu; web delete Loop modal
qa_status: untested
bug_ids: BUG-20260713-custom-loop-delete-missing
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-delete-restores-readonly-catalog.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-delete-restores-readonly-detail.dom.txt; /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-loop-goalless-delete-restored-bundled.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps:
---

Deletion is verified from a fresh catalog read and an old detail permalink.

2026-07-13: failed in CH-loop-goal-delete. The workspace-owned v1 detail exposes Configure, Fork & edit, Run, and Open in builder; Configure exposes only Reset to defaults/Cancel/Save. No overflow, destructive modal, or Delete action is reachable although the Web adapter and `useDeleteLoop` mutation exist.

2026-07-13: passed same-persona retest twice. Delete is workspace-only; the modal keeps its final action disabled for an incorrect name, Cancel preserves the shadow, and the exact Loop name confirms one deletion. Fresh `/loops` and detail reads restored bundled read-only `reviews-watch` v0 with `Fork & edit` and no Delete action. The strict goal-less replay repeated the full confirmation and restoration after a real run.
