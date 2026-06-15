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

- User accepted a replacement plan for a stepped Bubble Tea wizard: workflow selection/order, runtime, execution, nested per-workflow overrides, and review in one pre-run experience. Core implementation is in place; post-peer-review validation is in progress.

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
- Integrated workflow Run Order into the Bubble Tea wizard without alphabetical resorting, with focusable ordering controls.
- Added an Overrides step inside the wizard that loads workflow-scoped type/task targets and materializes TaskRuntimeRule values without invoking the old Huh form after the wizard.
- Changed task-run interactive apply to set executionTaskRuntimeRules, replace configured rules, and mark task-runtime explicit directly from wizard inputs.
- Added focused tests for workflow order preservation/reordering and workflow-scoped override generation.
- rtk go test ./internal/cli -count=1 passed with 488 tests.
- Peer review round 2 returned FIX_BEFORE_SHIP with blocker B-001: override selections were lost after Overrides -> back -> forward navigation.
- Fixed B-001 by syncing override rules when leaving Overrides and reusing the loaded form when the override load key is unchanged; added regression coverage.
- Fixed additional peer review risks/nits: text inputs now accept q/? without triggering global shortcuts, removed unreachable Huh runtime-form path, removed dead replaceTaskRuntime field, made override target accessors read-only during View, and handled G in workflow list.
- Post-remediation validation passed: rtk go test ./internal/cli -count=1 passed with 490 tests; changed-package tests passed with 683 tests; rtk make lint reported 0 issues; rtk make verify exited 0.
- Peer review round 4 returned FIX_BEFORE_SHIP with blocker B-001: focused text inputs still lost h/j/k/l/space to runtime/override navigation.
- Fixed round 4 B-001 by routing non-navigation keys to focused text inputs before step-level shortcuts, including the override editor parent focus switch; added regressions for runtime and override model fields accepting "haiku".
- Removed the orphaned clearTaskRunRuntimeRules helper/test after the wizard became the only tasks-run runtime override collection path.
- Focused post-fix validation passed: rtk go test ./internal/cli -run 'Test(TaskRunWizard|TaskRunFormInputsApplyMultipleWorkflowSelection|TaskRunRuntimeForm)' -count=1; full internal/cli passed with 490 tests.

Now:

- Run changed-package validation, full make verify, and cy-impl-peer-review again after fixing round 4 blocker.

Next:

- Run cy-impl-peer-review round 5 against the remediated diff until SHIP, then final report.

Open questions (UNCONFIRMED if needed):

- None for implementation scope. User confirmed task selection means runtime overrides, not execution filtering.

Working set (files/ids/commands):

- .codex/plans/2026-06-14-task-run-multiple-tui.md
- .codex/plans/2026-06-14-stepped-task-run-wizard.md
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
