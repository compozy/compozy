# BUG-20260906-deferred-schedule-drain: drain consumes deferred fires before execution

- **Status:** fixed — integration and public restart re-walk passed
- **Impact:** Data-Loss · **Severity:** Major · **Priority:** P1
- **Persona:** Bruno · **Scenario:** TA-schedule-catchup-overlap
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

With automation.max_concurrent_jobs=1, a running job occupies the gate. A one-shot
and a recurring schedule persist their original fire/run IDs as scheduled with a
retry cursor. During owned daemon restart, capacity release wakes those loops while
admission is draining and the model catalog is already shutting down. Session Create
fails before execution, but Dispatcher terminalizes the reserved run and Scheduler
clears its deferral. The original one-shot cannot resume; the recurring schedule
advances to a newer fire. These are not successful executions.

Integrated lab evidence: schedule-*-before-restart.json and schedule-*-runs-before-restart.json
retain original one-shot run_8466168f63acd17225c988b4/fire_8466168f63acd17225c988b4
and recurring run_70ed38bcda15447f4f96188f/fire_70ed38bcda15447f4f96188f.
After restart, the one-shot is failed with daemon-is-draining; the recurring original
is canceled with model-catalog context cancellation and a newer fire fails admission.
The holder used the real general/Claude provider; it was not the controlled driver.
The recurring job was disabled after inspecting this failure to keep the walk bounded.

Repair must retain an unstarted scheduled reservation on transient drain/shutdown
cancellation, preserve at-most-once execution identity, and avoid a hot retry loop.
Already started executions keep their existing terminal semantics. The canonical
TestSchedulerIntegrationDeferredFire suite owns real database reopen coverage.

## Repair and verification

Unstarted scheduled dispatches now retain their fire on admission draining or
creation cancellation/deadline. The scheduler atomically restores the reservation
with its retry cursor under the current schedule ownership guard; only capacity
rejections subscribe to capacity wakes. A context canceled before a new claim exits
without claiming. Started sessions retain terminal cancellation semantics and
unstarted deferrals emit no failed lifecycle hooks.

`TestSchedulerIntegrationDeferredFire` adds draining, cancellation, deadline and
recurring-drain reopen cases. RED: `.cache/sessions-qa-deferred-drain-red.log`;
GREEN with race: `.cache/sessions-qa-deferred-drain-final.log`. The broader owning
scheduler/dispatcher/globaldb selection passed in
`.cache/sessions-qa-deferred-drain-owner.log`. Scoped fast lint is clean in
`.cache/sessions-qa-deferred-drain-lint.log`. The test-shape checker identifies only
three untouched pre-existing top-level cases; the changed deferred-fire suite uses
parallel `Should` subtests. SQL changed through the sqlc generator; no schema change.

Public re-walk at 14:16–14:17 UTC used the controlled ACP provider, explicitly set
on the lab's general agent through `agent update`. With capacity1, one-shot
run_a8075bbe7a38adcb199f2adc and recurring run_c92696f85965e7f0ac31b77a were scheduled
behind running holder run_977fc9b8d8fbbb623b0b8da9. Owned daemon restart canceled the
started holder. Both original pending IDs then completed with attempt1, with no
replacement of either original identity. The recurring schedule subsequently ran
two distinct due fires normally and was disabled to bound the walk. Evidence:
`schedule-fixed-{once,recurring}-runs-{before,after}-restart.json`, matching job
snapshots, `schedule-fixed-holder-runs-after-restart.json`, and
`schedule-fixed-recurring-disabled.json` under the report's integrated lab.
