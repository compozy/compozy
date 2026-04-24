## TC-INT-001: Daemon Lifecycle And Temporary Node API Project

**Priority:** P0 (Critical)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 35 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** E2E
**Automation Status:** Missing
**Automation Command/Spec:** Live CLI smoke using `./bin/compozy` with isolated temp `HOME` and temp Node.js API workspace
**Automation Notes:** This validates a realistic operator flow through public CLI/daemon boundaries. Live LLM execution may be blocked by missing ACP credentials/adapters; dry-run/local daemon proof remains required.

### Objective

Verify the daemon starts, reports status, resolves/registers a realistic Node.js API workspace, and can prepare a task run without relying on repository internals.

### Preconditions

- `./bin/compozy` exists or can be built with `make go-build`.
- Temporary workspace contains a minimal Node.js API project and `.compozy/config.toml`.

### Test Steps

1. Create a temp Node.js API project with `package.json`, `src/server.js`, and `.compozy/tasks/<slug>` task artifacts.
   **Expected:** Project is self-contained and outside the main source tree.
2. Start daemon with isolated `HOME`, `XDG_CONFIG_HOME`, and ephemeral HTTP port.
   **Expected:** `daemon status` reports running and an HTTP URL.
3. Run `workspaces resolve` and `workspaces list` for the temp project.
   **Expected:** Workspace is resolved/listed with the temp project name/path.
4. Run `tasks run <slug> --dry-run --stream --ide codex`.
   **Expected:** CLI prepares the workflow using `gpt-5.5` default without starting a live ACP session.
5. Stop daemon.
   **Expected:** `daemon stop` succeeds and status reflects stopped state.

### Edge Cases

| Variation             | Input                     | Expected Result                                                |
| --------------------- | ------------------------- | -------------------------------------------------------------- |
| Missing task metadata | Invalid task front matter | Validation fails before execution with clear message           |
| Live adapter missing  | `codex-acp` unavailable   | Dry-run/local proof passes; live execution recorded as blocked |
