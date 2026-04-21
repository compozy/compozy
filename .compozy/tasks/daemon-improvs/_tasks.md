# Daemon Improvements Analysis - Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Canonical Daemon Contract Foundation | pending | high | - |
| 02 | Daemon Integration Harness and Validation Lane | pending | high | task_01 |
| 03 | Shared Transport Contract Migration | pending | critical | task_01, task_02 |
| 04 | Client and Run-Reader Contract Adoption | pending | high | task_01, task_02, task_03 |
| 05 | Runtime Shutdown, Logging, and Storage Discipline | pending | critical | task_02, task_03 |
| 06 | ACP Liveness, Subprocess Supervision, and Reconcile Hardening | pending | critical | task_02, task_03, task_05 |
| 07 | Observability, Snapshot Integrity, and Transcript Assembly | pending | critical | task_03, task_04, task_05, task_06 |
| 08 | Daemon Improvements QA plan and regression artifacts | pending | high | task_03, task_04, task_05, task_06, task_07 |
| 09 | Daemon Improvements QA execution and operator-flow validation | pending | critical | task_08 |
