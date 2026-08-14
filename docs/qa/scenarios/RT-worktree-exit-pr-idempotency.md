---
id: RT-worktree-exit-pr-idempotency
area: RT
title: Open or reuse the worktree pull request
persona: Ada
journey: J-worktree-management
expected: Assisted exit chooses the correct base, prefills an unambiguous base-tree pull-request template, creates at most one GitHub pull request, and reuses an existing open pull request on repetition.
entry_points: compozy worktree status --forge; compozy worktree exit|pr -o json; GET .../exit; POST .../exit/actions
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-commit-forge.jsonl; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-push-publish; RT-worktree-exit-browser-fallback
---

QA impact: Task 05 adds the `forge.provider` GitHub pull-request path and idempotent create contract.
