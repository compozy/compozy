# Daemon — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Home-Scoped Daemon Bootstrap | pending | high | — |
| 02 | Global DB and Workspace Registry | pending | high | task_01 |
| 03 | Run DB and Durable Run Store | pending | high | task_01 |
| 04 | Shared Transport Core | pending | critical | task_01, task_02, task_03 |
| 05 | Daemon Run Manager | pending | critical | task_02, task_03, task_04 |
| 06 | Reconciliation, Retention, and Graceful Shutdown | pending | high | task_03, task_05 |
| 07 | Sync Persistence Rewrite | pending | high | task_02 |
| 08 | Active-Run Watchers and Legacy Metadata Cleanup | pending | high | task_05, task_07 |
| 09 | Archive Rewrite on DB State | pending | medium | task_07, task_08 |
| 10 | Extension Runtime Daemon Adaptation | pending | high | task_04, task_05 |
| 11 | CLI Daemon Client Foundation | pending | high | task_04, task_05 |
| 12 | TUI Remote Attach and Watch | pending | high | task_04, task_05, task_11 |
| 13 | pkg/compozy/runs Daemon-Backed Migration | pending | high | task_04, task_05 |
| 14 | Workspace, Daemon, Sync, and Archive Command Completion | pending | high | task_06, task_09, task_11 |
| 15 | Reviews and Exec Flow Migration | pending | high | task_05, task_10, task_11 |
| 16 | Regression Coverage, Docs, and Migration Cleanup | pending | medium | task_12, task_13, task_14, task_15 |
