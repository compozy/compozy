# QA Run Report — 2026-10-23 — Issue 939 restart continuation

- **Scope:** interrupted sessions, shutdown admission, subagent result delivery, and Loop recovery ownership.
- **Cadence tier:** targeted.
- **Build:** issue-939-restart-resume working branch; current-head delivery evidence will be recorded after rebase.
- **Environment:** disposable homes, sockets, ports, and real daemon subprocesses using the repository's ACP mock agent. No operator runtime was restarted.
- **Status:** in-progress.

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status | Coverage note |
|---|---|---|---|---|---|
| CH-daemon-restart-continuation | J-daemon-restart-continuation / RT-daemon-restart-continuation | Dora | Interrupt | Pending | Fresh structured reads after final rebase; Web interaction remains separate from the subprocess probe. |
| CH-daemon-restart-continuation | RT-daemon-drain-admission | Dora | Interrupt | Pending | Boot admission and shutdown budget owning tests passed; final composition-root rerun pending. |
| CH-daemon-restart-continuation | RT-subagent-restart | Dora | Interrupt | Pending | Real subprocess SIGKILL probe passed after fixing result delivery during parent resume. |

## Automated Evidence Collected During Implementation

- `TestDaemonRestartContinuesSessions`: three simultaneous mid-turn sessions, requested restart, SIGKILL, and explicit stop. Partial text and native ACP identity survive. A third boot does not duplicate continuation. ACP diagnostics independently count three continuations after restart/crash and zero after explicit stop.
- `TestSubagentCrashRestartDaemonIntegration`: real daemon process killed mid-turn; one child continuation, one parent continuation, and one delivered result. A repeated recovery does not duplicate settlement or the wake.
- `TestResumeInterruptedSession`: durable SQLite admissions; exactly-once admission, opt-out, explicit stop, policy/route changes, pending permission and clarification, maintenance, completion during drain, and ordered queued inputs.
- `TestResumeInterruptedSessionCrashLoop`: three persisted continuation attempts across manager reopen; the fourth interruption is gated.
- `TestLoopActionBootRestartRecovery`: the Loop owner reserves eligible continuations and pauses unsafe nodes before generic lease recovery.
- Focused config upgrade/default tests, boot admission, shutdown budget/order, CLI relaunch timeout, and ACP resume-only capability checks passed.
- React Doctor inspected the two changed React files: **100/100**, no findings.
- The lifecycle matrix and native tool catalog were regenerated through their owning generators. The native tool catalog had no diff.
- Isolated lab teardown reported all tracked processes clean. Test fixtures also own and clean their subprocesses and temporary directories.

## Fixes Found by the Probes

- A ready runtime retains its last binding strategy. Treating that strategy as an unfinished transition falsely classified SIGKILL as an agent crash; readiness now distinguishes those cases.
- Reordering boot inventory ahead of pending stop settlement broke an existing recovery guarantee. Pending stops again settle before the repair inventory; restart capture precedes decision expiry and history repair.
- A subagent result could be disposed while its parent was in `starting` during resume. Delivery now remains pending across that transition.

## Limits and Remaining Work

No full local build, full suite, E2E, race/coverage, or `make gate` ran: the operator explicitly reserved those gates for CI because the shared machine was overloaded. The ACP mock is an authorized lab boundary; this is not evidence of live-provider availability or a browser usability walkthrough. Current-head CI, final rebased lab evidence, CodeRabbit, and Greptile remain required before PR delivery.

No human-only verification or product decision is currently required. This report remains open until the remaining scoped checks and scenario write-back are complete.
