# CH-loop-result-budget-paging: Keep large Loop results durable and bounded

```yaml
charter:
  id: CH-loop-result-budget-paging
  mission: "As Ada, produce a large Loop action result and prove every structured reader returns the same durable bytes while the configured budget still stops runaway output cleanly."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-oversized-action-result-fails, TA-task-run-result-paging]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Return results immediately below and above 16 KiB, 64 KiB, and the effective action-result budget."
      - "Concatenate ordered CLI, HTTP, UDS, Host API, and native-tool pages around a multibyte boundary and compare exact bytes."
      - "Restart before reading again; probe invalid ranges and a foreign workspace; confirm the above-budget lease is released."
    must_avoid:
      - "Using direct database reads as the verdict."
      - "Treating a decoded partial UTF-8 page as the complete result."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
