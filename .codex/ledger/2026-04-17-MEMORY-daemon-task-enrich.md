Goal (incl. success criteria):

- Enrich the remaining daemon tasks (task_05 through task_16) with concrete codebase context, integration points, coupling risks, and test focus, without editing product code.
- Success means: a structured per-task mapping in pt-BR that names real files, dependent files, concrete requirements, and the most important tests.

Constraints/Assumptions:

- Do not edit source code for this exploration.
- Preserve the user's request to focus on runtime/CLI/TUI/public API surfaces.
- No destructive git commands.
- Final answer must stay in pt-BR.

Key decisions:

- Treat `internal/cli`, `internal/core/run`, `internal/core/tasks`, `internal/core/migration`, `internal/core/extension`, and `pkg/compozy/runs` as the highest-risk seams.
- Use the current `start`/`exec`/`validate-tasks`/`migrate`/`runs` tests as the primary regression map.
- Use the TechSpec and task list as the source of task boundaries; use code inspection to refine file-level dependencies.

State:

- Context gathered; ready to synthesize the final task-by-task enrichment.

Done:

- Read the daemon task list and TechSpec.
- Inspected CLI root/command wiring, workspace config, task validation, preflight, run execution, extension runtime, migration, reviews, run scope, journal, TUI, and public run readers/watchers.
- Collected the most relevant test files and named test cases for the affected seams.

Now:

- Produce the structured response, grouped by task 05-16.

Next:

- If asked, turn the analysis into task-file enrichment guidance or a concrete backlog order.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/_techspec.md`
- `internal/cli/root.go`
- `internal/cli/commands.go`
- `internal/cli/run.go`
- `internal/cli/validate_tasks.go`
- `internal/cli/workspace_config.go`
- `internal/cli/task_runtime_form.go`
- `internal/core/run/run.go`
- `internal/core/run/preflight.go`
- `internal/core/run/executor/*`
- `internal/core/run/ui/*`
- `internal/core/tasks/*`
- `internal/core/migration/*`
- `internal/core/reviews/*`
- `internal/core/model/*`
- `internal/core/extension/runtime.go`
- `pkg/compozy/runs/*`
