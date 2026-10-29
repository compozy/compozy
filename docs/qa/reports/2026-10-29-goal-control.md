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

The affected existing owner suites cover command parsing/transport, session admission and stop,
managed Goal execution, checkpoint persistence, and Web control interactions. Local validation is
targeted; no `make gate`, full race lane, or full local E2E was run. Current-head CI and the PR review
threads remain the delivery authority. The broader GL scenario verdicts remain untested where their
full release journey extends beyond this targeted walk.

## Focused validation

- `TestDaemonGoalControlHeldTurn`: all four public held-turn scenarios passed, including ordinary-message reopening and one explicit continuation.
- `TestDaemonE2EGoalCommandsShouldSurviveControlsDisconnectAndRestart`: all existing lifecycle, approval, removal, and restart scenarios passed.
- Existing session, daemon, store, transport, parser, and typed CLI owner suites passed. The shared stop suite retained its original deadlines; an initial resource-contention timeout passed on the serial package rerun.
- Root Turborepo Web tests and typecheck passed for the affected suites. React Doctor scanned 16 changed files: 93/100, no issues.
- `make codegen` regenerated the wire artifacts. Incidental Open Design bundle drift from the local Bun version was excluded.
- The installed Go linter predates the repository Go version. The pinned type-aware attempt was stopped after exceeding its configured time budget; focused syntax lint and the current-head CI lane provide separate evidence. No full local gate was substituted.

## Limits

The deterministic ACP fixture exercises actual daemon/provider transport and persistence. No paid
provider run was required. External effects a provider completed before cancellation are retained;
Stop does not roll them back. The control receipt fences future work immediately, while physical
termination follows the existing bounded session cancellation ladder.
