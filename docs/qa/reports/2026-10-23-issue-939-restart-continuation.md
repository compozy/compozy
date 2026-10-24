# QA Run Report — 2026-10-23 — Issue 939 restart continuation

- **Scope:** interrupted sessions, shutdown admission, subagent result delivery, and Loop recovery ownership.
- **Cadence tier:** targeted.
- **Build:** PR #942, rebased onto main including #940. The recorded probe includes the review fix and public-readback assertions committed with this report, on top of `6eb8e4ab7`. Current-head CI is recorded on the PR.
- **Environment:** disposable homes, sockets, ports, and real daemon subprocesses using the repository's ACP mock agent. No operator runtime was restarted.
- **Status:** targeted automated verification complete; PR delivery gates pending.

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status | Coverage note |
|---|---|---|---|---|---|
| CH-daemon-restart-continuation | RT-daemon-restart-continuation: restart/stop and public readback | Dora | Interrupt | Pass | Three concurrent sessions, normal restart, SIGKILL, explicit stop, third boot; fresh HTTP and UDS detail/events agree. Automated scope. |
| CH-daemon-restart-continuation | RT-daemon-drain-admission: changed admission/shutdown contracts | Dora | Interrupt | Pass | Boot fence, public undrain isolation, shutdown budget/order, cancellation and real replacement probe passed. Automated scope. |
| CH-daemon-restart-continuation | RT-subagent-restart: crash and exactly-once result/wake | Dora | Interrupt | Pass | Rebased real subprocess SIGKILL probe passed; fixture provider homes and discovery are isolated. Automated scope. |
| CH-daemon-restart-continuation | Full Web/CLI persona walkthroughs of the three scenarios | Dora | Interrupt | Skipped | This implementation run uses the operator-authorized focused ACP mock probes; full local builds/E2E are prohibited. Browser usability and live-provider behavior are not certified. |

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

## Public Readback and Final Focused Probe

Evidence: [public-readback.txt](../evidence/2026-10-23-issue-939-restart-continuation/public-readback.txt).
The composition-root probe passed in **22.288s**, including subprocess and temporary-directory cleanup.
`TestBootSessionRepair` passed in **1.961s**, and changed-package lint reported **0 issues**.

For each of three sessions, each boot performs fresh GETs to
`/api/workspaces/{workspace}/sessions/{session}` and its `/events` route over both HTTP and UDS.
The two projections agree with the preserved conversation and with each other:

| Interruption | After replacement | Continuations per session | After another boot |
|---|---|---|---|
| Requested restart | `active`, badge `done`, partial and finished text retained, no failure | 1 | Same retained text and one continuation |
| SIGKILL | `active`, badge `done`, partial and finished text retained, no failure | 1 | Same retained text and one continuation |
| Explicit stop | `stopped`, badge `stopped`, cancellation diagnostic, partial text retained | 0 | Still stopped, zero continuations |

The test explicitly stops each replacement before the third boot; that boot therefore correctly
shows `stopped` while preserving completed work. Independent ACP diagnostic records confirm the
continuation counts. Queue order, pending decisions, policy changes and the durable breaker remain
owned by the session manager suite described above; this probe does not duplicate those unit seams.

The first public-readback run exposed an incorrect new test expectation: explicit cancellation
already carries a `cancellation` diagnostic without a failed badge. The assertion now checks that
existing contract explicitly. It also exposed unrelated auto-title/discovery activity in the fixture;
the probe now persists the same background-service isolation as the subagent fixture. No production
assertion or cleanup check was weakened.

The second CodeRabbit round identified one outside-diff finding: a session-specific checkpoint-read
error aborted the repair loop. The fix logs that session's error and repairs later eligible sessions;
cancellation still aborts immediately. Durable interruption capture remains fatal on error, before
decision expiration, because proceeding could discard the safety evidence needed for continuation.
Both error-isolation and cancellation cases passed in the canonical boot repair suite.

## Session Debrief

The bounded automated walk reached its restart/stop end states and independently confirmed them
through the two public transports after replacement. No restart failure badge, duplicate continuation,
or lost partial text remained in those probes. The full persona scenarios keep `qa_status: untested`:
the evidence above establishes the targeted runtime/API contracts, not every Web/CLI interaction or
side-effect branch in the broader charter. No browser paper-cut assessment was performed.

## Limits and Remaining Work

No full local build, full suite, E2E, race/coverage, or `make gate` ran: the operator explicitly reserved those gates for CI because the shared machine was overloaded. The ACP mock is an authorized lab boundary; this is not evidence of live-provider availability or a browser usability walkthrough. Final-head CI and reviewer coverage remain required before PR delivery. At `6eb8e4ab7`, CodeRabbit approved and Greptile confirmed all four prior findings addressed (5/5); all eight inline threads were resolved. The outside-diff fix still needs coverage on the next head.

No human-only verification or product decision is required for the authorized automated scope. There are no pending matrix rows. This report does not promote the broader persona scenarios to pass or claim PR readiness before final-head CI and reviews finish.
