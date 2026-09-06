# Phase C — QA

Run only the printed action.

`qa_report` — select the remaining scope:

1. Reconcile task evidence and the full diff. Check existing gate records
   (`make gate-status`); run `make gate` only for missing/invalidated evidence.
   Select remaining changed/integration journeys, required visual rows, and
   any adjacent behavior with a concrete propagation risk.
2. Reuse current QA plans and verified scenarios. Use `qa-report` with
   `qa-docs-path=docs/qa` for the affected planning gaps, locally by default;
   delegate only when useful under `references/herdr-delegation.md`.
   Do not rebuild unchanged journeys/charters or bootstrap a runtime to plan.
3. Record selected scope, reused evidence, and remaining checks in
   `memory/qa-report.md`. If no applicable QA remains, record why and cite
   the covering evidence; editorial changes need no invented scenarios/lab.
4. Verify the plan or no-work disposition, then run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase C --qa-report-done --action "QA scope reconciled" --outcome completed --memory-written "memory/qa-report.md,memory/MEMORY.md"`.

`qa_execution` — local:

1. If no checks remain, go directly to step 3. Otherwise execute only the
   selected outstanding scope through `qa-execution` with
   `qa-docs-path=docs/qa`; write the dated report and update affected scenario
   verdicts. Pass the reuse/ownership decisions into the skill so it does not
   repeat covered walks or take over Phase E's PR checks. Use targeted real
   browser/runtime checks; full E2E suites require scope/risk or project policy.
   Bootstrap an isolated lab through `eng-qa-bootstrap` only when the selected
   runtime journeys require it, then tear it down on every exit with
   `teardown.json` reporting `clean: true`.
2. When the report has unresolved in-scope QA obligations or a Blocks-Completion/Data-Loss bug is
   open, keep the Phase C action open: repair every in-scope bug, rerun the
   affected QA, and update the same report. Do not restart unaffected charters or
   set `--qa-execution-done` on an intermediate failure. Pending Phase E PR
   CI is delivery work, not a reason to repeat completed QA sessions.
3. If no checks remain, record the reused evidence or applicability reason in
   `memory/qa-execution.md`; do not invoke `qa-execution`, manufacture a run
   report, or label unexecuted checks as passed. Both QA flags mean the scoped
   obligations are resolved, not that a new lab/session necessarily ran.
4. Once the applicable report or evidence reconciliation is complete, run `python3 .agents/skills/cy-loop-tasks/scripts/update-state.py <slug> --phase C --qa-execution-done --action "QA obligations resolved" --outcome completed --memory-written "memory/qa-execution.md,memory/MEMORY.md" --verify-pass`.

mode=tasks addition: when the printed QA action corresponds to the pending QA
task file, flip that task's frontmatter `status:` to `completed` and add
`--task-completed <stem>` to the same update-state call so `tasks.pending`
drains.

Done when: selected QA obligations have evidence (or a concrete no-work
disposition), memory is current, and the corresponding flag is recorded.
