# Terminal close UX — issue #594

- Scope: targeted branch validation of terminal/window close behavior.
- Environment: isolated local macOS runtime and production Web bundle; no host-daemon changes.
- Persona: Marina, an operator running shell work and managing terminal tabs.
- Status: behavioral QA PASS. Delivery gate and current-head CI are recorded in the pull request.

## Session matrix

| Journey | Observable | Status |
| --- | --- | --- |
| Running close | Cancel preserves work; confirm stops it and closes the window | PASS |
| Group close | Exactly the affected terminals are confirmed; unrelated work survives | PASS |
| Stop and exited close | Stop keeps the window; exited close needs no running warning | PASS |
| Reconnect and failure | Failure retains a retryable window; reconnect permits a fresh attempt | PASS |
| Adjacent windows | Ordinary app close and native view-only close retain their contracts | PASS |

## Charter

From the Terminal dock, run ordinary shell work, reload, manage two terminal tabs, and close through
window chrome and keyboard/menu. Try canceling, stopping first, and disconnecting the browser.
Use public CLI/API terminal and window reads to verify persisted state. Capture the confirmation,
failure, and final state. A running process without a visible, retryable window is a failure.

## Implementation and root cause

The former header action called terminal DELETE without closing the managed window. Window chrome,
keyboard/menu and tab close called only the window manager, so they detached a view while work
continued. The browser runtime now runs terminal lifecycle preparation before its existing window
close command. One confirmation covers the selected running terminals. A fresh daemon read and
layout/scope checks precede termination; the final close is revision-fenced.

## Cross-surface impact

- Native tools: `compozy__window_close` and `compozy__terminal_close` keep their separate contracts,
  schemas, policy gates, and CLI/HTTP/UDS fallbacks. Browser `window.close` client operations now use
  the same confirmation as chrome. No native close becomes implicitly process-destructive.
- Extensibility/hooks/config: no new hooks, settings, capabilities, or SDK contracts. Existing
  terminal close and window close events remain daemon-owned; the Web composes their public APIs.
- Workspace/profile isolation: the browser captures the owning desktop workspace and resolves terminal
  owners from the labeled catalog, submits exact-profile deletes, and abandons stale shell bindings.
- User state: no database/config/layout shape changes or migrations. Journals, recordings, and
  retention policies are unchanged. Shared viewers observe the same authoritative process exit.
- Internal compatibility: redundant header Close props/actions are removed together with their
  consumers. The terminal-limit dialog retains its separate slot-recovery action.
- Web/Docs: terminal header, shell close admission/dialog, browser E2E and controller/runtime suites;
  official Terminal skill, terminal safety docs and affected QA journeys/scenarios updated.

## Verification

- Focused Turbo unit run: 103 tests passed across four existing suites.
- First typecheck found a missing timestamp in the new exit fixture; corrected before final checks.
- Repository-root Turbo typecheck and production build passed.
- Final real E2E run: E2E-002, E2E-020 and E2E-019 passed (3 tests), using isolated launch runtimes and the production bundle.
- E2E-002 proves both terminal IDs survive reload/cancel, both exit after group confirmation, their windows remain closed after reload, and retained output is readable through CLI quote.
- E2E-020 disconnects the actual browser transport before confirmation; visible failure preserves the window and running process. Reconnect/reload restores the same terminal; Stop preserves its window; exited close is direct.
- Manual browser QA: Cancel receives initial focus; keyboard and window-menu close use the same dialog; two viewers observe a shared exit; Tasks closes normally. A mixed Tasks/alpha/beta group lists only running terminals. Close others cancellation preserves the group, Close right ends beta only, and final group close ends alpha/removes Tasks while an external terminal remains running.
- Native compatibility probe: CLI window close removes the managed view; a subsequent terminal get still reports running with zero viewers.
- React Doctor changed-source scan: 100/100, no issues.
- Global-scope regression: close resolves the retained desktop's owning project even while the data destination is unscoped; E2E-002 and E2E-020 passed with Global enabled before running/exited close.
- QA teardown reported `clean: true`, with no surviving registered processes.
- Delivery commands: `make gate` and current-head required PR checks; their final outcomes are recorded in the pull request.

## Runtime limitations

Goal activation and a structured read returned active/running with zero turns used. This does not
prove automatic Goal execution. The completion-report tool is unavailable to this ordinary prompt (`goal_not_active`). Host runtime repair is outside this change. Linux and packaged
Electron behavior have not been verified in this local pass.
