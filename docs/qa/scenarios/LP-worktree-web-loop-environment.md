---
id: LP-worktree-web-loop-environment
area: LP
title: Declare loop and node execution environments in the builder
persona: Ada
journey: J-isolated-task-loop-execution
expected: The loop configure dialog sets a loop-level environment default that survives an unrelated save, and agent-executing nodes expose exactly one Environment control with the locked mode vocabulary. Switching a mode writes only the companion key that mode allows. The retired working-directory field is gone from every node, and a definition still carrying it fails validation with the daemon's migration reason on the offending node.
entry_points: S12 Loop configure dialog -> Environment default; S13 loop builder -> node inspector -> Environment
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/loops/components/__tests__/loop-editor.test.tsx; web/src/systems/loops/components/__tests__/loop-run-form.test.tsx
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: LP-loop-environment-resolution
---

QA impact: Task 07 adds the loop environment default, the node Environment descriptor, and the hard cut of params.cwd from the builder.
