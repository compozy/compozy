# Goal control — issue #1015

Scope: session-origin Goal controls, their interaction with session Stop, terminal persistence,
and the Web control surface. Branch: `fix/issue-1015-goal-control`; baseline: `8f63304b3`.

## Contract

| Action | Durable result | Runtime effect |
| --- | --- | --- |
| Pause Goal | Pause requested, then paused | Interrupt the exact managed turn; retain its terminal; admit no successor. |
| Resume Goal | Running | Explicitly continue a paused Goal after its session is active. |
| Stop Goal | Canceled, visible | Revoke Goal ownership and interrupt its work; preserve the audit. |
| Clear Goal | Hidden | Cancel live work if necessary; preserve history. |
| Stop session | Goal paused; session stopped | Fence Goal work before session teardown. |
| Send an ordinary message to a stopped session | Session active; Goal still paused | Reopen the conversation without restarting Goal work. |
| Remove session | Goal canceled | End retained work before deleting its origin. |

The operator pause actor is retained through the checkpoint boundary. Cancellation may close a
checkpoint before the provider finishes: the exact original dispatch identity can still attach its
terminal event witness to the revoked receipt, but cannot reopen the checkpoint or create another
turn. Session control commands do not implicitly reopen stopped sessions.

## Reproduction and corrections

An isolated baseline daemon with an ACP fixture held the first work turn until cancellation.
Pause remained running through the 15-second observation deadline. Baseline session Stop canceled
the Goal instead of leaving resumable paused work. Clear committed a canceled Goal, but that first
baseline walk did not independently prove provider quiescence. The historical hours-long production
incident was not replayed; targeted fault injection covers its event-read failure boundary.

The fixed walk waits for a provider message before controlling the held turn, then independently
reads Goal state, the ordered turn audit, and the provider terminal through public HTTP/UDS surfaces.
It also exercises stopping before first submission, removal, explicit continuation, approvals, and
restart persistence. A separate store test retains a late terminal after cancellation and rejects
changed identity, conflicting evidence, or duplicate completion events.

The walk exposed and repaired additional control races: queued work could never settle after its
session stopped; early Resume could attempt to create a second coordinator; canceled setup could
attempt terminal persistence using an already-canceled context; slash-command admission could reopen
the session before interpreting the command. `session.turn_quiesced` is lifecycle evidence after the
provider terminal, not a second provider result. Result-bearing events and duplicate terminals remain
invalid.

Context telemetry read failures now produce `unknown` plus `goal_context_unavailable`; durable Goal
state and controls remain available. The Web retains the last confirmed state on refresh failure,
offers Retry, and displays mutation errors beside the controls.

## Evidence

The targeted lab root is `/private/tmp/compozy-goal-control-1015-20261029-014829-546810-lab`.
Its bootstrap manifest is `qa-artifacts/qa/bootstrap-manifest.json`. Each runtime harness additionally
creates its own home, workspace, port, socket, and provider process, and owns its teardown.
No operator daemon was restarted or reconfigured.

- `qa-artifacts/baseline-held-turn.log`: baseline reproduction.
- `qa-artifacts/fixed-held-turn.log`: held-turn controls and early session-stop/removal.
- `qa-artifacts/fixed-goal-scenarios.log`: public Goal scenario suite, including restart.
- `qa-artifacts/fixed-held-resume.log`: explicit continuation and ordinary-message reopening.
- `qa-artifacts/screenshots/manifest.json`: screenshot dimensions, hashes, and story URLs.
- `qa-artifacts/screenshots/README.md`: capture commands and browser teardown evidence.

Visual evidence uses the production Goal components in deterministic Storybook states, including
active, pausing, paused, stopping, stopped, both confirmation dialogs, errors, retry, and unavailable
context, in light and dark themes. These captures establish rendering and dialog behavior; public
daemon tests establish backend effects. They are not a claim of a complete live-provider browser walk.

Six additional 1440×900 captures (23–28, `real-app-manifest.json`) exercise the real Web app against
an isolated daemon and controlled ACP provider. In one session, real button clicks demonstrated
active → pausing → paused, explicit continuation, session-stop confirmation, a stopped session with
an editable composer, ordinary-message reopening with the Goal still paused at two turns, and explicit
Resume starting turn three. HTTP snapshots and retained terminal sequences accompany the browser
actions. The first exploratory walk exposed typed cancellation events rendered as failures; the final
walk ran after the Web classification fix. Provider errors remain errors.

The real-app lab manifest is
`/Users/pedronauck/dev/qa-labs/compozy-goal-control-real-app-1015-20261029-031304-400987-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Its daemon binary is `d21438aeef78e7dd0e9546d33dc01dae70012383`; the Web includes the subsequent
unknown-Goal confirmation and cancellation fixes. This establishes scoped runtime behavior, not
exact-head CI evidence. The browser walk passed. The broader strict scenario audit remains failed
because its natural-startup rule rejects the fixture objective's literal “release readiness”; historical
prompts were retained. No natural startup or paid-provider release-readiness claim is made.
The canonical teardown completed with `clean: true` and no surviving owned processes.

The affected existing owner suites cover command parsing/transport, session admission and stop,
managed Goal execution, checkpoint persistence, and Web control interactions. Local validation is
targeted; no `make gate`, full race lane, or full local E2E was run. Current-head CI and the PR review
threads remain the delivery authority. The broader GL scenario verdicts remain untested where their
full release journey extends beyond this targeted walk.

## Focused validation

- `TestDaemonGoalControlHeldTurn`: all four public held-turn scenarios passed, including ordinary-message reopening and one explicit continuation.
- `TestDaemonE2EGoalCommandsShouldSurviveControlsDisconnectAndRestart`: all existing lifecycle, approval, removal, and restart scenarios passed.
- Existing session, daemon, store, transport, parser, and typed CLI owner suites passed. The shared stop suite retained its original deadlines; an initial resource-contention timeout passed on the serial package rerun.
- Root Turborepo Web tests and typecheck passed for the affected suites. The initial working-diff React Doctor scan missed a branch-level complexity warning caught by CI. After decomposing the primary control, the strict comparison against `main` scanned 18 files with no new issues.
- `make codegen` regenerated the wire artifacts. Incidental Open Design bundle drift from the local Bun version was excluded.
- The installed Go linter predates the repository Go version. The initial pinned type-aware attempt exceeded its time budget. CI identified an unused context parameter and a boolean simplification; after correction, the pinned `revive` and `staticcheck` checks passed for `internal/session` and `internal/daemon`. No full local gate was substituted.

## Review follow-up

Greptile identified Goal mutations exposed inside read-only transcripts. The header now receives
the existing `readOnly` boundary, omits every mutation callback, and guards command dispatch while
retaining state reads and Retry. The existing header suite exercises active, paused, approval, and
terminal Goals in that mode; the two focused Web suites passed all 20 tests.

The OpenAPI compatibility suite now explicitly admits the optional `pause_requested` and context
`diagnostic` fields and the `stop`/`stopped` vocabulary additions. All historical required fields,
types, and properties remain compared. Both canonical OpenAPI suites passed after this update.

CodeRabbit follow-up covers concurrent Stop/Clear (the winning Clear returns a valid hidden result),
same-turn usage after the provider terminal (tokens still max-flush without changing terminal identity),
and session Stop when Goal state is unknown (confirmation remains available with conditional copy).
The stopped response and CLI suites retain the snapshot audit fields. A deterministic store interleaving
also reproduced Pause winning before any prompt lease exists: preparation must reject the pending
pause, and executor recovery settles its control boundary without creating a queued successor.

Greptile's next review found a concurrent replacement race: Stop could cancel its original Run, read
the replacement, and incorrectly acknowledge that new Goal as stopped. Stop now compares the final
projection's RunID with its selected target and returns `goal_control_stale` (HTTP 409) if they differ.
The same race reproduced for Pause and Resume, so both use the same identity check. The existing
daemon lifecycle suite injects Replace after Stop's first read and before Pause/Resume's final read,
verifies the conflict payload, and proves the replacement remains running. Clear winning Stop still
returns cleared. The API outcome matrix explicitly verifies the 409 status without a success snapshot.
The complete canonical daemon lifecycle suite and HTTP outcome matrix passed after this correction.
The isolated public daemon lifecycle/restart scenario was then rerun and passed in 232 seconds.
The separate daemon-only fast lint reported zero findings but exceeded its two-minute time limit;
it is not counted as a completed lint gate. The preceding `40712f07c` CI Go format/lint check passed.

The second CodeRabbit review also found a stale session-stop confirmation across an external
stop/reopen cycle, unconditional Goal guidance when its presence is unknown, and cancellation
classification hiding an unrecognized provider diagnostic. Confirmation state now belongs to the
mounted primary action, the unknown-Goal note only explains reopening the session, and any attached
provider error preserves failure classification. The two existing Web owner suites passed all 29
tests; root Turborepo typecheck, changed-file lint, and React Doctor (22 files) passed. Turbo reported
an environment cache I/O warning without failing either task. The known-live Goal states in the six
real-app captures are unchanged.

CI exposed Resume reading `COALESCE` timestamp text through an RFC3339-only parser. The owning SQLC
query now returns the declared timestamp columns, letting the driver decode existing driver-written
timestamps and canonical text losslessly. The reentry owner suite preserves nanosecond precision,
prefers the actual pause-request time, and verifies concurrent Resume still creates one successor.

After the follow-up, focused session/daemon/API/CLI suites passed, followed by the store reentry and
preparation suites, executor recovery suite, and both isolated daemon Goal suites. The final runtime
batch passed in 159 seconds. Web verification passed 54 owner-suite tests, root Turbo typecheck,
changed-file lint, and React Doctor (22 changed files, no issues). SQLC was regenerated through its
owning generator. A transient intermediate build observed source before regeneration finished;
the completed generated tree was then rebuilt and verified successfully.
The final package-scoped Go lint attempt exceeded its five-minute bound and was interrupted;
edit-time checks and changed-file formatting passed. Current-head CI owns the remaining Go lint gate.

## Limits

The deterministic ACP fixture exercises actual daemon/provider transport and persistence. No paid
provider run was required. External effects a provider completed before cancellation are retained;
Stop does not roll them back. The control receipt fences future work immediately, while physical
termination follows the existing bounded session cancellation ladder.
