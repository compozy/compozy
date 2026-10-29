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
buffer on exit without a retained-output reread. Focused Web checks total 211
passing tests across ten existing suites; typecheck/lint passed and React Doctor
reported 100/100 with no issues. No full local E2E or heavy gate was run. The targeted evidence audit passed. Manifest-based teardown confirmed all lab
processes were stopped.

PR delivery remains in progress until current-head CI and both requested
reviewers complete.
