# Multi-Task Run — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Add Multi-Run Config and Slug Parsing Foundations | completed | medium | — |
| 02 | Add Multi-Run Daemon API Contracts and Client Surface | completed | medium | task_01 |
| 03 | Implement Daemon-Owned Sequential Multi-Run Coordinator | completed | critical | task_01, task_02 |
| 04 | Wire `tasks run-multiple` CLI Command and Non-UI Modes | completed | high | task_01, task_02, task_03 |
| 05 | Add Tabbed Multi-Run TUI Attach Experience | completed | critical | task_03, task_04 |
| 06 | Document Multi-Run Usage and Add End-to-End Coverage | completed | medium | task_04, task_05 |
