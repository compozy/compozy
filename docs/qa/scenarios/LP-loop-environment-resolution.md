---
id: LP-loop-environment-resolution
area: LP
title: Resolve each Loop node into its declared environment
persona: Ada
journey: J-isolated-task-loop-execution
expected: A Loop node overrides the loop default, resolves a ready worktree or contained directory, creates a distinct worktree for every per-run fan-out instance, and fails before session start when the environment cannot be resolved.
entry_points: compozy loop create|validate --file; compozy loop configure --file|--set; compozy loop run --config-file; HTTP/UDS Loop definition, config, and run routes; compozy__loop_create|configure|run.environment
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact: Task 04 replaces Loop `params.cwd` with the closed `environment` contract for run-agent
and goal nodes. The Phase C walk must cover root, worktree, per_run fan-out, and a templated directory,
plus the retired `cwd` validation message and run-loop inheritance.
