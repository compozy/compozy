# Development startup performance QA — 2026-09-05

Targeted walk of `RT-dev-bootstrap-ready`, using Ada and the existing `CH-untested-066-operate-daemon-schema-ada` charter. The other charter scenarios are unchanged and outside this performance pass. Tour: Garbage Tour; scope: startup, first live UI requests, refresh, and graceful supervisor shutdown.

| Scenario | Status | Evidence |
| --- | --- | --- |
| RT-dev-bootstrap-ready | Pass | Current-run readiness, browser refresh, CLI/direct/proxy status, and failed-build recovery captured. |

## Session debrief

The baseline `make dev` reached Vite after the current daemon reported readiness. Initial tool installation and compilation are recorded separately from the repeated config-query benchmark. The lab uses a private runtime home, HTTP port, socket, and local catalog. No live provider journey is in scope.

With generation evidence current, the optimized supervisor reached daemon readiness at 4.640 s and Vite at 5.488 s. Onboarding rendered and survived refresh. Browser status, workspace, task, and catalog/command stream requests succeeded; CLI status, direct HTTP, and the Vite proxy agreed on the isolated daemon. A deliberately invalid Go source edit failed compilation while the API kept returning HTTP 200. Removing the edit completed a successful rebuild and retained the unchanged daemon.

The browser navigation command timed out waiting for completion while the app was already usable; subsequent accessibility snapshots and public requests confirmed the rendered state. No page errors were recorded. This timing is Vite readiness, not a cold-browser page-load measurement.

The abandonment branch stopped the supervisor and its daemon. No production credentials or provider-backed work were exercised.

## Evidence

- Lab: `/Users/pedronauck/dev/qa-labs/compozy-dev-performance-20260905-173738-372390-lab`.
- Startup and benchmark logs: `.compozy/tasks/tests-cleanup/performance/dev-*`.
- Final affected gate: `.compozy/tasks/tests-cleanup/performance/final-gate-3.log`.
- Runtime E2E: `.compozy/tasks/tests-cleanup/performance/final-e2e-runtime.log` (262 passing tests).
- Lab teardown: `qa-artifacts/qa/teardown.json` under the lab above, with `clean: true` and no survivors.

## Final status

PASS. The targeted behavioral walk, affected gate, and runtime E2E passed. The
development lab and runtime envelope both completed clean teardown. Provider-backed
journeys and the full Playwright suite were outside this targeted validation.
