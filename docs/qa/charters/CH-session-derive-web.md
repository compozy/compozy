# CH-session-derive-web: Continue and fork from every web entry point without touching the source

```yaml
charter:
  id: CH-session-derive-web
  mission: "As Bruno, continue a session with another agent and fork it (whole and from a message) from every web entry point, and prove each submit opens exactly one truthful child while the source window, transcript, and fences stay unchanged."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [ET-web-session-continue, ET-web-session-fork-from-here, ET-web-sessions-catalog-modal, ET-web-session-sidebar-threads, RT-conversation-rewind]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Every entry point: row menu in the sessions modal, window sidebar, and agent detail; window overflow; Fork from here on a durable user message; the provider-error marker; the dead-runtime banner. Items absent on archived/spawned rows."
      - "Both placements: New window opens one extra window; This window retargets the current one; lists outside a window always open a new window."
      - "Refusals: preview error, a cut whose turn is still running, a transcript changed after opening (send a CLI prompt with the dialog open). Primary stays disabled and nothing appears in GET /api/sessions."
      - "Child truth after reload: origin pill links to the source, divider sits above the child's first turn (or alone above the empty state), inspector Origin/Seed match `session status -o json`; the child nests under its source in the rail and the modal."
    must_avoid:
      - "Judging pixel parity here — the VC bundles (E2E-005) own that."
      - "Native-clone behavior beyond the dialog sentence — CH-session-derive-real-agents owns it."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
