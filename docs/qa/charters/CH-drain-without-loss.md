# CH-drain-without-loss: Quiesce the daemon without losing a single admitted unit of work

```yaml
charter:
  id: CH-drain-without-loss
  mission: "As Dora, drain the daemon while a prompt and a claimed run are in flight, prove new admission refuses deterministically on every transport while admitted work finishes, then undrain, restart, and read truthful doctor/memory evidence throughout."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-drain-daemon-safely
  scenarios: [RT-daemon-drain-admission, MS-daemon-memory-reporting, RT-002]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Drain via one transport, read identical state via the other two; new session/prompt/enqueue/claim each refused with the deterministic temporary reason while the in-flight prompt and run complete."
      - "Second drain call = idempotent no-op; undrain restores admission; drain again then restart → boots active (in-memory state)."
      - "runtime.memory doctor item: populated fields at a positive interval, deterministic disabled state at 0s, identical over HTTP/UDS/CLI; doctor items list includes the runtime memory entry (RT-002)."
      - "Confirm agents can observe draining through status (SD-011) and no detached work was cancelled."
    must_avoid:
      - "Killing the daemon as a substitute for drain; the abrupt path is exactly what this feature replaces."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
