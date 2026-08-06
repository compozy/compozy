# CH-acp-stream-disconnect-recovery: Keep partial work and recover after an ACP disconnect

```yaml
charter:
  id: CH-acp-stream-disconnect-recovery
  mission: "As Ada, drive one session through a provider-process disconnect and prove every structured surface reports the failure without losing delivered output or replaying the failed prompt."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-15
  scenarios: [RT-acp-stream-disconnect-recovery]
  tour: Network Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Send a prompt through HTTP to a deterministic ACP subprocess that emits one assistant chunk and then exits; read the terminal stream frame, transcript, session detail, and crash diagnostics."
      - "Repeat through the UDS-backed CLI in JSONL mode; preserve stdout and confirm the process exits nonzero after printing both the partial chunk and terminal error frame."
      - "Invoke `compozy__session_prompt` through the native-tool HTTP surface and confirm it returns `tool_backend_failed` with `backend_dead` rather than a successful result."
      - "Send one different explicit prompt to the same Compozy session and confirm it completes without replaying the failed prompt or duplicating the partial chunk."
      - "Compare HTTP, UDS, CLI, native-tool, persisted transcript, and runtime diagnostics for the same failure classification."
    must_avoid:
      - "Do not automatically retry the failed prompt; its provider-side effects are unknown."
      - "Do not treat a client-only network disconnect as a provider-process disconnect; J-15 owns that separate reconnect path."
```

<!-- Immutable charter: write each run's outcome only in that run report. -->
