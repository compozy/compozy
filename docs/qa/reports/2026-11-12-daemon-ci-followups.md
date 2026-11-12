# Daemon and CI follow-up investigation

Refs #1547. Source inspection and retained CI logs only; no local test, lint,
build, daemon or QA execution is claimed. CI on the PR head owns validation.

## Main integration after the boundary diagnostic round

The branch rebased onto `a8af28f80`, retaining the nineteen scoped commits.
Main's #1594 now owns the same standing-augmenter parity correction and a full
twelve-peer overflow fixture. Those accepted implementations and their scenario
notes are preserved rather than duplicated; historical branch evidence below
still describes the heads on which it was collected. The current fixture uses
alternating original peer descriptions with unique names. All budget, envelope
and overflow assertions remain unchanged. Terminal journey resolutions retain
the exact pre-rebase branch source, including command-ID journal correlation,
verified exit and tab-remount selection, alongside main's shared navigation
helper improvements. Both heartbeat scenario additions are retained. Main also
includes the separate #1592 SQLite acceptance repair; only new-head CI can
establish its result. No local test, lint, build or runtime execution occurred.

## Boundary steering drain timeout

[Full integration shard 5, job 131045669959](https://github.com/compozy/compozy/actions/runs/43177532728/job/131045669959)
on `188533440` failed two existing `TestManagerBoundarySteer` cases at
`WaitForPromptDrains`: transient boundary queue failure and delayed native refusal
followed by a timeout. The active stream had drained, but a tracked task remained.
The log contains only a context deadline, so the task and cause are unconfirmed.
This package invocation failed before the daemon invocation; it does not verify
the preceding standing-context overflow correction.

The owning suite now captures a bounded 1 MiB stack dump with manager, session,
turn and truncation identity on that error. A separate CI-only race job repeats
the existing suite twenty times. The invariant remains correct boundary steering
and complete owned task drainage under the original five-second context. All
behavioral assertions, deadlines and cleanup remain unchanged. This is diagnostic
instrumentation pending attributable evidence, not a claimed production fix.

## Standing-context overflow fixture after #1572 integration

[Full integration shard 5, job 131023234338](https://github.com/compozy/compozy/actions/runs/43170690094/job/131023234338)
on `8d7e32afb` failed the existing bounded-roster case from #1572. Its fixture
contains sixty long rules but only two peers and `more: false`. Static budget
inspection confirms the remaining space fits both peers: the renderer correctly
has no reason to emit the peer-overflow pointer. The assertion nevertheless
requires that pointer. This is a fixture/contract mismatch, not evidence of
dropped peer information in production.

The owning `TestPromptInputCompositeIntegrationStandingContext` case now supplies
a full roster of twelve described peers for the bounded rendering branch. It
must actually omit peers as well as rules. The budget, envelope and both overflow
pointer assertions remain unchanged; the small-roster/input-form case is intact.
No renderer behavior, public contract or timeout changes. CI owns execution.

## Historical failures

- Late boot reconciliation: job 130360806209 timed out at five seconds before
  entering observer reconciliation. The current suite already reports early boot
  errors and uses its bounded context. Capture bounded goroutine stacks on its
  remaining deadline path; do not infer a production defect from the old sample.
- Loop neutralization: job 130435223440 timed out after releasing the barrier.
  Current source already captures goroutine stacks on that path. The original
  log predates those diagnostics. Preserve the five-second completion assertion.
- Integration shutdown: job 129540374905 failed stopping an active session at the
  two-second phase budget. Capture elapsed time, phase and bounded goroutine stacks
  on deadline errors, preserving the error chain and every teardown phase.
- Usage cooldown: job 130931103759 observed 00:20 instead of 00:21 without a race
  report. Preserve the assertion and log the target, clock and call count on a
  mismatch. Linux/race repetitions must distinguish a reproducible defect from
  an isolated sample. Earlier non-race passes do not establish a cause.

The boot changes are diagnostic improvements, not claimed causal fixes. The
historical boot items remain unchecked until CI evidence supports a disposition.

### Reproduced CLI shutdown timeout

[Full integration shard 4, job 131010754677](https://github.com/compozy/compozy/actions/runs/43166808033/job/131010754677)
on `e2c0b2b4a` reproduces the two-second active-session shutdown timeout during
`TestSessionCompactIntegration/completed` cleanup. The compact result assertions
passed. The retained 64 KiB stack dump truncates before the responsible stop
worker; a captured stop-observation worker belongs to a different manager and a
real process, whereas this fixture owns synthetic process IDs. That stack is not
evidence of this session's cause.

The existing CLI integration harness now captures up to 1 MiB, explicitly reports
capacity exhaustion and goroutine count, and identifies the target manager,
ACP session, state, runtime and permission status in the phase error. The owning
invariant remains complete, verified shutdown within the original two-second
phase budget; neither the deadline nor error handling nor compact assertions
change. A separate CI-only race job repeats the existing compact integration
suite twenty times to collect attributable evidence. The production cause remains
unconfirmed; this diagnostic round does not claim to fix the timeout.

## Usage cooldown root cause

[Linux/race job 130976658467](https://github.com/compozy/compozy/actions/runs/43156218682/job/130976658467)
on `96da0511b` reproduced the original floor-after-success mismatch: expected
00:21, observed 00:20. The temporary fixture instrumentation captured
`net/http: HTTP/1.x transport connection broken: http: CloseIdleConnections called`
with no request-context cancellation. The collection recorded `network_error`,
so the service correctly retained the previous successful snapshot's 00:20
refresh time instead of receiving the new HTTP 429 and calculating 00:21.

The fixture used `http.DefaultTransport`, shared with parallel HTTP tests.
Go's `httptest.Server.Close` closes idle connections on that global transport,
allowing another fixture's teardown to interrupt a newly reused connection.
The owning `TestUsageRateLimitBackoff` suite must preserve exact cooldown/reset
behavior without unrelated fixture teardown changing its HTTP response.

`usageFixture` now supplies `server.Client().Transport`, which is owned and
cleaned up by that server. Existing cleanup ordering shuts down the service
before the server. Real loopback HTTP, parallelism, fake-clock expectations,
call counts and the Linux/race stress workload are unchanged. Temporary raw
transport logging is removed after capturing the causal evidence; status and
timestamp diagnostics remain. No production retry/backoff behavior changes.
Current-head CI must verify the repair; no local execution is claimed.

## Provider recovery evidence follow-ups

PR #1560 merged as `84874c0f5` and is included after the main rebase.
Its independent review, comment 6812346714, establishes the following provenance:

| Walk | Binary SHA-256 | Source attribution |
| --- | --- | --- |
| Corrective finalization | `857221bfc7fdf4e17e1d9ddfde00e4ac4086e51416f8380bd48ab665099d5dfd` | After B4 repair, before final cancellation change; previous VCS commit, `vcs.modified=true` |
| Cancellation | `8f6bd49663b21d996845e93513df8f7320f8b0c50f9ecb8ae2220c39d3f5496a` | Final production tree later committed as `76d08f05f7b05f5d5ea5b52f52531a82caa14542`; previous VCS commit, `vcs.modified=true` |

These are retained historical receipts, not clean-head executions. The corrective
receipt now records the per-walk binary hashes, dirty VCS attribution and source
ordering; the unknown exact previous VCS revision is explicit. Scenario frontmatter
links the #1557 receipt and fix commits while preserving older evidence and its
partial/pending status. No new local walk is claimed.

The existing full-daemon shutdown integration suite now includes a real ACP
subprocess authentication failure, observes recovery backoff, requests the normal
restart transition, completes `Daemon.Shutdown`, and reopens durable session
history to check resumable intent, absence of a skip and shutdown metadata.
CI must execute this case before D1 receives a verified disposition. The earlier
temporary-overlay observation is still not attributed as a regression.

## Product decision

Automation `completed` currently records execution lifecycle, not an agent's
semantic success. A new agent-reported outcome needs a specified authenticated
reporting surface, durable outcome model, and scheduler/CLI/Web projection.
Inferring failure from prose or changing existing lifecycle meanings would be
unsafe and incompatible. Leave that item unchecked pending a product contract;
the existing intake-failed/watchdog mitigation remains separate.

## Terminal browser CI reconciliation

[Web shard 6 job 130980892165](https://github.com/compozy/compozy/actions/runs/43157453465/job/130980892165)
on `095596a36` failed E2E-003 and E2E-008. Its retained browser trace,
network envelopes, accessibility contexts and rendered screenshots establish:

- E2E-003 successfully opens the exact terminal. Immediately afterward its
  two-second command exits, and the authoritative `terminal.retire` event removes
  that window and returns focus to the session. This matches the accepted #1570
  scenario; waiting for an exit bar in that retired window contradicts it.
- E2E-008 successfully sends the sourced quote and closes the conversation tab.
  The terminal is visible again but has no selected range after remounting.
  Selection actions require a current nonempty emulator selection.

The existing hosted-native-tools/browser suite owns approval-gated execution,
exact-terminal discovery, successful exit, retained output and journal provenance,
plus the sourced quote and no-conversation selection fallback. E2E-003 now uses
a stdin barrier instead of a fixed sleep: observe the live pane/output, release
it through the native write tool, check native wait's exit reason/code and both
output markers, verify the window retires, then reopen Terminal to view the same
approved journal row. E2E-008 selects the current output again after the tab
remount before checking Choose and Copy. No production behavior, timeout or valid
approval/quote/journal assertion is relaxed. CI must run the corrected journeys;
no local browser, daemon, test, build or lint execution is claimed.

## Integration boot publication race and recovery fixture diagnostics

[Full Integration Suite (6), job 130980691091](https://github.com/compozy/compozy/actions/runs/43157453599/job/130980691091)
on `095596a36` reports the same boot race already tracked in #1588: the MCP
declaration watcher reads `runtimeWorkers.proactiveChecks` while
`bootProactiveChecks` publishes it. Its stacks affect unattended-instance and
orphan-report boot scenarios. A boot-state RWMutex now protects publication and
all boot-state readers; publication occurs after service recovery and callback
binding. The existing boot-state synchronization suite exercises concurrent
publication/readers. The watcher lifetime, declaration refresh, configuration
recovery and cleanup retain their owners. This internal fix changes no public
contract, persisted data or configuration. CI must run its race regression and
the actual integration paths. No local execution is claimed.

That job also ended the shutdown-recovery prompt before its expected backoff;
the original assertion captured no terminal events. The subprocess fixture now
receives an explicit auth-required test mode rather than matching rendered prompt
prose. Unexpected early completion retains its synthetic event/status diagnostics.
This isolates fault injection from prompt formatting while preserving the required
observed auth backoff, full daemon shutdown and durable restart-intent assertions.
The original failure's exact terminal event is unavailable; no production recovery
regression is inferred from that missing observation. CI must establish the actual
recovery path before this item is accepted.

CodeRabbit review 5869566379 identified the recovery fixture's remaining invalid
precondition: its auth-free provider mode maps an auth-required diagnostic to
`Inspect` in `AgentProcess.providerAuthRecovery`, whereas automatic sign-in
recovery requires `Login`. The recovering case now configures `native_cli` in
both the in-memory config and persisted fixture TOML. The provider validator
accepts that mode without credential slots or an ACP auth-method requirement;
`bootConfig` performs the real config validation before the tested prompt. Other
fixture users retain `none` and its local-transport rationale. The subprocess and
HOME remain test-owned; no real provider login is involved. Full backoff, shutdown
and durable-intent assertions are unchanged and await the new CI head.

The subsequent [Web shard 6 job 130995361828](https://github.com/compozy/compozy/actions/runs/43161958417/job/130995361828)
on `0245f05a0` passes E2E-008 and reaches E2E-003's journal assertion after
observing live output, successful exit and window retirement. Its remaining
failure selects a row with approval `none`: the stdin-release write is itself
a promptless journal command on the same terminal. The execution tool's public
result requires a `command_id`; the owning E2E now selects that exact command's
row and still requires both `approved_once` and the expected terminal ID. It
does not select by the expected approval value or weaken the approval invariant.
Current-head CI must confirm the repaired correlation and remaining UI walk.

## Desktop first-run state record (#1595)

The later #1547 comment 6813880087 retains [Desktop first-run job 130953022505](https://github.com/compozy/compozy/actions/runs/43148743622/job/130953022505):
the real CLI rejects a boot snapshot missing `attempt`. Source confirms the
`starting/ready` projection omits it and the app-state writer persisted it as-is,
although the shared schema requires it throughout `starting`. Product publication
is asynchronous after renderer load, exposing this transient record.

The owning writer now supplies the first attempt and retains an explicit reported
attempt across readiness. Leaving `starting` clears that carry-over. The existing
publisher suite reads real temporary files across start, ready and fresh-start
transitions. The existing Electron first-run journey waits for asynchronous
product-state publication through the same strict CLI, preserving its full
installed/running/product/attached/owned assertions. The schema and validation are
unchanged; no new field, version or migration is introduced. CI owns all execution.

A contradictory existing upper bound remains explicit: CLI
`bootstrapMaximumAttempts` is 5 and the Desktop preload accepts up to 5, while
`desktop/schema/app-state.schema.json` caps attempts at 3. Resolving that public
contract requires its own compatibility decision. This missing-field fix preserves
actual explicit counts and does not clamp them or relax the schema.

## Main standing-context integration

On `9dfde45ad`, [runtime E2E job 131001094715](https://github.com/compozy/compozy/actions/runs/43163765451/job/131001094715)
and [full integration shard 7 job 131000918609](https://github.com/compozy/compozy/actions/runs/43163765340/job/131000918609)
fail the same HTTP/UDS harness-observation parity test. The retained events include
the legitimate third `standing` augmenter introduced by merged #1572, but the
stale oracle expects only `skills` and `situation`, so its exact five-event match
can never settle after all six events arrive.

After rebasing onto `e226c9ccf`, the owning transport suite now expects the exact
six-event sequence and explicitly requires metadata for all three augmenters.
HTTP/UDS deep equality and every deadline remain intact. No production event,
filter or validation policy changes. The existing full patch series replayed
without conflicts and range-diff preserves all prior commits. CI must replay
both the integration and runtime lanes on this integrated head.

The [SQLite acceptance lane on the previous head](https://github.com/compozy/compozy/actions/runs/43163765328/job/131000842035)
remains excluded under #1433: no invalid head trial and no throughput regression
were recorded, but baseline transcript-tail pair 0 is incomplete, so acceptance
correctly fails rather than averaging away that missing pair. No SQLite harness
or threshold is changed.


## Main integration after #1576

Rebased onto `c80892f71`. Main now forwards the boot attempt explicitly through
`bootstrapSnapshot`, including readiness; this incorporates the earlier R2-N1
follow-up while the app-state writer still guarantees the existing required field.
Main's result-view/catalog/reply-watch changes remain intact. The terminal journal
read now retains main's eventual-publication poll while selecting the exact exec
`command_id` and asserting both approval and terminal ownership. The approved
branch's generic Terminal journal open, verified exit and output assertions are
preserved. Main's visible-terminal check before re-selection is retained. No
assertion or timeout is weakened. Current-head CI/review must validate integration;
prior-head passes remain historical evidence.
