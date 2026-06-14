Goal (incl. success criteria):

- Implement accepted plan for production-grade compozy tasks run pre-run TUI:
  - unified workflow multi-select when running compozy tasks run interactively without workflow flags;
  - preserve headless --multiple alpha,beta compatibility;
  - workflow-scoped per-task runtime overrides for provider/IDE, model, and reasoning;
  - focused tests and clean make verify;
  - after implementation, run $cy-impl-peer-review until SHIP status.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE: all shell commands via rtk, no destructive git commands, no unrelated reverts.
- Required skills loaded this turn: brainstorming, tui-design, tui-glamorous, bubbletea, systematic-debugging, no-workarounds, golang-pro, testing-anti-patterns, cy-final-verify.
- User approved defaults: multi-select unified, workflow-scoped runtime, custom Bubble Tea pre-run TUI.

Key decisions:

- Treat current --multiple limitation as a CLI flow split: multiple branch bypasses interactive collection and the existing form only has singular workflow selection.
- Keep daemon request shape unchanged; express workflow scope inside task_runtime_rules.
- Keep static config task runtime rules global-by-type only.

State:

- Primary task-run pre-run wizard has been replaced with a direct Bubble Tea model. Focused tests and full `make verify` pass. External cy-impl peer review remains.

Done:

- Final `make verify` completed successfully after fixing a package-level lint issue in agents command constants.
- `make lint` separately reported 0 issues after the goconst cleanup.
- Attempted cy-impl peer review round1 and round2 under .peer-reviews/20260614T042117Z; both compozy exec runs stayed running without verdict output and were canceled via daemon API.
- Replaced the prior Huh-backed task-run pre-run wizard with a custom Bubble Tea model covering workflow multi-select, filtering, runtime defaults, execution toggles, and review submission.
- Added focused Bubble Tea model tests for multi-workflow selection, filtered select-all, manual workflow fallback, runtime left/right cycling, and runtime-per-task toggling.
- Focused wizard/apply/interactive dispatch tests passed: `rtk go test ./internal/cli -run 'Test(TaskRunWizard|TaskRunFormInputsApplyMultipleWorkflowSelection|TasksRunInteractiveFormCanStartMultipleWorkflows)' -count=1`.
- Changed-package tests passed: `rtk go test ./internal/cli ./internal/core/model ./internal/core/plan ./internal/core/workspace -count=1` reported 679 tests passing.
- Full repository verification passed: `rtk make verify` exited 0 after frontend lint/typecheck/test/build, Go lint/test/build, and web E2E.
- Explored current CLI/form/daemon/runtime code and prior ledgers.
- Added workflow-scoped TaskRuntimeRule qualifier and target matching.
- Added CLI parsing/formatting support for workflow= in --task-runtime and config validation rejection for static workflow-scoped rules.
- Added task-run wizard form with multi-workflow selection and runtime/execution options.
- Refactored tasks run dispatch so interactive collection can choose multi-run before daemon start.
- Extended task runtime form to generate workflow-scoped type/task rules for multi-workflow selections.
- Added focused tests for parser, model matching, config validation, prepare runtime targets, wizard apply, multi-runtime form, and interactive multi-run dispatch.
- Presented and received approval for the implementation plan.
- Persisted accepted plan under .codex/plans/2026-06-14-task-run-multiple-tui.md.

Now:

- Preparing and running cy-impl-peer-review against the verified diff.

Next:

- Capture peer-review verdict and either remediate blockers or finish if SHIP.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether the external cy-impl peer-review runtime will complete this time; two earlier attempts hung and were canceled.

Working set (files/ids/commands):

- .codex/plans/2026-06-14-task-run-multiple-tui.md
- .codex/ledger/2026-06-14-MEMORY-task-run-multiple-tui.md
- internal/core/model/task_runtime.go
- internal/core/model/model_test.go
- internal/core/plan/prepare.go
- internal/core/plan/prepare_test.go
- internal/core/workspace/config_validate.go
- internal/core/workspace/config_test.go
- internal/cli/task_runtime_flag.go
- internal/cli/task_runtime_flag_test.go
- internal/cli/tasks_run_wizard.go
- internal/cli/tasks_run_wizard_test.go
- internal/cli/task_runtime_form.go
- internal/cli/form.go
- internal/cli/form_test.go
- internal/cli/agents_commands.go
- internal/cli/agents_commands_test.go
- internal/cli/daemon_commands.go
- internal/cli/root_command_execution_test.go
