# BUG-20261004-profile-palette-navigation-stall: Opening profile creation can leave the desktop unresponsive

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, submit a profile lifecycle command in an attached desktop
- **Scenarios:** ET-profile-remote-write-boundary; ET-profile-palette-view
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-lifecycle-plan-recovery, Interrupt Tour, isolated production Web at port 50727,
desktop Chrome 1512×862, en-US. Several existing task-owned tabs are attached to Studio Operations.
Choose Create profile in the palette, enter dispatch-palette, and submit. On the first attempt,
the active document stops answering accessibility and Runtime.evaluate requests. The browser
connection and root Target requests remain healthy. A second affected tab shows the consumed
name query while the originating tab retains flow=create&name=dispatch-palette.

Expected: the canonical dialog opens and the desktop remains responsive.
Actual: the first attempt cannot be observed or operated. Two owned affected tabs are closed after
capturing the failure; a fresh tab opens normally. One clean retry opens the dialog but loses the
name, recorded separately as BUG-20261004-profile-palette-drops-arguments.

## Evidence

Cycle receipts: profile-local-palette-argument-observed.json,
profile-local-palette-argument-final-state.json, profile-local-palette-browser-diagnostic.json,
profile-local-palette-hang-root-targets.json, profile-local-palette-hang-close-owned-tab.json,
and profile-local-palette-hang-recover-tab.json. Recording profile-local-palette-argument-ada
stops at four frames. Its last frame precedes the timeout and is not proof of an opened dialog.
Independent UDS still returns profile_not_found; no profile mutation occurred.

## Investigation

The first stalled document and a second renderer consumed CPU while the real daemon stayed
reachable. Resource contention was initially a competing hypothesis. A controlled Bruno replay
on the corrected argument/result build closes the completed QA tabs, starts exactly two current
clients on resume-editorial, and repeats the palette creation. The originating renderer times
out again. Independent UDS reads show layout revision 31 advancing to 33 with the intent already
consumed, while the origin retains flow=create&name=dispatch-two-clients. No profile is created.
Receipts: profile-navigation-controlled-*.json; the recording is stopped at nine frames and both
owned clients are released after the failed session. Source work starts after this session ends.

The existing E2E-027 reproduces the same two-client failure against its own daemon. One page stops
answering, including screenshot capture during teardown. Runtime admission accepts a new command
while its command state already records a revision conflict. Navigation then publishes an
optimistic route which the queued command immediately refuses. Consuming the flow repeatedly
publishes and rolls back that route before the recovery read can settle.

Rejecting that command at admission stops the renderer loop, but exposes a second race: userOpen
pushes its original target route after the canonical dialog has already consumed the intent.
The delayed history write recreates the old flow and queues another reconciliation. Bounded
diagnostic logs in profile-navigation-two-clients-e2e-diagnostic-artifacts show the original open,
both cleanup requests, the obsolete route match, the conflict and the recovered clean route.
Those temporary diagnostics are removed. Adding a Query notification alone did not repair this
race and is not retained.

## Fix

- **Fix commit:** 259d7142c

The runtime refuses commands at admission during a known conflict, without publishing optimistic
route state or replacing the conflict diagnostic. After an accepted open, RoutingCoordinator
uses the window's current route before falling back to the original requested route. Existing
queued-command sequencing, conflict recovery and newer-user-intent fences remain in place.
No timer, diagnostic suppression, transport retry or public contract change is introduced.

## Verification

The existing runtime admission case fails before its repair; the owning routing suite's consumed
route case separately fails before its repair. The two suites now pass 92 tests. The first admission
test edit was placed in the wrong neighboring fixture; its receipt is retained, and only
profile-navigation-admission-unit-red-corrected.json is causal admission evidence.

All three production E2E-027 repetitions complete the navigation, cancellation, clean-URL and
rename/reload legs. Each fails later on a separate stale-client inventory defect, tracked as
BUG-20261004-palette-lists-closed-clients; the full E2E is not reported as green.

Fresh Bruno replay on Web index 9b5e90b5aca523c0a22cfeeb3173fbc339a1bc21d80345ff99baab2fc89a05e0
confirms both clients can open and cancel the supplied form, reload without reopening it, and
remain responsive. Independent UDS reads prove both abandoned names absent. The adjacent Tasks
and browser Back leg passes. Evidence: profile-navigation-fixed-*.json and the inspected
profile-navigation-fixed-create.png. The 25-frame recording is stopped before source work.
The corrected replay explicitly dismisses the Command palette returned after Cancel; the first
wait for the inert main button was a driver assumption. Production repair: 259d7142c. The gate passed before commit; a hook-only formatter disagreement in the integration fixture is being normalized separately.
