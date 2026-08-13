---
id: MS-worktree-config-bootstrap
area: MS
title: Configure future worktree creation without moving existing checkouts
persona: Dora
journey: J-worktree-management
expected: Worktree defaults, parent-workspace overlays, copy and setup bootstrap, timeouts, discovery caching, and the default task mode apply live only to future operations; invalid values name the exact config key, adoption skips bootstrap, setup failure leaves a truthful ready-but-flagged worktree, and existing paths never move after a live config edit.
entry_points: config.toml [worktrees].root|run_branch_namespace|copy_list|setup_command|setup_timeout|discovery_cache_ttl; [task.orchestration.profile].default_worktree_mode; compozy config show|list|get; compozy config set|unset --scope global|workspace -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-cli-lifecycle; TA-task-per-run-worktree-isolation
---

QA impact: Task 01 adds the worktree configuration lifecycle and bootstrap contract; Tasks 04 and
08 expose the default task mode and public documentation. The walk must compare defaults with a
workspace overlay, reject each invalid shape as `worktree_config_invalid`, and prove the minimal
setup environment contains no daemon secret or inherited `GIT_*` value.
