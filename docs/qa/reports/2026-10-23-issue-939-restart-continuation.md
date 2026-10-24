# QA Run Report — 2026-10-23 — Issue 939 restart continuation

- **Scope:** interrupted sessions, shutdown admission, subagent result delivery, and Loop recovery ownership.
- **Cadence tier:** targeted.
- **Build:** PR #942, rebased onto main including #940; first-review remediation checks run before their commit. Current-head CI is recorded separately.
- **Environment:** disposable homes, sockets, ports, and real daemon subprocesses using the repository's ACP mock agent. No operator runtime was restarted.
- **Status:** in-progress.

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status | Coverage note |
|---|---|---|---|---|---|
| CH-daemon-restart-continuation | J-daemon-restart-continuation / RT-daemon-restart-continuation | Dora | Interrupt | Pending | Fresh structured reads after final rebase; Web interaction remains separate from the subprocess probe. |
| CH-daemon-restart-continuation | RT-daemon-drain-admission | Dora | Interrupt | Pending | Boot admission and shutdown budget owning tests passed; final composition-root rerun pending. |
| CH-daemon-restart-continuation | RT-subagent-restart | Dora | Interrupt | Pending | Rebased real subprocess SIGKILL probe passed; fixture provider homes and discovery are isolated. |

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

## First Review and CI Remediation

The original session contract tests caught an extra checkpoint event changing ordinary transcript
sequence numbers and managed Goal metadata. The dispatch checkpoint now lives in the existing atomic
session metadata. The unchanged failing suites, plus #940 inventory/cache contracts, passed (18.899s).
Existing metadata upgrades additively; no SQL schema or public DTO changes are involved.

Focused reruns after the review fixes passed: session recovery/permissions/batched-policy/deterministic
fingerprinting/stop receipt (10.472s), boot admission (0.198s), Loop/boot composition (1.850s), and the
combined ordinary/subagent real-process probes (19.800s). The boot fence cannot be opened through
public undrain; asynchronous Loop dispatch retains the composition-root admission context.

A subagent-probe teardown failure revealed installed-provider discovery subprocesses. The fixture now
persists discovery opt-outs, assigns an isolated HOME/CODEX_HOME, and passes the prebuilt mock driver
into the helper. The final probe passed including cleanup; no retries or weakened cleanup assertions
were added. One failed setup's isolated temporary directory was removed after confirming no owned
process remained.

## Limits and Remaining Work

No full local build, full suite, E2E, race/coverage, or `make gate` ran: the operator explicitly reserved those gates for CI because the shared machine was overloaded. The ACP mock is an authorized lab boundary; this is not evidence of live-provider availability or a browser usability walkthrough. Current-head CI, final rebased lab evidence, CodeRabbit, and Greptile remain required before PR delivery.

No human-only verification or product decision is currently required. This report remains open until the remaining scoped checks and scenario write-back are complete.
