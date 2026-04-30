Goal (incl. success criteria):

- Create planning artifacts for `reviews-watch` under `.compozy/tasks/reviews-watch`.
- Success means `_techspec.md`, key ADRs, a compact `_tasks.md`, and enriched `task_*.md` files exist and validate.

Constraints/Assumptions:

- User explicitly requested no further clarification questions; prior Plan Mode questions already selected daemon job and auto-push policy.
- Use `cy-create-techspec`; user-provided plan is the technical clarification input.
- Workspace policy forbids destructive git commands.
- Plan accepted in Plan Mode must be persisted under `.codex/plans/`.

Key decisions:

- `reviews watch` is a daemon-owned parent run that orchestrates child `reviews fix` runs.
- `--auto-push` forces `auto_commit=true`, allows dirty worktrees, and never cleans/restores/stages manually.
- Clean detection requires provider review status for the current PR head, not just a premature zero-item fetch.
- Task breakdown should stay compact; use 4 tasks rather than many fine-grained slices.

State:

- Compact task files created and task validation passed; repository verification is blocked by unrelated/concurrent frontend and daemon changes.

Done:

- Read `cy-create-techspec` skill.
- Confirmed target `.compozy/tasks/reviews-watch` did not exist.
- Confirmed worktree was clean before edits.
- Loaded template and ADR template.
- Wrote accepted plan to `.codex/plans/2026-04-30-reviews-watch.md`.
- Wrote `_techspec.md` and ADR-001 through ADR-003 under `.compozy/tasks/reviews-watch`.
- Refined TechSpec with config precedence, coordinator state machine, and git safety invariants.
- Ran `make verify`; frontend bootstrap/lint/typecheck/test/build passed, then Go lint/typecheck failed in modified daemon code outside this docs task.
- Read `cy-create-tasks` skill and task templates.
- Confirmed `.compozy/config.toml` is absent, so built-in task types apply.
- Explored current review CLI/API/provider/daemon/config/events/SDK files locally.
- Wrote compact `_tasks.md` plus `task_01.md` through `task_04.md`.
- `go run ./cmd/compozy tasks validate --name reviews-watch` passed with `all tasks valid (4 scanned)`.
- Re-ran `make verify`; it failed during `frontend:typecheck` in modified frontend files outside this task.

Now:

- Preparing final handoff.

Next:

- Resolve unrelated workspace typecheck failures, then rerun `make verify`.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-30-reviews-watch.md`
- `.compozy/tasks/reviews-watch/_techspec.md`
- `.compozy/tasks/reviews-watch/_tasks.md`
- `.compozy/tasks/reviews-watch/task_01.md`
- `.compozy/tasks/reviews-watch/task_02.md`
- `.compozy/tasks/reviews-watch/task_03.md`
- `.compozy/tasks/reviews-watch/task_04.md`
- `.compozy/tasks/reviews-watch/adrs/adr-001.md`
- `.compozy/tasks/reviews-watch/adrs/adr-002.md`
- `.compozy/tasks/reviews-watch/adrs/adr-003.md`
- Task validation command: `go run ./cmd/compozy tasks validate --name reviews-watch`
- Task validation result: `all tasks valid (4 scanned)`
- Verification command: `make verify`
- Latest verification failure: frontend typecheck errors in `web/src/systems/reviews/components/reviews-index-view.tsx`, `web/src/systems/reviews/components/reviews-index-view.test.tsx`, and `web/src/systems/workflows/components/workflow-inventory-view.tsx`
