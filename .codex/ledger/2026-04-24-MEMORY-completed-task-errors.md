Goal (incl. success criteria):

- Fix errors when opening completed workflow task detail pages, starting with `http://localhost:2323/workflows/agent-capabilities/tasks/task_1`.
- Success means root cause is identified, fixed without workarounds, covered by regression tests, verified in browser-use, and `make verify` passes.

Constraints/Assumptions:

- Must not run destructive git commands (`restore`, `checkout`, `reset`, `clean`, `rm`) without explicit user permission.
- Must use `systematic-debugging` and `no-workarounds` before fixing.
- Must use browser-use for local browser validation.
- Must use React/TypeScript/TanStack Router skills for frontend route work and `testing-anti-patterns` before test changes.
- Must use `cy-final-verify` before completion and run `make verify`.
- Existing unrelated worktree changes must not be reverted.

Key decisions:

- No fix until the exact browser/API error is reproduced and traced to source.

State:

- Implementation and verification complete.

Done:

- Read prior dev-proxy ledger for cross-agent awareness.
- Scanned recent ledgers and task files for task detail/completed-task context.
- Read browser-use, React, TypeScript, testing anti-patterns, TanStack Router, systematic-debugging, and no-workarounds instructions.
- Reproduced the failing API path for `agent-capabilities/task_1`.
- Confirmed two related failure modes:
  - Archived workflow row: detail uses active-only lookup and returns `globaldb: workflow not found`.
  - Stale active row with archived files on disk: detail reads `.compozy/tasks/<slug>/task_01.md` and returns `document_not_found`.
- Traced `TaskDetail`, `WorkflowSpec`, `WorkflowMemory*`, and `ReviewDetail` to `resolveWorkflow` + `workflowRootDir`, both active-only.
- Added regressions for archived workflow reads and stale active projections whose files live in `_archived`.
- Updated daemon `queryService` read-model routes to resolve a read target with the active or archived filesystem root.
- Verified live API:
  - `ws-9a41faacc8f4be2a` archived row returned 200 for `agent-capabilities/task_1`.
  - `ws-c830bd28d4bf6328` stale active row returned 200 for `agent-capabilities/task_1`.
- Verified browser-use route renders `Capability Catalog Loader and Validation` with no task-detail error and no console errors.
- Ran `make verify`; it passed after removing the now-unused active-only helper.

Now:

- Ready to report completion.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None for this fix. Follow-up consideration: proactively reconciling duplicate/stale workspace rows is outside this read-model repair.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-24-MEMORY-completed-task-errors.md`
- User URL: `http://localhost:2323/workflows/agent-capabilities/tasks/task_1`
