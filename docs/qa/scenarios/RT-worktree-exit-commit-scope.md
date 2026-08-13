---
id: RT-worktree-exit-commit-scope
area: RT
title: Commit the complete visible worktree change set
persona: Ada
journey: J-worktree-management
expected: The assisted commit preview names untracked files, excludes ignored files, reports the bounded change summary, and commits exactly the full visible worktree state with the supplied or deterministic default message.
entry_points: compozy worktree exit|commit -o json; GET .../exit; POST .../exit/actions
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-dirty-exit-remove.jsonl; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-cli-lifecycle; RT-worktree-api-surface-parity
---

QA impact: Task 05 adds the named-untracked stage-all commit contract and streamed step results.
