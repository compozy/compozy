# Daemon — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Home-Scoped Daemon Bootstrap | completed | high | — |
| 02 | Global DB and Workspace Registry | completed | high | task_01 |
| 03 | Run DB and Durable Run Store | completed | high | task_01 |
| 04 | Shared Transport Core | completed | critical | task_01, task_02, task_03 |
| 05 | Daemon Run Manager | completed | critical | task_02, task_03, task_04 |
| 06 | Reconciliation, Retention, and Graceful Shutdown | completed | high | task_03, task_05 |
| 07 | Sync Persistence Rewrite | completed | high | task_02 |
| 08 | Active-Run Watchers and Legacy Metadata Cleanup | completed | high | task_05, task_07 |
| 09 | Archive Rewrite on DB State | completed | medium | task_07, task_08 |
| 10 | Extension Runtime Daemon Adaptation | completed | high | task_04, task_05 |
| 11 | CLI Daemon Client Foundation | completed | high | task_04, task_05 |
| 12 | TUI Remote Attach and Watch | completed | high | task_04, task_05, task_11 |
| 13 | pkg/compozy/runs Daemon-Backed Migration | completed | high | task_04, task_05 |
| 14 | Workspace, Daemon, Sync, and Archive Command Completion | completed | high | task_06, task_09, task_11 |
| 15 | Reviews and Exec Flow Migration | completed | high | task_05, task_10, task_11 |
| 16 | Daemon Performance Optimizations | completed | critical | task_05, task_06, task_08, task_12, task_13, task_15 |
| 17 | Regression Coverage, Docs, and Migration Cleanup | completed | medium | task_12, task_13, task_14, task_15 |
| 18 | Daemon QA plan and regression artifacts | completed | high | task_12, task_13, task_14, task_15, task_16, task_17 |
| 19 | Daemon QA execution and operator-flow validation | completed | critical | task_18 |
