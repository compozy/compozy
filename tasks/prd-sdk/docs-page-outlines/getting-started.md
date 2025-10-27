# Page Outline: SDK Getting Started
- **Purpose:** Provide a guided setup from workspace configuration to first workflow run. Sources: tasks/prd-sdk/05-examples.md §§ 1-4, 04-implementation-plan.md bootstrap tasks, 02-architecture.md context pipeline.
- **Audience:** Engineers beginning SDK adoption.
- **Prerequisites:** Go 1.25.2 environment, installed Compozy CLI (link to CLI install guide).
- **Key Sections:**
  1. Workspace prerequisites (Go workspace setup, module import paths) referencing 04-implementation-plan.md §1.
  2. Context-first initialization (config + logger) referencing 02-architecture.md "Context-First Pattern".
  3. Building initial project + workflow using builders (summary of project/workflow builders with link to detailed pages).
  4. Running first workflow (`go run sdk/examples/01_simple_workflow.go`) with explanation from 05-examples.md.
  5. Next steps (link to testing, migration, builders directory).
- **Cross-links:** CLI quickstart for environment setup, Core workflow execution doc, API client doc for optional remote execution.
- **Examples Strategy:** Include short snippet (<15 lines) for context initialization; link to `sdk/examples/01_simple_workflow.go` and `02_parallel_tasks.go` for full code.
- **Notes:** Emphasize using `config.ContextWithManager` and `logger.ContextWithLogger` per `_techspec.md` guardrails.
