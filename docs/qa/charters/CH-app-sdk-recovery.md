# CH-app-sdk-recovery: Recover readers without stopping agent work

```yaml
charter:
  id: CH-app-sdk-recovery
  mission: As Ana, interrupt streams and close app clients while proving that transcript recovery and private-daemon ownership remain correct.
  mode: charter-with-tour
  persona:
    name: Ana
    device: desktop
    network: flaky
    locale: en-US
  journey: J-build-external-app
  scenarios: [RT-sdk-stream-recovery, RT-sdk-node-lifecycle]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - Disconnect a prompt reader mid-turn, reattach through events after the last consumed sequence, then read the completed transcript.
      - Exercise queued admission and explicit cancellation separately from reader abort.
      - Start with no daemon, attach to an existing daemon, and replace an owned daemon before closing; observe which process remains alive.
      - Keep private home paths and process ownership receipts in the lab evidence.
    must_avoid:
      - Do not kill or reuse any daemon outside the assigned lab.
      - Do not restart a prompt merely because its reader disconnected.
```
