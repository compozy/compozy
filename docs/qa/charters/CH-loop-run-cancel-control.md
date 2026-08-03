# CH-loop-run-cancel-control: Cancel a live Loop from its run page

```yaml
charter:
  id: CH-loop-run-cancel-control
  mission: "As Bruno, cancel a live Loop from its run page and prove the page, API, and refreshed state all tell the same story."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-04
  scenarios: [TA-084]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open a real live run from the Runs surface and confirm Cancel is present while Stop is absent."
      - "Cancel once, observe immediate feedback, then refresh and compare the public run detail with the API response."
      - "Use browser back and reopen the deep link; the canceled state must remain truthful and readable."
    must_avoid:
      - "Using Storybook or mocked handlers as behavioral proof; those are visual evidence only."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
