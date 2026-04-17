## Goal (incl. success criteria):

- Execute the `qa-execution` workflow for `.compozy/tasks/tasks-props` and leave fresh evidence under `.compozy/tasks/tasks-props/qa`.
- Success criteria:
- Determine the repository QA contract and in-scope surfaces for this branch.
- Run baseline verification and targeted changed-surface/regression-critical scenarios.
- Record any failures as QA issues, fix root causes if needed, and rerun affected coverage.
- Produce a fresh verification report backed by current command output.

## Constraints/Assumptions:

- Follow workspace policies from `AGENTS.md` and `CLAUDE.md`, including ledger maintenance and non-destructive git handling.
- User explicitly invoked `qa-execution`; use `.compozy/tasks/tasks-props` as the QA output root.
- The repository-wide product surface includes JS workspaces, but the task-specific changed surface under test is CLI/TUI; browser QA is out of scope unless new evidence contradicts the existing task plan.
- `make verify` is mandatory before completion.

## Key decisions:

- Use the existing task-specific QA plan and regression suite in `.compozy/tasks/tasks-props/qa/` as the execution matrix seed.
- Treat E2E support as `manual-only` for this task: the repo has strong package-level Go integration coverage, but no dedicated browser or CLI/TUI E2E harness for the changed surface.
- Use targeted `go test` commands from the authored test cases plus direct CLI help/manual doc review as the primary scenario set.
- Add a live temporary workspace scenario to close the user's "casos reais" gap: drive the public `setup` and `start` commands against a small three-task Node.js API project under `/tmp`.

## State:

- Completed; live temp-workspace proof added and fresh repository verify passed.

## Done:

- Read root instructions and the `qa-execution` skill.
- Scanned other session ledgers, including prior planning for task runtime overrides and the generic QA skill work.
- Ran the contract discovery script and reviewed QA references/checklist.
- Confirmed the user-supplied QA output path already contains test plans and test cases.
- Determined that the branch under test is `pn/task-props`.
- Created QA artifact directories:
- `.compozy/tasks/tasks-props/qa/issues`
- `.compozy/tasks/tasks-props/qa/screenshots`
- `.compozy/tasks/tasks-props/qa/logs`
- Ran dependency setup and baseline verification:
- `make deps`
- `make lint`
- `make build`
- `make test`
- All baseline commands passed; `make test` reported `DONE 1934 tests`.
- Ran targeted changed-surface regression commands from the QA plan:
- `go test ./internal/cli -run 'TestParseTaskRuntimeRule' -count=1`
- `go test ./internal/core/workspace ./internal/cli -run 'TestLoadConfigParsesStartTaskRuntimeRules|TestLoadConfigMergesStartTaskRuntimeRulesByType|TestLoadConfigRejectsUnsupportedStartTaskRuntimeRuleID|TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides' -count=1`
- `go test ./internal/core/plan -run 'TestPrepareJobsResolvesPerTaskRuntimeOverrides|TestPrepareJobsRejectsPerTaskRuntimeThatCannotReuseGlobalAddDirs' -count=1`
- `go test ./internal/core/run/internal/acpshared ./internal/core/run/ui ./pkg/compozy/events -run 'TestCreateACPClientUsesPerJobRuntimeWhenPresent|TestTimelineRuntimeMetaFallbacks|TestPayloadStructsRoundTripJSON' -count=1`
- `go test ./sdk/extension ./internal/cli -run 'TestTypedHookRegistrationCoversAllPublicHookBuilders|TestPublicHookAndHostTypesStayAlignedWithRuntime|TestStartHelpShowsTaskFlagsOnly|TestStartHelpMatchesGolden' -count=1`
- `go test ./internal/core/plan -run 'TestPreparePlanPreResolveTaskRuntimeMutationUpdatesPreparedJob' -count=1`
- `go test ./internal/core/plan ./internal/core/run/executor -run 'TestPreparePlanPostPrepareJobsRejectsRuntimeMutation|TestJobRunnerDispatchPreExecuteRejectsRuntimeMutation' -count=1`
- `go test ./internal/core/run/executor -run 'TestPrepareExecutionConfigRunPreStartRejectsPreparedStateMutation|TestPrepareExecutionConfigRunPreStartAllowsLateMutableFields' -count=1`
- `go test ./internal/cli -run 'TestStartTaskRuntimeFormPreseedsConfiguredTypeRules' -count=1`
- All targeted regression commands passed.
- Ran direct public CLI checks:
- `./bin/compozy --help`
- `./bin/compozy start --help`
- `./bin/compozy exec --help`
- `./bin/compozy ext list`
- Verified invalid public parser failures:
- `./bin/compozy start --task-runtime 'model=gpt-5.4' ...` rejected missing selector.
- `./bin/compozy start --task-runtime 'id=task_01,type=frontend,model=gpt-5.4' ...` rejected conflicting selectors.
- Ran a public dry-run workflow fixture:
- `./bin/compozy start --tasks-dir .compozy/tasks/_archived/20260405-165117-acp-integration --name acp-integration --include-completed --tui=false --dry-run --format json --task-runtime 'type=frontend,model=gpt-5.4-mini' --task-runtime 'id=task_01,reasoning-effort=xhigh'`
- Dry-run passed and wrote result artifacts under `.compozy/runs/tasks-acp-integration-7b7ede-20260417-154241-000000000/`.
- Confirmed public artifact evidence in `result.json`:
- `task_01` carried `reasoning_effort: xhigh`
- `task_03` carried `model: gpt-5.4-mini`
- Reviewed docs alignment:
- `docs/extensibility/hook-reference.md`
- `skills/compozy/references/config-reference.md`
- Attempted a real interactive TUI session from a temporary workspace fixture to inspect the form flow.
- The initial interactive form rendered, but PTY automation did not provide a reliable path through the full Huh workflow; keep TUI acceptance manual-only and rely on the supporting form preseed test.
- Created a temporary workspace at `/tmp/compozy-qa-node-api-live-WSURtR` with a three-task workflow:
- `task_01` scaffold a minimal Node.js API
- `task_02` add real HTTP tests
- `task_03` document how to run and test the project
- Ran public workspace bootstrapping:
- `COMPOZY_NO_UPDATE_NOTIFIER=1 ./bin/compozy setup --agent codex --yes --copy`
- Setup succeeded and installed the expected workflow skills into the temporary workspace.
- Ran public execution against the temporary workflow:
- `COMPOZY_NO_UPDATE_NOTIFIER=1 ./bin/compozy start --name qa-node-api --tasks-dir .compozy/tasks/qa-node-api --ide codex --tui=false --timeout 5m --format json`
- The run exited `0` and completed all three tasks; `.compozy/tasks/qa-node-api/_meta.md` now reports `Completed: 3` and `Pending: 0`.
- Confirmed generated deliverables in the temp workspace:
- `package.json`
- `src/server.js`
- `test/api.test.js`
- updated `README.md`
- Re-ran verification independently outside the Compozy agent session:
- `npm test` in `/tmp/compozy-qa-node-api-live-WSURtR` passed with `2` tests.
- A bounded `PORT=4137 npm start` smoke run responded successfully on `/health` and `/users`.
- Captured supporting logs under `.compozy/tasks/tasks-props/qa/logs/temp-node-api-*`.
- Ran fresh repository-wide final verification:
- `make verify`
- Final gate passed with `0 issues`, `DONE 1934 tests in 0.293s`, and a successful rebuild of `bin/compozy`.

## Now:

- Task complete.

## Next:

- None.

## Open questions (UNCONFIRMED if needed):

- No open functional questions. TUI end-to-end acceptance remains manual-only because the repository lacks a dedicated harness and PTY automation was insufficient for full-flow validation.

## Working set (files/ids/commands):

- `.codex/ledger/2026-04-17-MEMORY-tasks-props-qa.md`
- `.compozy/tasks/tasks-props/qa/test-plans/task-runtime-branch-test-plan.md`
- `.compozy/tasks/tasks-props/qa/test-plans/task-runtime-branch-regression.md`
- `.compozy/tasks/tasks-props/qa/test-cases/*.md`
- `/tmp/compozy-qa-node-api-live-WSURtR`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-setup.stdout.log`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-setup.stderr.log`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-start.jsonl`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-start.stderr.log`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-independent-npm-test.log`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-independent-start.log`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-independent-health.json`
- `.compozy/tasks/tasks-props/qa/logs/temp-node-api-independent-users.json`
- `.agents/skills/qa-execution/SKILL.md`
- Commands:
- `python3 .agents/skills/qa-execution/scripts/discover-project-contract.py --root .`
- `make deps`
- `make lint`
- `make build`
- `make test`
- `make verify`
- targeted `go test ... -run ... -count=1`
- `COMPOZY_NO_UPDATE_NOTIFIER=1 ./bin/compozy setup --agent codex --yes --copy`
- `COMPOZY_NO_UPDATE_NOTIFIER=1 ./bin/compozy start --name qa-node-api --tasks-dir .compozy/tasks/qa-node-api --ide codex --tui=false --timeout 5m --format json`
- `npm test` (in `/tmp/compozy-qa-node-api-live-WSURtR`)
- bounded `npm start` + `curl /health` + `curl /users` smoke run in `/tmp/compozy-qa-node-api-live-WSURtR`
- `.compozy/tasks/tasks-props/qa/verification-report.md`
