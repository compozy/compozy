---
id: MS-worktree-config-bootstrap
area: MS
title: Configure future worktree creation without moving existing checkouts
persona: Dora
journey: J-worktree-management
expected: Worktree defaults, parent-workspace overlays, copy and setup bootstrap, timeouts, discovery caching, and the default task mode apply live only to future operations; invalid values name the exact config key, adoption skips bootstrap, setup failure leaves a truthful ready-but-flagged worktree, and existing paths never move after a live config edit.
entry_points: config.toml [worktrees].root|run_branch_namespace|copy_list|setup_command|setup_timeout|discovery_cache_ttl; [task.orchestration.profile].default_worktree_mode; compozy config show|list|get; compozy config set|unset --scope user|profile|workspace -o json
qa_status: untested
bug_ids: BUG-20260813-default-home-global-config-reclassified; BUG-20260813-worktree-config-paths-not-mutable
fix_status: fixed
retest_status: pass
fix_commits: 2e741d9d; a216668f
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-081758-371939-lab/qa-artifacts/qa/bootstrap-manifest.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/daemon-status-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-operator-home-list-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/config-worktree-slow-setup-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-cancel-complete.json
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-cli-lifecycle; TA-task-per-run-worktree-isolation
---

QA impact 2026-08-22: profile config layers add a writable personal-profile target and hard-cut the
user scope name. Reset for the Task 13 layered-write walk; historical worktree-bootstrap evidence is
still useful but does not settle the new destination matrix.

QA impact: Task 01 adds the worktree configuration lifecycle and bootstrap contract; Tasks 04 and
08 expose the default task mode and public documentation. The walk must compare defaults with a
workspace overlay, reject each invalid shape as `worktree_config_invalid`, and prove the minimal
setup environment contains no daemon secret or inherited `GIT_*` value.

2026-08-13 fix replay: a fresh isolated daemon retained the native operator home, registered it as
the default workspace, and exposed a running public status without reclassifying the operator's
global config. Public config mutation then applied and removed `worktrees.setup_command` live; a
browser creation reached pending setup and cancellation removed both its checkout and branch. The
remaining invalid-value, copy, failure, timeout, cache, and default-policy matrix continues in the
Task 10 charter.
