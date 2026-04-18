## TC-FUNC-001: Workspace Registry CLI Flow

**Priority:** P0 (Critical)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 12 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(WorkspaceCommandsReflectDaemonRegistryAgainstRealDaemon|WorkspacesUnregisterRejectsActiveRunsAgainstRealDaemon)' -count=1`
- Supporting seam: `internal/cli/daemon_commands_test.go`
**Automation Notes:** These tests exercise the public `workspaces` commands against a real daemon instance and validate operator-visible path resolution, registry output, and active-run conflict handling.

### Objective

Verify that `compozy workspaces register|show|list|unregister` behaves as the daemon-native operator control plane, including path normalization and active-run protection.

### Preconditions

- [ ] Real-daemon CLI integration tests can launch with isolated home/runtime directories.
- [ ] At least one temp workspace fixture is available.

### Test Steps

1. Run the focused CLI workspace-registry suite listed above.
   **Expected:** The package exits `0` and all listed subtests pass.

2. Confirm the suite validates `register`, `show`, and `list` against the daemon registry rather than local filesystem heuristics.
   **Expected:** The operator output reflects daemon-owned workspace records and stable IDs/paths.

3. Confirm the suite validates unregister conflict behavior while a workspace still has active runs.
   **Expected:** Unregister is rejected with stable, operator-readable conflict behavior.

4. Review the supporting command-wiring tests if root-cause evidence is needed.
   **Expected:** JSON/text output formatting and daemon-bootstrap helpers remain stable.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Path ref instead of ID | `id-or-path` CLI input | Path resolves to the registered workspace ID |
| Workspace subdirectory | Command invoked below the workspace root | Registry operations still target the correct workspace |
| Active-run unregister | Unregister while daemon has active runs | Stable conflict response, no registry corruption |

### Related Test Cases

- `TC-INT-001`
- `TC-FUNC-003`

### Traceability

- TechSpec Workspace registry rules: path normalization, lazy register and explicit register parity, unregister rejection with active runs.
- ADR-001: explicit workspace registry for a global daemon.
- ADR-004: explicit workspace operations remain part of the final UX.
- Task reference: `task_14`.

### Notes

- Treat this as the control-plane identity check for later sync/archive and run-lifecycle scenarios.
