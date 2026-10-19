---
id: RT-worktree-exit-merged-cleanup
area: RT
title: Clean up a merged worktree safely
persona: Ada
journey: J-worktree-management
expected: Assisted exit gives fresh forge-merged evidence precedence, distinguishes local-only from remote cleanup, blocks on stale or conflicting evidence, and never removes a worktree while an exit operation is running.
entry_points: compozy worktree status --forge; compozy worktree exit|remove -o json; GET .../exit; POST .../exit/actions
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/daemon_worktree_e2e_integration_test.go; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-pr-idempotency; RT-worktree-cli-lifecycle
---

QA impact: Task 05 adds merged-evidence precedence, two-tier cleanup guidance, and the atomic removal fence.

## Gone upstream after squash merge

In an isolated Git repository with a local bare remote, create and push a feature branch, squash it
into main, delete the remote feature branch, and fetch with prune. Verify:

1. Status remains readable with `has_upstream=true` and absent ahead/behind counts.
2. The exit plan describes the unavailable upstream; fresh merged forge evidence with the feature
   HEAD makes cleanup safe even though no remote reference contains that HEAD.
3. Removal without force succeeds for that clean checkout, including a record without a base ref.
4. A later local commit, wrong PR head, absent forge status, or expired evidence retains the unpushed
   refusal; explicit force still works. Uncommitted files retain the dirty refusal even with a
   matching merged PR. A persisted merge without a head SHA must obtain it from the provider; a
   provider outage cannot authorize removing unpublished commits.

Owning automated check: `CGO_ENABLED=1 go test -race -tags=integration ./internal/worktree
-run TestWorktreeLifecycleIntegration/Should_clean_up_a_gone_upstream`.
The real Git boundary is exercised; forge responses are supplied at the provider/store boundary.
