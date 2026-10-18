# Update discovery — issue #804

- Scope: targeted update discovery and Settings interaction on the issue branch.
- Environment: isolated home and loopback ports, real branch daemon and Vite web server, live GitHub beta metadata. No packaged app build or installer execution.
- Persona: Bruno, desktop, en-US, keeping the installed version current.
- Charter: bounded discovery subset of `CH-electron-browser-update-parity`; desktop installer journeys remain CI-owned.
- Status: closed for the scoped discovery walk; PR delivery remains in progress.

| Journey | Status | Evidence |
| --- | --- | --- |
| Read discovered release and last successful check through HTTP and UDS | Pass | Live beta.34 daemon discovered beta.35; checked_at advanced through the manual route and matched UDS reads. |
| Open Settings, check manually, reload and preserve checked-at truth | Pass | Checked-at advanced to 2026-10-18T14:42:28.765226Z after the UI action and survived reload. |
| Compare narrow viewport and update offer | Pass | 1440×900 and 320×800 captures; narrow row width and scroll width both 272px. |
| Packaged macOS installer handoff | Skipped | Desktop builds/full E2E excluded by operator instruction; focused policy/consumer regressions and PR CI own coverage. |

## Focused verification

Go manager/projection, daemon background/controller, API handlers, config lifecycle and OpenAPI tests passed with race instrumentation. Web's three owning files passed 89 tests; desktop consumer/policy/state passed 29 tests. Web and desktop typechecks passed via root Turbo. React Doctor returned 100/100 without findings.

The test-convention heuristic passed the changed update/daemon suites. Its eight findings in `internal/api/core/settings_test.go` concern pre-existing unrelated functions; the new check-handler case uses a parallel `Should ...` subtest.

## Limitations

No host daemon/home changes, no desktop build, no broad local gate or E2E suite. Required CI and completed automated reviews remain necessary before PR readiness.


## Session debrief

Bruno completed normal onboarding using only the isolated project folder, opened General settings,
and saw the beta.35 offer in both the menubar and Settings while the same beta.34 daemon stayed
running. **Check for updates** showed a pending state, then advanced **Last checked**. Reload and an
independent UDS read preserved that timestamp. Two subsequent HTTP checks inside one minute shared
the same successful timestamp. No installer was invoked and no update operation was acquired.

The narrow viewport initially exposed a clipped existing update control. The product row now stacks
its label and wraps its controls inside the existing Settings components. The 320px re-walk showed
both check and update actions without overflow; the 1440px view retained its normal horizontal row.

Captures: `../evidence/2026-10-18-issue-804-update-discovery/settings-desktop.png` and
`../evidence/2026-10-18-issue-804-update-discovery/settings-narrow.png` (QA evidence sidecar).

The first CUA browser connection timed out; the isolated agent-browser session successfully completed
the walk. Its initial navigation command waited on live page activity, but the subsequent accessible
snapshot showed the normal onboarding screen; this was a driver wait, not a product stall.


## Scoped outcome

PASS for the discovery walk above. All three observed rows passed; packaged installer execution was
skipped under the explicit operator restriction. Focused Go lint completed with zero issues. No broad
local gate ran: CI is the official delivery gate by operator override. The lab strict auditor's C14
full-local-gate requirement is superseded by that override; its missing-report checks were remedied,
and no full-gate success is claimed. Lab teardown reported `TEARDOWN_CLEAN=true` with no survivors.
