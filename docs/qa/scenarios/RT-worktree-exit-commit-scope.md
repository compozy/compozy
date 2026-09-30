---
id: RT-worktree-exit-commit-scope
area: RT
title: Commit whole-worktree or explicitly reviewed file scopes
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

2026-09-29 selective scope: use repeated `exit --include` and pass its fingerprint with the same
`commit --include` set and `--expected-scope`. Verify that only those files enter the commit,
that an unignored `.compozy/tasks/private.md` remains local, and that unrelated pre-staged file
contents remain staged unchanged even if their working copy differs. Repeat with more than 200
untracked files and select a name beyond the bounded default preview. Change a selected file after
review and confirm refusal before staging; traversal and symlink includes must also refuse.
Domain real-Git rewalk: `TestWorktreeLifecycleIntegration/Should_commit_only_reviewed_files_while_preserving_private_and_pre-staged_work`.
CLI/API public transport rewalk remains required by the delivery report.

## Managed delivery continuation

With a real managed session bound to the checkout, read the scoped exit plan and submit `worktree deliver` with a stable delivery ID, the reviewed HEAD, selected paths, and returned scope fingerprint. Verify the command returns an operation ID, Compozy stops only the caller, and the resulting remote branch and draft PR contain only the selected commit. Unrelated files and pre-staged changes remain local. While delivery owns the checkout, another start or resume on it is refused; unrelated checkout sessions remain usable.

Repeat the same admitted intent after an interrupted commit, push, or ambiguous forge response. Verify the daemon reconciles the exact branch/base/HEAD and reuses the intended draft PR without duplicate commits or PRs. Change the base, selected content, remote, or HEAD and verify recovery refuses publication. Configure a different push URL and verify managed delivery refuses it before pushing.

PR #686 review regression: after admission, restage a selected file with different contents and
restore its reviewed working bytes. Delivery must refuse before overwriting that staged version;
repeat through recovery. Recover the daemon's recorded authorized staging, but refuse a legacy
ambiguous post-staging state. A deterministic safety refusal remains terminal after caller resume
and daemon restart. Completion observers must see the session fence released. Corrupt or unknown
journals remain unchanged, sibling recovery continues, and an interrupted deliver receipt without
a readable journal fails without stopping a session or performing Git/forge effects. Existing
`TestWorktreeManagedDeliveryIntegration` and `TestExitActions` own these regressions in CI;
historical native delivery receipts do not certify the new remediation bytes.

If the journal directory exists but cannot be listed, preserve running receipts and journal
bytes without execution effects. Once the inventory becomes readable, recover the same receipt.
This differs from an absent directory, which establishes missing journals for orphan handling.
Restore access after daemon boot and verify automatic recovery without restarting or submitting
another action. While inventory is unavailable, retries back off; recovery stops polling after
success and cancels/joins on shutdown or a later boot failure. A concurrent live delivery retains
its receipt and completes once, without recovery releasing its fence or duplicating publication.
