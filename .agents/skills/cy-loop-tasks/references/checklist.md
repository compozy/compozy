# Transition audit

Use the relevant section on resume or suspected state/evidence drift. The phase
procedure owns normal completion checks; this audit neither adds another test/QA
run nor requires a second printed report.

## Shared invariants

- The detector's action matches current task frontmatter and writer-owned state.
- Meaningful task evidence/decisions are in memory before completion is recorded.
- Required checks cover their claimed inputs. Worker evidence is inspected and
  reused where valid; a phase transition or timestamp alone does not invalidate it.
- Pending integration checks have an owner and are not reported as executed.
- Real failures are repaired before closing the action; external blockers have
  evidence. Search misses and normal wait expiry do not start recovery.
- Workers use TUIs, preserve unrelated edits, and do not commit. Reuse them for
  planned integration follow-ups; retire when the assignment is settled.
- The iteration summary cites evidence and the checkpoint result; completed
  non-E actions continue at detect without waiting for a new invocation.
- `goal_signature` remains unchanged. Scope decisions and overrides live in memory.

## Phase 0

- `_spec.md` exists; mode follows filesystem truth; memory and state are initialized.
- `--frontend` and `--stacked` reflect the invocation. Stacked prerequisites are
  checked when applicable.
- Existing spec/preflight evidence is reused; missing or changed facts are checked.

## Phase B

- One task or coherent free slice owns the action. It was marked active before work.
- The selected lane matches the task type/free-slice owned surfaces.
- Its outcome has focused owning-suite/probe evidence and the diff was reviewed.
  Explicit task-owned live/visual acceptance is complete.
- Remaining changed/integration journeys and visual rows are named for Phase C;
  affected scenario files are added/reset. No full QA cycle is imposed per task.
- Required local commit checks passed under repository policy using valid cached
  evidence; a focused PASS alone does not waive the commit gate.
- Memory, task status/progress, and state reflect the same completed scope.
- A free-mode `deliverables_complete` records implementation completion and the
  remaining integration obligations; it does not assert Phase C already ran.
- The checkpoint result is recorded. Only owned files are staged; the add-all
  helper is used only when every dirty path belongs to this checkpoint.
- Peer-review rounds remain in Phase D.

## Phase C

- QA scope is reconciled before execution. Unchanged plans and valid prior walks
  are reused; remaining scope follows the diff, contracts, and integration risks.
- Planning was local or delegated as useful; delegation is not a completion gate.
- Runtime bootstrap occurred only when the selected execution needed a lab.
  Every created lab is torn down with `teardown.json` reporting `clean: true`.
- Remaining walks/visual rows passed, or no-work/reuse disposition is documented
  with concrete coverage. Unexecuted checks are not labeled as passing runs.
- Found bugs are fixed and affected journeys re-walked in the existing report;
  unaffected charters are not restarted.
- Both QA flags represent resolved scoped obligations. In tasks mode, corresponding
  QA task completion is recorded so the pending queue drains.

## Phase D

- Phase B is complete and QA obligations are resolved before review.
- One `deep-review` round ran with the full loop diff/spec; subsequent rounds reuse
  incremental review state. The verdict and resolved findings are recorded.
- Confirmed findings are fixed; skipped nits have a reason. Only invalidated
  checks/journeys/visual rows are repeated after remediation.
- The local commit gate is satisfied, the round records `--verify-pass`, and the
  owned checkpoint result is recorded. A clean review adds no QA cycle.

## Phase E

- State has `qa.report_done=true`, `qa.execution_done=true`, `review.ship=true`,
  and `verify.last_status=PASS`; underlying evidence still covers the final inputs.
- No delivery-owned integration or visual requirement remains pending.
- Local commit/push policy is satisfied; all required PR checks are green at the
  current recorded heads. No repair commit remains unpushed.
- The done-signature is emitted only after those conditions pass and is the final
  output line. Pending/red CI keeps delivery in progress.
