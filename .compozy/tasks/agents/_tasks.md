# Agents — Task List

## Tasks

| # | Title | Status | Complexity | Dependencies |
|---|-------|--------|------------|--------------|
| 01 | Agent registry, parsing, validation, and override resolution | pending | high | — |
| 02 | Runtime resolution and canonical system prompt assembly | pending | high | task_01 |
| 03 | ACP MCP plumbing and nested `run_agent` execution engine | pending | critical | task_01, task_02 |
| 04 | CLI integration for `exec --agent`, `agents`, and hidden `mcp-serve` | pending | high | task_02, task_03 |
| 05 | Observability, safeguards, and end-to-end integration hardening | pending | high | task_03, task_04 |
| 06 | User-facing documentation and example agent fixtures | pending | medium | task_04, task_05 |
