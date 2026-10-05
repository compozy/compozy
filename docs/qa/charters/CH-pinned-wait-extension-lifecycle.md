# CH-pinned-wait-extension-lifecycle: Resume an existing wait while its extension is unavailable

```yaml
charter:
  id: CH-pinned-wait-extension-lifecycle
  mission: "As Bruno, acknowledge a parked Studio handoff after disabling its later extension action in the Run's Profile, and verify the acknowledgement persists while a second Profile keeps working."
  mode: scenario-based
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-04
  scenarios: [LP-live-run-survives-extension-disable]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start a handoff Loop with an operator wait followed by spec-cycle's read-only task importer; capture its pinned Profile and executed definition."
      - "Disable spec-cycle only in that Profile, then resume the already-open wait through HTTP with a valid acknowledgement payload."
      - "Require the acknowledgement to persist in fresh CLI/UDS reads and Web after reload; a later unavailable extension action belongs to its node failure lifecycle, not an HTTP 500 on resume."
      - "Read the second Profile's extension inventory and invoke its importer successfully while the first Profile is disabled."
      - "Restore the owned Profile's extension and finish or explicitly cancel only this Run; verify the sibling Profile and all final extension states remain intact."
    must_avoid:
      - "Changing an active provider run, broad extension disablement, direct store inspection, or internal handler calls."
      - "Treating an accepted HTTP response alone as persisted wait completion."
```

<!-- Immutable charter: each run's debrief belongs in the dated report. -->
