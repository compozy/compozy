# BUG-20261003-profile-recovery-blank-desktop: An unavailable profile leaves Settings blank with a generic retry notice

- **Status:** verified
- **Fix commit:** pending
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-operate-profiles, inspect a failed lifecycle operation from Settings
- **Scenarios:** ET-profile-operations-recovery
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

CH-profile-lifecycle-plan-recovery, isolated profile-recovery lab, b915570a8 plus initial
audit/identity fixes, desktop 1512x862, en-US, browser-use:

1. Remember recovery-drafts globally, then rename it with a conflicting imported destination.
2. The catalog now names recovery-published, with its failed operation reserving the owner.
3. Open /settings/profiles in a fresh browser tab.
4. The dock shows recovery-published, but Settings never opens. A blank desktop only says
   "Can't save window layout — retrying"; it does not identify the profile reservation or its remedy.
5. Correct the directory conflict and explicitly retry through CLI. Reload now opens Settings.

Expected: the client reports the unavailable owner and a useful recovery path; it must not turn
a durable lifecycle failure into an unexplained layout retry. The profile must remain reserved
until recovery completes. CLI/HTTP already provide profile_unavailable with the operation ID and
the instruction to inspect profile ops.

## Evidence

docs/qa/evidence/2026-10-02-untested/profile-recovery-fixed-ada-{web-entry,entry-divergence,
refresh-after,ended}.json; profile-recovery-entry-divergence.png and profile-recovery-after-retry.png.
The first entry waits twelve seconds for Settings and fails. The screenshot is inspected.
Recovery follows correction of the actual failed operation; no refresh loop or internal state
mutation is used. The browser recording is stopped before source investigation.

## Investigation

The window-manager adapter only parses its own flat error envelope. Profile scope resolution
returns the public nested profile refusal before that handler, so its code, message and recovery
action are lost to a schema error. Desktop projection keeps only the resulting degraded flag;
the menubar cannot distinguish an unavailable owner from a transient connection failure.

Preserve the typed profile refusal at the HTTP adapter and expose the current load error through
the existing desktop projection. The existing status surface can then explain that profile
recovery is needed and that the operator can choose another profile; its detail retains the
server's operation and CLI remedy. No automatic owner switch, guard bypass or synthetic window
is appropriate. After explicit retry, a fresh entry must open the original profile normally.

Invariant, owner, canonical suite: a cold Web entry under a reserved profile exposes actionable
recovery without changing its remembered owner, and can reach Settings through an explicit
available-profile switch. Profiles E2E-031 in web/e2e/__tests__/profiles.spec.ts owns this
cross-boundary behavior, using a real filesystem conflict and daemon. Existing OS fixture builders
will carry a null load error to satisfy the internal projection shape; no duplicate behavior
assertions or new test files are needed.

E2E-031 reproduces the failure before production changes: the cold entry retains recovery-guides
but the status remains "Can't save window layout — retrying" for the full assertion window.
Receipt: profile-recovery-blank-red.log, with its real-daemon trace and screenshot. No profile
or layout refusal is mocked. The test's independent operation read establishes the failed owner.

The first repaired run renders both the recovery label and the exact operation/remedy. Its
inspected screenshot and accessibility text confirm them, but the test incorrectly assumes a
tooltip role that this existing primitive does not expose for the status trigger. The canonical
OS-shell/primitive E2Es select data-slot=tooltip-content. E2E-031 now uses that same selector and
still requires visible detail, the original operation ID and the CLI action. The adjacent external
deletion and project-memory E2Es both pass. Receipt: profile-recovery-blank-green.log.

## First repair replay

The final E2E-031 passes in 4.7s. Root-Turbo lint, typecheck and production build pass;
React Doctor scans 58 changed files at 100/100 with no findings. The fresh Ada Chrome replay
retains recovery-handbook and names op_01M414TFWE5XZNYX8JGE8RABZD with its CLI remedy.
An independent UDS selection read proves the owner did not change implicitly. Choosing default
opens Settings and persists that explicit selection. The operation stays failed until Ada moves
the conflicting imported outline aside and retries. Selecting recovery-handbook then restores
its own empty desktop; opening Settings and reloading succeeds. Both authored files survive.

The first Chrome entry after the live lab rebuild reused the previous build's HTML and received
404 for its removed entry script. It ended before diagnosis. Current HTTP HTML matched disk;
restarting only the manifest-owned daemon aligned its static cache lifetime with the build.
That preparation failure is preserved separately, not counted as a successful profile entry.
A later driver check incorrectly expected the previous profile's Settings window after switching
to a previously empty desktop; the observed / route and normal launch controls establish the
correction to explicit Settings navigation. Neither case weakens the recovery invariant.

Evidence: profile-recovery-blank-final-{green.log,ada-*.json}, profile-recovery-blank-setup-diagnosis.json,
and the inspected profile-recovery-blank-{remedy,default-settings,final-settings}.png. The final
12-frame recording is stopped; no product source was read during either persona walk.
Required delivery gate and commit bookkeeping follow this verified replay.

## Delivery gate follow-up

The complete Web gate catches 40 failures in three existing window-manager/terminal suites.
Importing the broad Profiles barrel into the low-level window-manager adapter introduces a cycle
back through OS hooks/runtime, crossing the adapter boundary during module initialization.
Remove that dependency: normalize the nested public profile refusal into the adapter's existing
WindowManagerApiError, carrying its code, actual request workspace, and message/action diagnostic.
The status reads that same owned type. No assertion or mock is weakened; the existing failed
runtime/stream/terminal suites and final-source recovery replay must pass before verification.

## Final boundary repair verification

The adapter now owns the normalized WindowManagerApiError without a Profiles-barrel dependency.
All 106 affected runtime/stream/terminal/status tests pass unchanged. Root-Turbo production build,
lint and typecheck pass; React Doctor remains 100/100. Final-source E2E-031/030/013 pass against
the real daemon (3 tests, 26.9s). Receipts: profile-recovery-blank-boundary-{green,e2e}.log and
profile-recovery-blank-final-{web,doctor}.log.

A fresh Ada walk on this exact final implementation creates recovery-appendix, remembers it, and
reproduces a real rename conflict at recovery-contents. Cold Settings identifies the reserved
owner, op_01M416KR31W8GZQTD7A3DX20QH, and the public CLI remedy. UDS confirms no implicit
selection change. Explicitly selecting default restores Settings. Preserving the imported outline
and retrying completes the operation; the recovered profile's Settings and remembered selection
survive reload. Original and imported content remain readable. Both screenshots were inspected.
The 11-frame recording is stopped, and no product source was read during the walk.
Evidence: profile-recovery-boundary-ada-*.json, profile-recovery-boundary-{remedy,settings}.png.
The required full delivery gate is recorded separately before commit.
