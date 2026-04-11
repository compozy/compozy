# Extensibility — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Extract internal/core/subprocess package and add protocol version constant | completed | medium | — |
| 02 | Scaffold internal/core/extension package with manifest parser and enablement model | completed | medium | task_01 |
| 03 | Three-level discovery pipeline with provider and skill asset extraction | completed | medium | task_02 |
| 04 | Capability enforcement and audit log | completed | medium | task_02 |
| 05 | Hook dispatcher and Host API router | completed | medium | task_02, task_04 |
| 06 | Host API services for tasks, runs, memory, artifacts, prompts, and events | completed | high | task_05 |
| 07 | Early run-scope bootstrap kernel refactor | completed | high | task_03, task_04, task_05 |
| 08 | Extension manager lifecycle with spawn, initialize, shutdown, and health | completed | high | task_01, task_06, task_07 |
| 09 | Integrate extension bootstrap into start, fix-reviews, and exec commands | completed | medium | task_08 |
| 10 | Plan, prompt, and agent phase hook dispatches | completed | high | task_09 |
| 11 | Job, run, review, and artifact phase hook dispatches | completed | high | task_09 |
| 12 | CLI management commands and local enablement state | completed | high | task_03, task_04 |
| 13 | Declarative asset integration for skill packs and provider overlays | completed | high | task_03, task_12 |
| 14 | Go SDK sdk/extension package | completed | high | task_08 |
| 15 | TypeScript SDK, starter templates, and author documentation | completed | high | task_14 |
