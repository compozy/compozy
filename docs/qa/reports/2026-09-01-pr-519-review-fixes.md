# QA Run Report — 2026-09-01 — PR 519 review fixes

- **Scope:** Worktree removal during Loop extension calls and read-only advanced Web environments
- **Cadence tier:** targeted
- **Build:** working tree for PR 519 · **Environment:** isolated lab, daemon `http://127.0.0.1:57779`, Web `http://localhost:3000`
- **Started:** 2026-09-01T15:11:10Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Session |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-worktree-tool-root |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-web-environment-authoring |

## Session Matrix & Results

| # | Charter | Journey / Scenario | Tour | Status |
|---|---|---|---|---|
| 1 | CH-loop-worktree-tool-root | J-isolated-task-loop-execution / LP-loop-environment-resolution | Feature Tour | Pass |
| 2 | CH-loop-web-environment-authoring | J-isolated-task-loop-execution / LP-worktree-web-loop-environment | Feature Tour | Pass |

## Session Debriefs

### CH-loop-worktree-tool-root — Ada

- A real one-action Loop claimed `run.loop.looprun-c6a1c35ff80474c8.g1.node.load_tasks.0` and held
  `ext__spec_cycle__import_tasks` open against ready Worktree `wt_6a764ac4712021cf`.
- Concurrent structured CLI removal returned exit 65 and `worktree_operation_in_progress`.
- The tool then completed with the task path under the selected Worktree; removal succeeded only
  after the tool released its lease.
- Scenario settled: LP-loop-environment-resolution → pass.

### CH-loop-web-environment-authoring — Bruno

- The live configure form continued to offer only Inherit, Workspace root, and Named worktree for
  authored choices.
- CLI-authored directory and per-run values rendered as disabled read-only buttons on fresh loads.
- The directory textbox preserved its exact absolute value.
- Scenario settled: LP-worktree-web-loop-environment → pass.

## Paper Cuts

None in scope.

## Runtime Errors Observed

- Deliberately blocked FIFO probes caused two earlier extension-host health timeouts before the
  final timestamped run. They were lab-probe artifacts, not product findings; the final run
  released normally and completed through the same public runtime path.

## Final Status

- **Behavioral sessions:** PASS — 2/2 affected scenarios passed.
- **Evidence:** repository screenshots plus structured artifacts under the isolated lab.
- **Local gate:** PASS — `make gate` completed the scoped Go lint, Go race-test, and Web
  lint/typecheck/test lanes with zero failures.
- **Teardown:** PASS — `qa/teardown.json` records `"clean": true` and no survivors.
- **QA verdict:** PASS — the worktree lease blocks removal for the complete tool call, and advanced
  Web environment values remain visible but non-interactive.
