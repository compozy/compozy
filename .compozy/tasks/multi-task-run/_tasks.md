# Multi-Task Run — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Add Parallel Limit Workspace Configuration | completed | medium | — |
| 02 | Extend Multi-Run API and Client Contracts | completed | high | task_01 |
| 03 | Add Parallel CLI Controls and Request Wiring | completed | medium | task_01, task_02 |
| 04 | Add Multi-Run Event and Snapshot Worktree Metadata | completed | high | task_02 |
| 05 | Add Git Worktree Allocation and Path Planning | completed | high | task_04 |
| 06 | Refactor `task_multi` Into a Mode-Aware Scheduler | pending | high | task_03, task_04 |
| 07 | Register and Remap Parallel Children to Worktree Workspaces | pending | high | task_05, task_06 |
| 08 | Implement Bounded Parallel Fanout and Fail-Late Aggregation | pending | critical | task_07 |
| 09 | Render Parallel Worktree Handoff in TUI and CLI Output | pending | high | task_04, task_08 |
| 10 | Update Documentation and End-to-End Coverage | pending | medium | task_08, task_09 |
