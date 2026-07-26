# CH-schedule-recovery-guard: Downtime recovers once and no job can kill its daemon

```yaml
charter:
  id: CH-schedule-recovery-guard
  mission: "As Bruno, take the daemon down across schedule boundaries and prove run_once_on_catchup fires exactly once, skips are explained in durable history, overlap never happens, and any daemon-lifecycle command is rejected at creation."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-24
  scenarios: [TA-schedule-catchup-overlap, TA-daemon-lifecycle-command-guard, TA-055]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Downtime shorter than grace under run_once_on_catchup → exactly one synthetic run, cursor advanced once; downtime beyond grace under skip_missed → grace-aware skip reason in history."
      - "A job whose prior fire is still running at the next due time → self_overlap skip recorded, next cycle normal."
      - "Create jobs containing 'agh daemon restart', 'pkill -f agh', and service-manager variants via CLI and agh__automation_jobs_create → deterministic blocked-class errors, nothing persisted; a prose mention in a non-command field must be accepted."
      - "Authoring surfaces expose the full catch-up policy set with non-negative grace; at-time schedules reject catch-up fields (TA-055)."
    must_avoid:
      - "Editing scheduler state directly; downtime is real daemon stop/start with timestamps."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
