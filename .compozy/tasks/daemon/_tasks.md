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
| 10 | Extension Runtime Daemon Adaptation | pending | high | task_04, task_05 |
| 11 | CLI Daemon Client Foundation | pending | high | task_04, task_05 |
| 12 | TUI Remote Attach and Watch | pending | high | task_04, task_05, task_11 |
| 13 | pkg/compozy/runs Daemon-Backed Migration | pending | high | task_04, task_05 |
| 14 | Workspace, Daemon, Sync, and Archive Command Completion | pending | high | task_06, task_09, task_11 |
| 15 | Reviews and Exec Flow Migration | pending | high | task_05, task_10, task_11 |
| 16 | Regression Coverage, Docs, and Migration Cleanup | pending | medium | task_12, task_13, task_14, task_15 |
| 17 | Daemon QA plan and regression artifacts | pending | high | task_12, task_13, task_14, task_15, task_16 |
| 18 | Daemon QA execution and operator-flow validation | pending | critical | task_17 |
