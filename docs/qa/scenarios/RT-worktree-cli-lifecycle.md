---
id: RT-worktree-cli-lifecycle
area: RT
title: Manage a worktree lifecycle through structured CLI output
persona: Ada
journey: J-worktree-management
expected: Creating, cancelling, adopting, listing, inspecting, refreshing status, executing or cancelling an assisted exit, refusing an unsafe removal, forcing removal, and dismissing through `compozy worktree` never falls back to the workspace root; every `<ref>` verb behaves the same for a name or ID, returns truthful structured output and deterministic exit codes, and remove → dismiss makes the old name reusable without losing ID-addressed history.
entry_points: compozy worktree list|create|cancel|adopt|inspect|status|exit|commit|push|pr|exit-cancel|remove|dismiss -o json|jsonl
qa_status: pass
bug_ids: BUG-20260814-worktree-mutation-output-loses-identity
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-create.jsonl; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-dirty-exit-remove.jsonl; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-force-remove.json; internal/daemon/daemon_worktree_e2e_integration_test.go; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-status-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-remove-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-dismiss-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-recreate-same-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-old-id-tombstone.json
last_report: docs/qa/reports/2026-08-14-worktree-lifecycle-fixes.md
overlaps: RT-worktree-api-surface-parity
---

QA impact: Task 02 adds the public worktree CLI lifecycle. The Phase C walk must cover an empty
list, pending-to-ready creation, cached and refreshed status, the dirty two-step removal refusal,
creation cancellation, adoption retry, forced removal, tombstone dismissal, and structured not-found behavior.

QA impact: Task 05 adds assisted exit verbs. The Phase C walk must verify the suggested action,
blocked reasons, operation id, cancellation, and exactly one successful completion action.

2026-08-14 lifecycle fix flag: cross a name through inspect, status, create cancel, exit actions,
exit cancel, remove, and dismiss; then recreate the dismissed name and confirm the old row remains
readable by ID.

2026-08-14 targeted walk and governed replay: status, remove, and dismiss accepted the name and
returned the canonical row identity. The catalog name was reused with the intentionally retained
Git branch, producing a distinct ready row while the dismissed predecessor remained readable by ID.

2026-10-19 #813 impact: `remove` and `dismiss` accept `--delete-branch` (and `dismiss` accepts
`--force` for unmerged branches). Receipts gain a `branch` object only when the flag is used; plain
calls are unchanged. Isolated-lab walk on 2026-10-19 verified a plain removal receipt without
`branch`, a `root_checkout_branch` refusal, a branch-only retry on the already-removed record, and a
`not_fully_merged` refusal followed by `--force` deletion. Evidence is in the PR description.

2026-10-19 merge-train branch-safety follow-up: force must keep branches when a remote exists
without a known `origin/HEAD`; a replacement registration at a dismissed record's old path must
remain unchanged, with the old branch kept. Git read failure/cancellation must preserve the
original error instead of claiming `branch_absent`, and a subsequent valid retry must acquire the
released usage/repository locks. The existing real-Git lifecycle integration suite passes these
cases with race detection, including default `trunk`, replacement branch and failed-read retries.

2026-10-20 discovered-removal impact: `remove --path <listed path>` removes a discovered checkout
without adoption. Isolated-lab walk on 2026-10-20: a prunable entry cleared; a dirty checkout
refused with `worktree_dirty_requires_force` (exit 65, file kept) and `--force` removed it while its
branch survived; `<ref>` plus `--path` and `--path` plus `--delete-branch` fail as usage errors.

2026-10-24 #953 impact: a checkout whose `.git` link is gone while Git lists it as prunable (an
interrupted removal) must refuse `remove <ref>` and `remove --path <listed path>` with
`worktree_checkout_orphaned` (exit 65, folder kept, `risk.leftover_files` > 0, a detached HEAD's
unique commits in `risk.unpushed_commits`), never a 500 `worktree_removal_failed`. `--force` deletes
the folder and clears only that Git entry; the record becomes removed and the path leaves
`git worktree list`. Text output names the reason after the code. A record left `removing` by a
daemon stop returns to `ready` on boot without failing startup.

2026-10-31 #1078 impact: a creation whose branch or checkout phase fails must stay listed as
`failed` with `pending_phase` and a redacted `create_error`, never vanish as `worktree_not_found`.
Rollback removes the linked checkout and its Git admin entry first — including the locked,
half-written checkout a killed `git worktree add` leaves — and deletes the branch only when the
creation minted it and the checkout is gone; an incomplete rollback keeps the branch and says
`rollback incomplete`, and the next daemon start retries that rollback for the checkout and branch Git
still attributes to the record, then keeps only the original cause. Isolated-lab walk on 2026-10-31 (30,000-file repo,
`worktrees.checkout_timeout = "150ms"`): `create` returned `pending`; `inspect` showed `failed`,
`pending_phase: checkout` and the `checkout exceeded worktrees.checkout_timeout` cause, with a
"Creation Failure" section in text output; `git worktree list` listed only the main checkout, with
no `.git/worktrees` entry and no `big-one` branch; the daemon log held a WARN
`worktree creation failed and was rolled back`; `logs --type worktree.creation_failed` returned the
event with phase and cause; `session new --worktree big-one` refused with `worktree_not_ready` and the
cause; `dismiss` released the name, and after raising the timeout to `15m` the same name reached
`ready`. Evidence is in the PR description.
