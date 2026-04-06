# engine-kernel — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Events package (types + taxonomy + bus) | completed | medium | — |
| 02 | ACP ingress buffer strategy | completed | medium | — |
| 03 | Journal writer upstream of fanout | completed | high | task_01 |
| 04 | Service Kernel with typed commands | completed | medium | task_01 |
| 05 | Executor integration and post-execution event emission | completed | critical | task_01, task_02, task_03, task_04 |
| 06 | TUI decoupling via bus-to-uiMsg adapter | completed | medium | task_05 |
| 07 | Reader library over .compozy/runs/ | completed | high | task_01 |
| 08 | CLI kernel bootstrap, command refactor, and documentation | pending | high | task_04, task_05 |
