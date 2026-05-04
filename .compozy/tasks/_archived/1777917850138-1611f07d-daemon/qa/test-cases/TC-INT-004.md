## TC-INT-004: Temporary Node Workspace Task-to-Review Flow

**Priority:** P1 (High)
**Type:** Integration
**Status:** Not Run
**Estimated Time:** 20 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'TestTaskAndReviewCommandsExecuteDryRunAgainstTempNodeWorkspace' -count=1`
- Live operator commands captured under `.compozy/tasks/daemon/qa/logs/node-e2e-*.log`
**Automation Notes:** This case proves the daemon-backed `tasks` and `reviews` surfaces work against a realistic external workspace instead of only repository-owned Go fixtures. The automated test keeps the flow repeatable, while the captured live commands prove the real CLI binary, daemon bootstrap, setup, and temp workspace all cooperate end to end.

### Objective

Validate a realistic operator flow against a temporary Node.js API workspace: install required Compozy skills, start the daemon explicitly, validate and sync workflow artifacts, run `compozy tasks run` in dry-run mode, inspect the review round through `compozy reviews list/show`, and start `compozy reviews fix` in dry-run mode.

### Preconditions

- [ ] A temporary Node.js workspace exists with `package.json`, `src/server.js`, `test/server.test.js`, and `.compozy/tasks/node-health/` artifacts.
- [ ] The temp Compozy home is isolated from unrelated daemon state.
- [ ] `compozy setup --agent codex --global --yes` has been run for the temp home before `reviews fix`.
- [ ] The daemon can bind its UDS socket and localhost HTTP port for the temp home.

### Test Steps

1. Run the automated CLI dry-run coverage command listed above.
   **Expected:** The package exits `0`, the task run writes prompt/result artifacts, sync materializes the review round, `reviews list/show` return the expected round data, and `reviews fix` writes prompt/result artifacts.

2. In the temp workspace, run `NODE_ENV=test node --test`.
   **Expected:** The Node fixture passes its baseline health-endpoint test.

3. In the same temp workspace and temp Compozy home, run `compozy setup --agent codex --global --yes`, `compozy daemon start`, and `compozy daemon status --format json`.
   **Expected:** Required workflow skills are installed, the daemon reaches `ready`, and status reports one home-scoped daemon with zero active runs before execution.

4. Run `compozy tasks validate --name node-health`, `compozy sync --name node-health --format json`, and `compozy workspaces resolve /tmp/... --format json`.
   **Expected:** Task metadata validates, sync reports one workflow plus one review round and one review issue upserted, and the workspace resolves into the daemon registry.

5. Run `compozy tasks run node-health --dry-run --stream`.
   **Expected:** The daemon-backed task run starts, streams one completed job, and finishes successfully without needing a live ACP runtime.

6. Run `compozy reviews list node-health`, `compozy reviews show node-health 1`, and `compozy reviews fix node-health --round 1 --dry-run --stream`.
   **Expected:** The latest review summary and issue rows match the temp review round, and the review-fix run starts and completes successfully through the daemon-backed lifecycle.

7. Run `compozy daemon stop --format json` and verify `compozy daemon status --format json` reports `stopped`.
   **Expected:** Shutdown is accepted and the daemon transitions cleanly to the stopped state.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Fresh temp home | Empty `~/.compozy` equivalent | `setup`, daemon bootstrap, sync, and runs succeed without prior state |
| External workspace | Temp Node project outside the repo tree | Workspace discovery, registry resolution, and run persistence still work |
| Review-fix preflight | `reviews fix` after installing skills | Required Compozy skills are found and the dry-run review flow starts |
| Graceful shutdown | `daemon stop` after task and review runs | Daemon exits cleanly and `status` returns `stopped` |

### Related Test Cases

- `TC-FUNC-002`
- `TC-FUNC-004`
- `TC-FUNC-005`

### Traceability

- User-requested daemon QA follow-up: validate a real temporary Node.js workspace through `compozy tasks` and `compozy reviews`.
- TechSpec Testing Approach: public CLI flows must remain daemon-backed and stable for task and review execution.
- ADR-002: human-authored workflow and review artifacts remain in the workspace.
- ADR-004: operator flows should remain ergonomic across daemon bootstrap, run, and attach semantics.

### Notes

- Keep the ACP lane in `--dry-run` mode for this case. The goal is to validate daemon orchestration and artifact handling with realistic inputs, not to nest another coding runtime inside QA.
