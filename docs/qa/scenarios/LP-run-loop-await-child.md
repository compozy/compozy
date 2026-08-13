---
id: LP-run-loop-await-child
area: LP
title: Await child Loops in authored order
persona: Ada
journey: J-recover-loop-node-failure
expected: A parent Loop stays live while each awaited child is live, starts its authored successor only after the child finishes, and remains truthful after a daemon restart.
entry_points: `compozy loop validate`; `compozy loop run`; `compozy loop status`; `compozy daemon stop|start`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/parent-before-restart.json;/Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/parent-after-restart.json;/Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/parent-during-second-child.json;/Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/parent-terminal.json;/Users/pedronauck/dev/qa-labs/compozy-issue-386-awaited-child-20260813-213849-412258-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-13-issue-386-awaited-child.md
overlaps: LP-cancel-restart-recovers
---

acceptance-walk: As Ada, validate and run a parent Loop with two awaited child Loops connected in authored order. Read the parent and child runs through structured CLI output while the first child is live, restart the daemon, then confirm the second child starts exactly once after the first child becomes terminal and the parent becomes terminal only after the second child.

QA result 2026-08-13: The public UDS CLI read showed the parent `running`, `z_first_child` as `awaiting_child` with `looprun-d67084dd1b2f9c62`, and `a_second_child` still `pending`. After the isolated daemon restart, those same run ids and states remained. The first child then reached `done`, exactly one second child (`looprun-7f6ec720fea52ca0`) began, and the parent reached `done` only after both child runs did.
