# Gone resource requests — issue #1034

Scope: the Web request lifecycle for missing workspace roots, catalog subscription
lifetime, pending session deletion, and bound exited terminals. Heavy delivery
gates belong to CI; local validation uses focused Web checks, a narrow Go
error-mapping suite, and an isolated production-bundle browser walk.

## Journey matrix

| Journey | Expected observable | Status |
| --- | --- | --- |
| Missing project folder | Worktree reads stop after a terminal refusal and resume after catalog-confirmed recovery | Fixed |
| Open and close desktop windows | The worktree catalog connection survives unrelated window changes | Fixed |
| Delete a visible session | Outline, navigation, and presence stop during deletion and recover after failure | Fixed |
| Reopen a bound exited terminal | Retained output and exit state remain visible without an attach-ticket request | Fixed |

## Owning invariants

- Workspace query/stream suites own terminal-error suspension, catalog recovery,
  and workspace-qualified invalidation.
- The desktop shell model suite owns stream lifetime across window changes.
  No generic WebSocket cancellation behavior changes.
- Session navigation/action/window suites own pending-delete suspension and
  rollback. The presence suite retains invalid-lease recovery.
- The terminal window suite owns exited-state presentation and retained output;
  the protocol suite retains the terminal-exited race backstop.

## Preparation

An isolated targeted Web/API lab was provisioned. Its daemon runs release 0.4.5
with a dedicated home, HTTP port, and Unix socket. The daemon was then replaced with a binary built from this checkout to verify
the 410 correction. The Web bundle proxies exclusively to that daemon. No operator daemon was changed.

The first cold Vite navigation timed out. A clean launch from the Web package
and dependency warmup made the desktop reachable. This is infrastructure
evidence, not a passing journey.

## Observations and fixes

- Missing root: the installed 0.4.5 daemon returned two worktree-list 500s. Its
  worktree error mapper discarded the wrapped missing-root error. The corrected
  binary returns 410 with `workspace_root_missing`. With the folder absent, the
  browser makes zero worktree-list requests; restoring it produces a 200 after
  the next workspace catalog observation. The owning query test additionally
  exercises a received 410, stream invalidation, and recovery.
- Catalog: opening Home after switching projects closed the worktree WebSocket.
  The measured close occurred in OPEN, not CONNECTING; the precise browser
  warning timing was not reproduced. The cause was the HTTP connection budget
  tied to visible windows. After removing that obsolete constraint for this
  WebSocket, one worktree subscription survives the same workspace/window walk.
  Generic transport cancellation and the session catalog's separate policy remain
  unchanged.
- Deletion: the baseline produced two outline 404s plus detail/presence 404s
  while the real DELETE response was held. The first repaired walk stopped
  outline/presence and recovered after an aborted DELETE, but exposed an extra
  shared detail observer in the desktop chrome. Its owning hook now observes
  deletion state too. The final re-walk has zero new session requests in both pending intervals;
  outline/presence resume after refusal and an independent read confirms 404
  after the successful deletion.
- Exited terminal: baseline attach-ticket returned 409. The repaired bound window
  reads retained output with 200, issues no attach-ticket, displays the emitted
  line and exit code 7. A pane that already streamed keeps its existing buffer.

## Evidence and verification limits

The [redacted network capture summary](./2026-10-29-gone-resource-requests.network.json)
contains before/after status counts and the pending-deletion intervals.

Raw captures are retained in the isolated lab: `before-workspace-catalog.json`,
`before-catalog-project-churn.json`, `before-deletion.json`, `before-terminal.json`,
`after-workspace.json`, `after-terminal-catalog.json`, `after-terminal.png`, and
`after-deletion.json`. Reproduction uses isolated Chromium instances, never a
shared browser tab. No provider turn, operator daemon, or shared runtime was used.

The development server initially timed out and later served stale transformed
modules. Those attempts are not correction evidence. Final browser validation
uses the successful production bundle. The build reported plugin timing and
large-chunk advisories; they are not changed-behavior failures.

Focused suites cover the canonical worktree stream/query, navigation rail,
presence, session mutation, shared session query, OS session window, desktop shell,
and terminal window boundaries. Typecheck and lint passed through Turborepo;
the missing SDK build output was prepared through its owning Turbo task first.
The narrow Go error suite passed with race detection. The Go shape heuristic
reported a pre-existing unrelated `TestCreateSessionWorktreeBinding` shape;
the changed test is a named subtest in `TestWorktreeErrorCodes`.

## Final status

PASS for the four targeted journeys and the running-terminal canary. The canary
minted one attach ticket (201), displayed live and final output, and kept that
buffer on exit without a retained-output reread. Focused Web checks total 222
passing tests across eleven existing suites; typecheck/lint passed and React Doctor
reported 100/100 with no issues. No full local E2E or heavy gate was run. The targeted evidence audit passed. Manifest-based teardown confirmed all lab
processes were stopped.

PR delivery remains in progress until current-head CI and both requested
reviewers complete.

## Review regression: catalog exit before final stream bytes

Greptile identified an independent-channel race: a catalog exit could disable
an already-live attachment before its final output arrived. The attachment now
keeps its initial live eligibility until its own stream closes. An initially
exited window still reads retained output without attaching. Retained reads do
not compete with a live attachment draining its first output.

The canonical terminal window suite reproduces the race with and without earlier
output: both variants failed before the fix and pass afterward (86 tests across
the terminal window and protocol suites). Final typecheck/lint passed; React
Doctor found no issues (its remote score API was unavailable).

A second isolated production-bundle lab delayed actual final WebSocket frames
by 2,500 ms, leaving real catalog events untouched. Catalog exit became visible
2,100 ms before frame release; the client closed 27 ms after release. Both output
lines remained visible, with exit code 0. The independent retained-output read
returned those same lines. The interception forwards real daemon frames; it
does not fabricate output, exit events, or API responses.

The remediation lab passed its strict evidence audit and was torn down using
its process-identity manifest.

## CodeRabbit review remediation

Presence cleanup now reads deletion state for its captured session rather than
the hook's latest binding. The same live check prevents acquiring a lease during
a binding change before the mutation observer catches up. The original-session
release regression failed before this fix. The exact binding-switch interleaving
is unit-tested; the real browser re-walk exercises deletion and rollback.

Reconnect on an exited streamed pane no longer starts an unused retained read.
Canonical suites also cover pending transcript search, successful detail deletion,
both real worktree hooks deriving recovery from catalog data, and retained reads
using the correct terminal and tail view. The review's request to close the socket
on catalog exit is rejected: doing so reproduces the confirmed final-byte loss.

The five affected suites pass 97 tests. Typecheck/lint passed; React Doctor found
no issues (remote score unavailable). A fresh isolated production-bundle walk
again had zero new session requests during both pending-delete intervals and an
independent 404 after deletion. The terminal drain canary showed catalog exit
2,336 ms before final frame release, client closure 17 ms after release, and both
lines visible. Initial lab setup attempts without the local agent fixture are
not passing journey evidence.

The review-remediation lab passed its strict evidence audit; manifest teardown
confirmed no surviving lab processes.
